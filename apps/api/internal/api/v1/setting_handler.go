package v1

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/apierr"
	"github.com/sailboxhq/sailbox/apps/api/internal/httputil"
	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type SettingHandler struct {
	svc *service.SettingService
}

func NewSettingHandler(svc *service.SettingService) *SettingHandler {
	return &SettingHandler{svc: svc}
}

func (h *SettingHandler) GetAll(c *gin.Context) {
	settings, err := h.svc.GetAll(c.Request.Context())
	if err != nil {
		httputil.RespondError(c, err)
		return
	}
	// Convert to map for easier frontend consumption
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	httputil.RespondOK(c, result)
}

func (h *SettingHandler) Update(c *gin.Context) {
	var input struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		httputil.RespondError(c, apierr.ErrValidation.WithDetail(err.Error()))
		return
	}
	if err := h.svc.Set(c.Request.Context(), input.Key, input.Value); err != nil {
		errMsg := err.Error()
		// "setting saved, but ..." = partial success (DB updated, side effect failed)
		if strings.Contains(errMsg, "setting saved") {
			httputil.RespondOK(c, gin.H{"key": input.Key, "value": input.Value, "warning": errMsg})
			return
		}
		httputil.RespondError(c, err)
		return
	}
	httputil.RespondOK(c, gin.H{"key": input.Key, "value": input.Value})
}

// VerifyDomain checks DNS resolution, reachability, and certificate status for a domain.
func (h *SettingHandler) VerifyDomain(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		httputil.RespondError(c, apierr.ErrBadRequest.WithDetail("domain is required"))
		return
	}

	result := gin.H{"domain": domain}

	// Step 1: DNS resolution
	ips, err := net.LookupHost(domain)
	if err != nil || len(ips) == 0 {
		result["dns"] = "failed"
		result["dns_message"] = "Domain does not resolve. Add an A record pointing to your server IP."
		httputil.RespondOK(c, result)
		return
	}
	result["dns"] = "ok"
	result["dns_ip"] = ips[0]

	// Check if it points to this server
	serverIP := h.svc.GetServerIP(c.Request.Context())
	pointsToUs := false
	for _, ip := range ips {
		if ip == serverIP {
			pointsToUs = true
			break
		}
	}
	if !pointsToUs {
		result["dns"] = "wrong_ip"
		result["dns_message"] = fmt.Sprintf("DNS resolves to %s, but your server IP is %s. Update your A record.", ips[0], serverIP)
		httputil.RespondOK(c, result)
		return
	}

	// Step 2: Port 443 reachability
	conn, err := net.DialTimeout("tcp", domain+":443", 5*time.Second)
	if err != nil {
		result["reachable"] = false
		result["reachable_message"] = "Port 443 not reachable. Check firewall settings."
		httputil.RespondOK(c, result)
		return
	}
	_ = conn.Close()
	result["reachable"] = true

	// Step 3: Certificate status
	tlsConn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp", domain+":443",
		&tls.Config{InsecureSkipVerify: true, ServerName: domain},
	)
	if err != nil {
		result["cert"] = "unknown"
		httputil.RespondOK(c, result)
		return
	}
	defer func() { _ = tlsConn.Close() }()

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		result["cert"] = "none"
		httputil.RespondOK(c, result)
		return
	}

	cert := certs[0]
	issuer := cert.Issuer.CommonName
	if len(cert.Issuer.Organization) > 0 {
		issuer = cert.Issuer.Organization[0]
	}

	if cert.Issuer.CommonName == "TRAEFIK DEFAULT CERT" {
		result["cert"] = "self_signed"
		result["cert_message"] = "Using Traefik default certificate. Let's Encrypt cert is being issued..."
	} else if strings.Contains(issuer, "Let's Encrypt") {
		result["cert"] = "valid"
		result["cert_issuer"] = issuer
		result["cert_expiry"] = cert.NotAfter.Format("2006-01-02")
		days := int(time.Until(cert.NotAfter).Hours() / 24)
		result["cert_days"] = days
	} else if strings.Contains(issuer, "Cloudflare") {
		result["cert"] = "cloudflare"
		result["cert_message"] = "Cloudflare proxy detected. Set SSL mode to Full (Strict)."
	} else {
		result["cert"] = "valid"
		result["cert_issuer"] = issuer
		result["cert_expiry"] = cert.NotAfter.Format("2006-01-02")
	}

	httputil.RespondOK(c, result)
}
