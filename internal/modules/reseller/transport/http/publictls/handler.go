package publictlshttp

import (
	"net/http"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/gin-gonic/gin"
)

// Handler exposes the narrow authorization endpoint used by Caddy on-demand
// TLS. It never creates or approves a domain; it only confirms that the
// requested hostname is an already-approved custom reseller domain.
type Handler struct {
	domains resellercontract.DomainLookupRepository
}

func NewHandler(domains resellercontract.DomainLookupRepository) *Handler {
	if domains == nil {
		panic("reseller public tls handler: domains is nil")
	}
	return &Handler{domains: domains}
}

func (h *Handler) Allow(c *gin.Context) {
	host := resellercontract.NormalizeHost(c.Query("domain"))
	if host == "" {
		c.Status(http.StatusForbidden)
		return
	}
	row, err := h.domains.FindActiveVerifiedDomain(host)
	if err != nil || row == nil || row.Type != resellerdomain.DomainTypeCustom {
		c.Status(http.StatusForbidden)
		return
	}
	c.Status(http.StatusNoContent)
}
