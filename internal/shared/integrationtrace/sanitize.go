package integrationtrace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
)

const redacted = "[REDACTED]"

// Digest returns a stable correlation fingerprint without retaining raw data.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SanitizeStringMap preserves protocol fields while removing credentials and
// fulfillment data that must never be written to the integration evidence log.
func SanitizeStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		if isSensitiveKey(key) {
			result[key] = redacted
			continue
		}
		result[key] = value
	}
	return result
}

// SanitizeForm is the url.Values equivalent of SanitizeStringMap.
func SanitizeForm(values map[string][]string) map[string][]string {
	result := make(map[string][]string, len(values))
	for key, value := range values {
		if isSensitiveKey(key) {
			result[key] = []string{redacted}
			continue
		}
		result[key] = append([]string(nil), value...)
	}
	return result
}

// SanitizeJSON converts a JSON body to a recursively redacted value. Invalid
// JSON is represented only by its size and digest, never by the raw body.
func SanitizeJSON(data []byte) any {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return map[string]any{"invalid_json": true, "size": len(data), "sha256": Digest(data)}
	}
	return sanitizeValue(value)
}

// URLSummary records only the destination and non-sensitive query fields.
func URLSummary(raw string) map[string]any {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return map[string]any{"valid": false}
	}
	return map[string]any{
		"scheme": parsed.Scheme,
		"host":   parsed.Host,
		"path":   parsed.Path,
		"query":  SanitizeForm(parsed.Query()),
	}
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if isSensitiveKey(key) {
				result[key] = redacted
				continue
			}
			result[key] = sanitizeValue(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = sanitizeValue(child)
		}
		return result
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "sign", "signature", "app_key", "merchant_key", "secret", "secret_key",
		"private_key", "platform_public_key", "api_key", "token", "password",
		"card", "cards", "card_code", "card_codes", "cdk", "delivery_data",
		"fulfillment", "payload":
		return true
	default:
		return false
	}
}
