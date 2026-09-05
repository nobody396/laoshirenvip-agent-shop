package sharedstock

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strings"
)

// Sign mirrors acg-faka SharedValidation: omit sign and empty values, sort
// byte-wise by key, join the raw key/value pairs, then append the shared key.
func Sign(values map[string]string, appKey string) string {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if key == "sign" || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	parts = append(parts, "key="+appKey)
	sum := md5.Sum([]byte(strings.Join(parts, "&")))
	return hex.EncodeToString(sum[:])
}
