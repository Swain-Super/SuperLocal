package httpapi

import (
	"net/http"
	"strings"
)

const (
	cookieSession = "sl_session"
	cookieAdmin   = "sl_admin"
	cookieMaxAge  = 604800 // 7 days
)

func setSessionCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// 浏览器里每个标签页用自己的令牌。Authorization 和查询参数优先于共享的 Cookie。
// 页面会带上 X-Superlocal-Tab。这种请求没有自己的令牌时，不再回退到 Cookie，
// 否则新开的窗口会沿用上一次登录，登录页出不来。
func userToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token != "" {
			return token
		}
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		return token
	}
	if r.Header.Get("X-Superlocal-Tab") == "1" {
		return ""
	}
	return readCookie(r, cookieSession)
}
