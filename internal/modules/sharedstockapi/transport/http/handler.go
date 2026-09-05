package sharedstockhttp

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	upstreamhttp "github.com/dujiao-next/internal/modules/upstreamapi/transport/http"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

const (
	upstreamUserIDKey       = "upstream_user_id"
	upstreamCredentialIDKey = "upstream_credential_id"
)

type Handler struct {
	upstreamhttp.Dependencies
}

func New(dependencies upstreamhttp.Dependencies) *Handler {
	return &Handler{Dependencies: dependencies}
}

func userID(c *gin.Context) uint {
	value, _ := c.Get(upstreamUserIDKey)
	id, _ := value.(uint)
	return id
}

func credentialID(c *gin.Context) uint {
	value, _ := c.Get(upstreamCredentialIDKey)
	id, _ := value.(uint)
	return id
}

func success(c *gin.Context, data any) {
	if data == nil {
		data = []any{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "success", "data": data})
}

func failure(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": message, "data": []any{}})
}

func localizedText(value jsonmap.JSON) string {
	for _, key := range []string{"zh-CN", "zh-TW", "en-US", "zh", "en"} {
		if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	for _, value := range value {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func sharedCode(productID uint) string { return fmt.Sprintf("dj-%d", productID) }

func parseSharedCode(value string) (uint, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "dj-"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid shared code")
	}
	return uint(id), nil
}
