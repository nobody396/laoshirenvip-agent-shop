package middleware

import (
	"crypto/subtle"
	"io"
	"net/http"
	"strings"
	"time"

	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/upstream"
	"github.com/dujiao-next/internal/upstream/sharedstock"

	"github.com/gin-gonic/gin"
)

const upstreamUserIDKey = "upstream_user_id"
const upstreamCredentialIDKey = "upstream_credential_id"

// UpstreamCredentialStore 只暴露签名鉴权链路所需的凭证能力。
type UpstreamCredentialStore interface {
	GetByApiKey(apiKey string) (*apicredentialdomain.ApiCredential, error)
	Update(credential *apicredentialdomain.ApiCredential) error
}

// SharedStockAPIAuthMiddleware authenticates the form-signature contract used
// by ACG/异次元 SharedStock. We deliberately reuse approved Dujiao API
// credentials: api_key is app_id and api_secret is app_key, so one downstream
// account has one wallet and one order ledger regardless of protocol.
func SharedStockAPIAuthMiddleware(credRepo UpstreamCredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		}
		if err := c.Request.ParseForm(); err != nil {
			sharedStockAuthError(c, "请求格式错误")
			return
		}
		appID := strings.TrimSpace(c.PostForm("app_id"))
		signature := strings.ToLower(strings.TrimSpace(c.PostForm("sign")))
		if appID == "" || signature == "" {
			sharedStockAuthError(c, "商户信息不完整")
			return
		}

		cred, err := credRepo.GetByApiKey(appID)
		if err != nil {
			logger.Errorw("shared_stock_auth_db_error", "error", err)
			sharedStockAuthError(c, "系统繁忙")
			return
		}
		if cred == nil || cred.Status != constants.ApiCredentialStatusApproved || !cred.IsActive ||
			cred.User == nil || cred.User.Status != constants.UserStatusActive {
			sharedStockAuthError(c, "商户ID不存在")
			return
		}

		values := make(map[string]string, len(c.Request.PostForm))
		for key := range c.Request.PostForm {
			values[key] = c.Request.PostForm.Get(key)
		}
		expected := sharedstock.Sign(values, cred.ApiSecret)
		if len(signature) != len(expected) || subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
			sharedStockAuthError(c, "密钥错误")
			return
		}

		now := time.Now()
		cred.LastUsedAt = &now
		go func() {
			if updateErr := credRepo.Update(cred); updateErr != nil {
				logger.Warnw("shared_stock_auth_update_last_used_failed", "error", updateErr)
			}
		}()
		c.Set(upstreamUserIDKey, cred.UserID)
		c.Set(upstreamCredentialIDKey, cred.ID)
		c.Set("upstream_api_key", cred.ApiKey)
		c.Next()
	}
}

func sharedStockAuthError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 0, "msg": message, "data": []any{}})
}

// UpstreamAPIAuthMiddleware 上游 API 签名鉴权中间件
func UpstreamAPIAuthMiddleware(credRepo UpstreamCredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(upstream.HeaderApiKey)
		timestampStr := c.GetHeader(upstream.HeaderTimestamp)
		signature := c.GetHeader(upstream.HeaderSignature)

		if apiKey == "" || timestampStr == "" || signature == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error_code": "missing_auth_headers", "error_message": "missing authentication headers"})
			return
		}

		timestamp, err := upstream.ParseTimestamp(timestampStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error_code": "invalid_timestamp", "error_message": "invalid timestamp"})
			return
		}

		if !upstream.IsTimestampValid(timestamp) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error_code": "timestamp_expired", "error_message": "timestamp expired"})
			return
		}

		cred, err := credRepo.GetByApiKey(apiKey)
		if err != nil {
			logger.Errorw("upstream_auth_db_error", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"ok": false, "error_code": "internal_error", "error_message": "internal error"})
			return
		}
		if cred == nil || cred.Status != constants.ApiCredentialStatusApproved || !cred.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error_code": "invalid_api_key", "error_message": "api key is invalid or disabled"})
			return
		}
		if cred.User == nil || cred.User.Status != constants.UserStatusActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"ok": false, "error_code": "user_disabled", "error_message": "user account is disabled"})
			return
		}

		// 读取 body 用于签名验证（限制最大 10MB 防止内存耗尽）
		var body []byte
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"ok": false, "error_code": "bad_request", "error_message": "failed to read request body"})
				return
			}
			// 重置 body 供后续 handler 读取
			c.Request.Body = io.NopCloser(
				&bodyReader{data: body},
			)
		}

		method := c.Request.Method
		path := c.Request.URL.Path

		if !upstream.Verify(cred.ApiSecret, method, path, signature, timestamp, body) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"ok": false, "error_code": "invalid_signature", "error_message": "signature verification failed"})
			return
		}

		// 更新最后使用时间（异步，不阻塞请求）
		now := time.Now()
		cred.LastUsedAt = &now
		go func() {
			if updateErr := credRepo.Update(cred); updateErr != nil {
				logger.Warnw("upstream_auth_update_last_used_failed", "error", updateErr)
			}
		}()

		// 将凭证信息存入 context
		c.Set(upstreamUserIDKey, cred.UserID)
		c.Set(upstreamCredentialIDKey, cred.ID)
		c.Set("upstream_api_key", cred.ApiKey)

		c.Next()
	}
}

// bodyReader 实现 io.Reader，用于重置 body
type bodyReader struct {
	data   []byte
	offset int
}

func (r *bodyReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}
