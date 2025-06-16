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

	"github.com/saiset-co/sai-service/types"
)

type CertManager struct {
	ctx           context.Context
	logger        types.Logger
	config        *types.TLSConfig
	autocertMgr   *autocert.Manager
	certCache     autocert.Cache
	renewalTicker *time.Ticker
	stopCh        chan struct{}
	mu            sync.RWMutex
	certificates  map[string]*tls.Certificate
	running       uint32
}

func NewCertManager(ctx context.Context, logger types.Logger, config types.ConfigManager) (types.TLSManager, error) {
	tlsConfig := config.GetConfig().Server.TLS

	cm := &CertManager{
		ctx:          ctx,
		logger:       logger,
		config:       tlsConfig,
		stopCh:       make(chan struct{}),
		certificates: make(map[string]*tls.Certificate),
	}

	if tlsConfig.AutoCert {
		if err := cm.initializeAutocert(); err != nil {
			return nil, types.WrapError(err, "failed to initialize autocert manager")
		}
	}

	return cm, nil
}

func (cm *CertManager) Serve(addr string) (net.Listener, error) {
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
	if !atomic.CompareAndSwapUint32(&cm.running, 0, 1) {
		return types.ErrServerAlreadyRunning
	}

	if cm.config.AutoCert {
		go cm.preloadCertificates()
		cm.startRenewalMonitor()
	}

	cm.logger.Info("TLS Certificate Manager started",
		zap.Strings("domains", cm.config.Domains))

	return nil
}

func (cm *CertManager) Stop() error {
	if !atomic.CompareAndSwapUint32(&cm.running, 1, 0) {
		return types.ErrServerNotRunning
	}

	close(cm.stopCh)

	if cm.renewalTicker != nil {
		cm.renewalTicker.Stop()
	}

	cm.logger.Info("TLS Certificate Manager stopped")
	return nil
}

func (cm *CertManager) IsRunning() bool {
	return atomic.LoadUint32(&cm.running) == 1
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
	for _, domain := range cm.config.Domains {
		if err := cm.checkDomainDNS(domain); err != nil {
			return types.WrapError(err, fmt.Sprintf("domain %s validation failed", domain))
		}
	}
	return nil
}

func (cm *CertManager) checkDomainDNS(domain string) error {
	if domain == "" {
		return types.NewErrorf("empty domain name")
	}
	return nil
}

func (cm *CertManager) preloadCertificates() {
	for _, domain := range cm.config.Domains {
		hello := &tls.ClientHelloInfo{
			ServerName: domain,
		}

		cert, err := cm.autocertMgr.GetCertificate(hello)
		if err != nil {
			cm.logger.Warn("Failed to preload certificate",
				zap.String("domain", domain),
				zap.Error(err))
			continue
		}

		cm.mu.Lock()
		cm.certificates[domain] = cert
		cm.mu.Unlock()

		cm.logger.Info("Certificate preloaded successfully",
			zap.String("domain", domain))
	}
}

func (cm *CertManager) startRenewalMonitor() {
	cm.renewalTicker = time.NewTicker(12 * time.Hour)

	go func() {
		defer cm.renewalTicker.Stop()

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
	cm.logger.Debug("Checking certificate renewal status")

	cm.mu.RLock()
	domains := make([]string, 0, len(cm.certificates))
	for domain := range cm.certificates {
		domains = append(domains, domain)
	}
	cm.mu.RUnlock()

	for _, domain := range domains {
		x509Cert, err := cm.getCertificateInfo(domain)
		if err != nil {
			cm.logger.Error("Failed to get certificate info",
				zap.String("domain", domain),
				zap.Error(err))
			continue
		}

		renewalTime := x509Cert.NotAfter.Add(-30 * 24 * time.Hour)
		if time.Now().After(renewalTime) {
			cm.logger.Info("Certificate renewal required",
				zap.String("domain", domain),
				zap.Time("expires_at", x509Cert.NotAfter))

			hello := &tls.ClientHelloInfo{
				ServerName: domain,
			}

			newCert, err := cm.autocertMgr.GetCertificate(hello)
			if err != nil {
				cm.logger.Error("Failed to renew certificate",
					zap.String("domain", domain),
					zap.Error(err))
				continue
			}

			cm.mu.Lock()
			cm.certificates[domain] = newCert
			cm.mu.Unlock()

			cm.logger.Info("Certificate renewed successfully",
				zap.String("domain", domain))
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
