package auth

import (
	"fmt"
	"regexp"
	"strings"
)

// keyPattern 允许 8–64 位字母、数字、下划线和连字符。
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func NormalizeKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if !keyPattern.MatchString(key) {
		return "", fmt.Errorf("Key 须为 8–64 位字母、数字、下划线或连字符")
	}
	return key, nil
}
