package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	"github.com/dujiao-next/internal/upstream/sharedstock"

	"github.com/gin-gonic/gin"
)

type sharedStockCredentialStore struct {
	credential *apicredentialdomain.ApiCredential
}

func (s *sharedStockCredentialStore) GetByApiKey(apiKey string) (*apicredentialdomain.ApiCredential, error) {
	if s.credential != nil && s.credential.ApiKey == apiKey {
		return s.credential, nil
	}
	return nil, nil
}

func (s *sharedStockCredentialStore) Update(credential *apicredentialdomain.ApiCredential) error {
	s.credential = credential
	return nil
}

func TestSharedStockAPIAuthAcceptsSignedForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &sharedStockCredentialStore{credential: &apicredentialdomain.ApiCredential{
		ID: 9, UserID: 7, ApiKey: "merchant-7", ApiSecret: "secret",
		Status: constants.ApiCredentialStatusApproved, IsActive: true,
		User: &userdomain.User{ID: 7, Status: constants.UserStatusActive},
	}}
	values := url.Values{"app_id": {"merchant-7"}, "code": {"dj-1"}}
	plain := map[string]string{"app_id": "merchant-7", "code": "dj-1"}
	values.Set("sign", sharedstock.Sign(plain, "secret"))

	router := gin.New()
	router.POST("/shared/commodity/item", SharedStockAPIAuthMiddleware(store), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user": c.GetUint(upstreamUserIDKey), "credential": c.GetUint(upstreamCredentialIDKey),
			"code": c.PostForm("code"),
		})
	})
	request := httptest.NewRequest(http.MethodPost, "/shared/commodity/item", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"user":7`) ||
		!strings.Contains(recorder.Body.String(), `"credential":9`) || !strings.Contains(recorder.Body.String(), `"code":"dj-1"`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestSharedStockAPIAuthRejectsBadSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &sharedStockCredentialStore{credential: &apicredentialdomain.ApiCredential{
		ID: 9, UserID: 7, ApiKey: "merchant-7", ApiSecret: "secret",
		Status: constants.ApiCredentialStatusApproved, IsActive: true,
		User: &userdomain.User{ID: 7, Status: constants.UserStatusActive},
	}}
	router := gin.New()
	router.POST("/shared/authentication/connect", SharedStockAPIAuthMiddleware(store), func(c *gin.Context) {
		t.Fatal("handler must not run")
	})
	request := httptest.NewRequest(
		http.MethodPost, "/shared/authentication/connect",
		strings.NewReader("app_id=merchant-7&sign="+fmt.Sprintf("%032d", 0)),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"code":0`) {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}
