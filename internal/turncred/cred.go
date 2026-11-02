package turncred

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

func Mint(secret []byte, userID string, now time.Time) (string, string) {
	username := fmt.Sprintf("%d:%s", now.Add(5*time.Minute).Unix(), userID)
	return username, password(secret, username)
}

func Verify(secret []byte, username, credential string, now time.Time) bool {
	expText, _, ok := strings.Cut(username, ":")
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(expText, 10, 64)
	if err != nil || now.Unix() > exp {
		return false
	}
	expect := password(secret, username)
	return hmac.Equal([]byte(credential), []byte(expect))
}

func URLs(port string, ips []string) []string {
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}
	out := make([]string, 0, len(ips)*2)
	for _, ip := range ips {
		host := net.JoinHostPort(ip, port)
		out = append(out, "stun:"+host, "turn:"+host+"?transport=udp")
	}
	return out
}

func password(secret []byte, username string) string {
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write([]byte(username))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Password 返回这个用户名对应的 TURN 密码。用户名必须已经通过 Verify 的过期检查。
func Password(secret []byte, username string) string {
	return password(secret, username)
}
