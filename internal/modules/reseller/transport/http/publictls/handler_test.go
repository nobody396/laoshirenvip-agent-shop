package publictlshttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/gin-gonic/gin"
)

type domainRepoStub struct {
	row *resellerdomain.Domain
	err error
}

func (s domainRepoStub) FindActiveVerifiedDomain(string) (*resellerdomain.Domain, error) {
	return s.row, s.err
}

func TestAllowOnlyApprovedCustomDomains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		repo domainRepoStub
		host string
		want int
	}{
		{name: "custom", repo: domainRepoStub{row: &resellerdomain.Domain{Type: resellerdomain.DomainTypeCustom}}, host: "shop.customer.test", want: http.StatusNoContent},
		{name: "system subdomain", repo: domainRepoStub{row: &resellerdomain.Domain{Type: resellerdomain.DomainTypeSubdomain}}, host: "shop.lsrai.shop", want: http.StatusForbidden},
		{name: "missing", repo: domainRepoStub{}, host: "missing.test", want: http.StatusForbidden},
		{name: "repository error", repo: domainRepoStub{err: errors.New("db unavailable")}, host: "shop.customer.test", want: http.StatusForbidden},
		{name: "invalid", repo: domainRepoStub{}, host: "", want: http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			RegisterRoutes(r.Group("/api/v1/public"), NewHandler(tc.repo))
			req := httptest.NewRequest(http.MethodGet, "/api/v1/public/reseller-domain/tls-allow?domain="+tc.host, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
