package upstreamhttp

import (
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/constants"

	"github.com/gin-gonic/gin"
)

// Ping POST /api/v1/upstream/ping
func (h *Handler) Ping(c *gin.Context) {
	userID := getUpstreamUserID(c)
	if userID == 0 {
		errorResponse(c, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	// 站点名称
	siteName := ""
	siteConfig, err := h.Settings.GetByKey(constants.SettingKeySiteConfig)
	if err == nil {
		siteName = configuredSiteName(siteConfig)
	}

	// 用户钱包余额
	balanceStr := "0.00"
	account, err := h.Wallet.GetAccount(userID)
	if err == nil && account != nil {
		balanceStr = account.Balance.StringFixed(2)
	}

	// 币种
	currency, _ := h.Settings.GetSiteCurrency("CNY")

	// 用户会员等级
	var memberLevel gin.H
	user, err := h.Users.GetByID(userID)
	if err == nil && user != nil && user.MemberLevelID > 0 && h.MemberLevels != nil {
		level, levelErr := h.MemberLevels.GetByID(user.MemberLevelID)
		if levelErr == nil && level != nil {
			memberLevel = gin.H{
				"id":   level.ID,
				"name": level.NameJSON,
				"slug": level.Slug,
				"icon": level.Icon,
			}
		}
	}

	successResponse(c, gin.H{
		"ok":               true,
		"site_name":        siteName,
		"protocol_version": "1.0",
		"user_id":          userID,
		"balance":          balanceStr,
		"currency":         currency,
		"member_level":     memberLevel,
	})
}

func configuredSiteName(siteConfig map[string]interface{}) string {
	if brand, ok := siteConfig["brand"].(map[string]interface{}); ok {
		if value, ok := brand["site_name"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	// Backward compatibility for pre-normalized settings rows.
	if value, ok := siteConfig["site_name"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
