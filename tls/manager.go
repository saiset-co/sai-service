package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/sync/errgroup"

	"github.com/saiset-co/sai-service/types"
)

type State int32

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
)

type CertManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	logger          types.Logger
	config          *types.TLSConfig
	autocertMgr     *autocert.Manager
	certCache       autocert.Cache
	renewalTicker   *time.Ticker
	stopCh          chan struct{}
	mu              sync.RWMutex
	certificates    map[string]*tls.Certificate
	state           atomic.Value
	shutdownTimeout time.Duration
	renewalInterval time.Duration
}

func NewCertManager(ctx context.Context, logger types.Logger, config types.ConfigManager) (types.TLSManager, error) {
	tlsConfig := config.GetConfig().Server.TLS

	managerCtx, cancel := context.WithCancel(ctx)

	cm := &CertManager{
		ctx:             managerCtx,
		cancel:          cancel,
		logger:          logger,
		config:          tlsConfig,
		stopCh:          make(chan struct{}),
		certificates:    make(map[string]*tls.Certificate),
		shutdownTimeout: 10 * time.Second,
		renewalInterval: 12 * time.Hour,
	}

	cm.state.Store(StateStopped)

	if tlsConfig.AutoCert {
		if err := cm.initializeAutocert(); err != nil {
			cancel()
			return nil, types.WrapError(err, "failed to initialize autocert manager")
		}
	}

	return cm, nil
}

func (cm *CertManager) Serve(addr string) (net.Listener, error) {
	if !cm.IsRunning() {
		return nil, types.ErrServerNotRunning
	}

	var ln net.Listener
	var err error

	if cm.config.AutoCert {
		tlsConfig := cm.GetTLSConfig()
		if tlsConfig == nil {
			return nil, fmt.Errorf("failed to get TLS config from manager")
		}

		ln, err = tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS listener")
		}
	} else {
		if cm.config.CertFile == "" || cm.config.KeyFile == "" {
			return nil, fmt.Errorf("TLS enabled but cert_file or key_file not specified")
		}

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}

		cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate files")
		}

		err = cm.validateCertificate(cert)
		if err != nil {
			return nil, fmt.Errorf("failed to validate certificate files")
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.InsecureSkipVerify = true

		ln, err = tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS listener")
		}
	}

	return ln, nil
}

func (cm *CertManager) GetTLSConfig() *tls.Config {
	if cm.autocertMgr == nil {
		return nil
	}

	tlsConfig := &tls.Config{
		GetCertificate: cm.autocertMgr.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
		MinVersion:     tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		SessionTicketsDisabled: false,
	}

	tlsConfig.GetCertificate = cm.wrapCertificateWithOCSP(cm.autocertMgr.GetCertificate)

	return tlsConfig
}

func (cm *CertManager) Start() error {
	if !cm.transitionState(StateStopped, StateStarting) {
		return types.ErrServerAlreadyRunning
	}

	defer func() {
		if cm.getState() == StateStarting {
			cm.setState(StateRunning)
		}
	}()

	if cm.config.AutoCert {
		ctx, cancel := context.WithTimeout(cm.ctx, 30*time.Second)
		defer cancel()

		g, gCtx := errgroup.WithContext(ctx)

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				cm.preloadCertificates()
				return nil
			}
		})

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				cm.startRenewalMonitor()
				return nil
			}
		})

		if err := g.Wait(); err != nil {
			cm.setState(StateStopped)
			select {
			case <-ctx.Done():
				cm.logger.Warn("TLS manager start timeout")
			default:
				cm.logger.Error("Error during TLS manager startup", zap.Error(err))
			}
			return types.WrapError(err, "failed to start certificate manager")
		}
	}

	cm.logger.Info("TLS Certificate Manager started",
		zap.Strings("domains", cm.config.Domains))

	return nil
}

func (cm *CertManager) Stop() error {
	if !cm.transitionState(StateRunning, StateStopping) {
		return types.ErrServerNotRunning
	}

	defer func() {
		cm.setState(StateStopped)
		cm.cancel()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), cm.shutdownTimeout)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			close(cm.stopCh)
			return nil
		}
	})

	g.Go(func() error {
		if cm.renewalTicker != nil {
			cm.renewalTicker.Stop()
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			cm.logger.Warn("TLS manager stop timeout, some components may not have stopped gracefully")
		default:
			cm.logger.Error("Error during TLS manager shutdown", zap.Error(err))
		}
	} else {
		cm.logger.Info("TLS Certificate Manager stopped gracefully")
	}

	return nil
}

func (cm *CertManager) IsRunning() bool {
	return cm.getState() == StateRunning
}

func (cm *CertManager) getState() State {
	return cm.state.Load().(State)
}

func (cm *CertManager) setState(newState State) bool {
	currentState := cm.getState()
	return cm.state.CompareAndSwap(currentState, newState)
}

func (cm *CertManager) transitionState(from, to State) bool {
	return cm.state.CompareAndSwap(from, to)
}

func (cm *CertManager) validateCertificate(cert tls.Certificate) error {
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	now := time.Now()
	if now.Before(x509Cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid")
	}
	if now.After(x509Cert.NotAfter) {
		return fmt.Errorf("certificate expired")
	}

	return nil
}

func (cm *CertManager) initializeAutocert() error {
	if len(cm.config.Domains) == 0 {
		return types.NewErrorf("no domains specified for TLS certificate")
	}

	if err := cm.validateDomains(); err != nil {
		return types.WrapError(err, "domain validation failed, certificates may not work properly")
	}

	cacheDir := cm.config.CacheDir
	if cacheDir == "" {
		cacheDir = "./certs"
	}

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return types.WrapError(err, "failed to create certificate cache directory")
	}

	cm.certCache = autocert.DirCache(cacheDir)

	cm.autocertMgr = &autocert.Manager{
		Cache:      cm.certCache,
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(cm.config.Domains...),
		Email:      cm.config.Email,
	}

	if cm.config.ACMEDirectory != "" {
		cm.autocertMgr.Client = &acme.Client{
			DirectoryURL: cm.config.ACMEDirectory,
		}
	}

	return nil
}

func (cm *CertManager) wrapCertificateWithOCSP(getCert func(*tls.ClientHelloInfo) (*tls.Certificate, error)) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert, err := getCert(hello)
		if err != nil {
			cm.logger.Error("Failed to get certificate",
				zap.String("server_name", hello.ServerName),
				zap.Error(err))
			return nil, err
		}

		cm.logger.Debug("Certificate retrieved successfully",
			zap.String("server_name", hello.ServerName),
			zap.Strings("supported_protocols", hello.SupportedProtos))

		return cert, nil
	}
}

func (cm *CertManager) validateDomains() error {
	ctx, cancel := context.WithTimeout(cm.ctx, 10*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	for _, domain := range cm.config.Domains {
		d := domain
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				if err := cm.checkDomainDNS(d); err != nil {
					return types.WrapError(err, fmt.Sprintf("domain %s validation failed", d))
				}
				return nil
			}
		})
	}

	return g.Wait()
}

func (cm *CertManager) checkDomainDNS(domain string) error {
	if domain == "" {
		return types.NewErrorf("empty domain name")
	}
	return nil
}

func (cm *CertManager) preloadCertificates() {
	ctx, cancel := context.WithTimeout(cm.ctx, 60*time.Second)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	for _, domain := range cm.config.Domains {
		d := domain
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				hello := &tls.ClientHelloInfo{
					ServerName: d,
				}

				cert, err := cm.autocertMgr.GetCertificate(hello)
				if err != nil {
					cm.logger.Warn("Failed to preload certificate",
						zap.String("domain", d),
						zap.Error(err))
					return nil
				}

				cm.mu.Lock()
				cm.certificates[d] = cert
				cm.mu.Unlock()

				cm.logger.Info("Certificate preloaded successfully",
					zap.String("domain", d))
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			cm.logger.Warn("Certificate preloading timeout")
		default:
			cm.logger.Error("Error during certificate preloading", zap.Error(err))
		}
	}
}

func (cm *CertManager) startRenewalMonitor() {
	cm.renewalTicker = time.NewTicker(cm.renewalInterval)

	go func() {
		defer func() {
			cm.renewalTicker.Stop()
			cm.logger.Debug("Certificate renewal monitor stopped")
		}()

		for {
			select {
			case <-cm.renewalTicker.C:
				cm.checkCertificateRenewal()

			case <-cm.stopCh:
				return

			case <-cm.ctx.Done():
				return
			}
		}
	}()
}

func (cm *CertManager) checkCertificateRenewal() {
	if !cm.IsRunning() {
		return
	}

	cm.logger.Debug("Checking certificate renewal status")

	cm.mu.RLock()
	domains := make([]string, 0, len(cm.certificates))
	for domain := range cm.certificates {
		domains = append(domains, domain)
	}
	cm.mu.RUnlock()

	ctx, cancel := context.WithTimeout(cm.ctx, 5*time.Minute)
	defer cancel()

	g, gCtx := errgroup.WithContext(ctx)

	for _, domain := range domains {
		d := domain
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
				x509Cert, err := cm.getCertificateInfo(d)
				if err != nil {
					cm.logger.Error("Failed to get certificate info",
						zap.String("domain", d),
						zap.Error(err))
					return nil
				}

				renewalTime := x509Cert.NotAfter.Add(-30 * 24 * time.Hour)
				if time.Now().After(renewalTime) {
					cm.logger.Info("Certificate renewal required",
						zap.String("domain", d),
						zap.Time("expires_at", x509Cert.NotAfter))

					hello := &tls.ClientHelloInfo{
						ServerName: d,
					}

					newCert, err := cm.autocertMgr.GetCertificate(hello)
					if err != nil {
						cm.logger.Error("Failed to renew certificate",
							zap.String("domain", d),
							zap.Error(err))
						return nil
					}

					cm.mu.Lock()
					cm.certificates[d] = newCert
					cm.mu.Unlock()

					cm.logger.Info("Certificate renewed successfully",
						zap.String("domain", d))
				}
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		select {
		case <-ctx.Done():
			cm.logger.Warn("Certificate renewal check timeout")
		default:
			cm.logger.Error("Error during certificate renewal check", zap.Error(err))
		}
	}
}

func (cm *CertManager) getCertificateInfo(domain string) (*x509.Certificate, error) {
	cm.mu.RLock()
	cert, exists := cm.certificates[domain]
	cm.mu.RUnlock()

	if !exists {
		return nil, types.NewErrorf("certificate not found for domain: %s", domain)
	}

	if len(cert.Certificate) == 0 {
		return nil, types.NewErrorf("no certificate data for domain: %s", domain)
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, types.WrapError(err, "failed to parse certificate")
	}

	return x509Cert, nil
}

func (cm *CertManager) GetCertificateStatus() map[string]types.CertificateStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	status := make(map[string]types.CertificateStatus)

	for domain, cert := range cm.certificates {
		if len(cert.Certificate) == 0 {
			status[domain] = types.CertificateStatus{
				Domain: domain,
				Status: "error",
				Error:  "no certificate data",
			}
			continue
		}

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			status[domain] = types.CertificateStatus{
				Domain: domain,
				Status: "error",
				Error:  err.Error(),
			}
			continue
		}

		certStatus := "valid"
		daysUntilExpiry := int(time.Until(x509Cert.NotAfter).Hours() / 24)

		if daysUntilExpiry <= 0 {
			certStatus = "expired"
		} else if daysUntilExpiry <= 30 {
			certStatus = "expiring_soon"
		}

		status[domain] = types.CertificateStatus{
			Domain:          domain,
			Status:          certStatus,
			Issuer:          x509Cert.Issuer.String(),
			Subject:         x509Cert.Subject.String(),
			NotBefore:       x509Cert.NotBefore,
			NotAfter:        x509Cert.NotAfter,
			DaysUntilExpiry: daysUntilExpiry,
		}
	}

	return status
}
