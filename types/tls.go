package types

import (
	"crypto/tls"
	"net"
	"time"
)

type TLSManager interface {
	LifecycleManager
	Serve(addr string) (net.Listener, error)
	GetTLSConfig() *tls.Config
	GetCertificateStatus() map[string]CertificateStatus
	//GetACMEChallengeHandler() func(string, string) error
	//ACMEChallengeMiddleware() func(ctx *fasthttp.RequestCtx, next func(requestCtx *fasthttp.RequestCtx), config *RouteConfig)
}

type CertificateStatus struct {
	Domain          string    `json:"domain"`
	Status          string    `json:"status"`
	Issuer          string    `json:"issuer,omitempty"`
	Subject         string    `json:"subject,omitempty"`
	NotBefore       time.Time `json:"not_before,omitempty"`
	NotAfter        time.Time `json:"not_after,omitempty"`
	DaysUntilExpiry int       `json:"days_until_expiry,omitempty"`
	Error           string    `json:"error,omitempty"`
}
