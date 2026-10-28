package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"superlocal/internal/auth"
	"superlocal/internal/httpapi"
	"superlocal/internal/store"
)

func TestSession(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	dir := t.TempDir()
	limiter := auth.NewLimiter()
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:      db,
		DataDir: dir,
		Limiter: limiter,
		Now:     func() time.Time { return now },
	}))
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	loginBody := `{"key":"user-key1","nickname":"甲"}`
	res, err := client.Post(srv.URL+"/api/session", "application/json", strings.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("login status=%d body=%s", res.StatusCode, body)
	}
	setCookie := res.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "sl_session=") {
		t.Fatalf("Set-Cookie missing sl_session: %q", setCookie)
	}
	if !strings.Contains(setCookie, "HttpOnly") {
		t.Fatalf("Set-Cookie missing HttpOnly: %q", setCookie)
	}
	if !strings.Contains(strings.ToLower(setCookie), "samesite=lax") {
		t.Fatalf("Set-Cookie missing SameSite=Lax: %q", setCookie)
	}

	meRes, err := client.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	meBody, _ := io.ReadAll(meRes.Body)
	meRes.Body.Close()
	if meRes.StatusCode != 200 {
		t.Fatalf("me status=%d body=%s", meRes.StatusCode, meBody)
	}
	var me map[string]any
	if err := json.Unmarshal(meBody, &me); err != nil {
		t.Fatal(err)
	}
	if me["key"] != "user-key1" {
		t.Fatalf("me.key = %#v, want user-key1", me["key"])
	}
	userID, _ := me["id"].(string)
	if userID == "" {
		t.Fatal("me.id empty")
	}

	// 第 21 次错误登录尝试返回 429（独立限流器）。
	failLimiter := auth.NewLimiter()
	failSrv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:      db,
		DataDir: dir,
		Limiter: failLimiter,
		Now:     func() time.Time { return now },
	}))
	t.Cleanup(failSrv.Close)
	for i := 0; i < 20; i++ {
		r, err := http.Post(failSrv.URL+"/api/session", "application/json", strings.NewReader(`{"key":"bad","nickname":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode == 429 {
			t.Fatalf("attempt %d got 429 early: %s", i+1, b)
		}
	}
	r21, err := http.Post(failSrv.URL+"/api/session", "application/json", strings.NewReader(`{"key":"bad","nickname":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	b21, _ := io.ReadAll(r21.Body)
	r21.Body.Close()
	if r21.StatusCode != 429 {
		t.Fatalf("21st bad login status=%d body=%s", r21.StatusCode, b21)
	}

	// 成功登录也计入限流：20 次成功后第 21 次为 429。
	okLimiter := auth.NewLimiter()
	okSrv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:      db,
		DataDir: dir,
		Limiter: okLimiter,
		Now:     func() time.Time { return now },
	}))
	t.Cleanup(okSrv.Close)
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("ok-key-%02d", i) // 9 chars, valid
		payload, _ := json.Marshal(map[string]string{"key": key, "nickname": "用户"})
		r, err := http.Post(okSrv.URL+"/api/session", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatalf("success attempt %d status=%d body=%s key=%s", i+1, r.StatusCode, b, key)
		}
	}
	rLast, err := http.Post(okSrv.URL+"/api/session", "application/json", strings.NewReader(`{"key":"user-key1","nickname":"甲"}`))
	if err != nil {
		t.Fatal(err)
	}
	bLast, _ := io.ReadAll(rLast.Body)
	rLast.Body.Close()
	if rLast.StatusCode != 429 {
		t.Fatalf("21st success-counted login status=%d body=%s", rLast.StatusCode, bLast)
	}

	if err := db.UpdateUserAdmin(userID, true, true, true, true); err != nil {
		t.Fatal(err)
	}
	banLimiter := auth.NewLimiter()
	banSrv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:      db,
		DataDir: dir,
		Limiter: banLimiter,
		Now:     func() time.Time { return now },
	}))
	t.Cleanup(banSrv.Close)
	banRes, err := http.Post(banSrv.URL+"/api/session", "application/json", strings.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	banBody, _ := io.ReadAll(banRes.Body)
	banRes.Body.Close()
	if banRes.StatusCode != 403 {
		t.Fatalf("banned login status=%d body=%s", banRes.StatusCode, banBody)
	}
	if !strings.Contains(string(banBody), "封禁") {
		t.Fatalf("banned body missing 封禁: %s", banBody)
	}
	if sc := banRes.Header.Get("Set-Cookie"); strings.Contains(sc, "sl_session=") {
		t.Fatalf("banned login must not set cookie: %q", sc)
	}
}

func TestAdmin(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	dir := t.TempDir()
	username, password, err := auth.EnsureAdmin(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	if username != "admin" {
		t.Fatalf("username=%q", username)
	}

	user, err := db.LoginOrCreate("admin-user-key", "乙", now)
	if err != nil {
		t.Fatal(err)
	}
	peer, err := db.LoginOrCreate("admin-peer-key", "丙", now)
	if err != nil {
		t.Fatal(err)
	}
	groupID, err := db.CreateGroup(user.ID, "测试群", []string{peer.ID}, now)
	if err != nil {
		t.Fatal(err)
	}

	limiter := auth.NewLimiter()
	handler := httpapi.New(httpapi.Deps{
		DB:      db,
		DataDir: dir,
		Limiter: limiter,
		Now:     func() time.Time { return now },
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// 错误密码
	badRes, err := http.Post(srv.URL+"/api/admin/session", "application/json",
		strings.NewReader(`{"username":"admin","password":"wrong-password!!"}`))
	if err != nil {
		t.Fatal(err)
	}
	badBody, _ := io.ReadAll(badRes.Body)
	badRes.Body.Close()
	if badRes.StatusCode != 401 {
		t.Fatalf("bad admin login status=%d body=%s", badRes.StatusCode, badBody)
	}
	s := string(badBody)
	if strings.Contains(s, "密码错误") || strings.Contains(s, "账号错误") {
		t.Fatalf("must not reveal password/account error: %s", s)
	}
	if !strings.Contains(s, "登录失败") {
		t.Fatalf("want 登录失败 in body: %s", s)
	}

	// 无 cookie → 401
	statsRes, err := http.Get(srv.URL + "/api/admin/stats")
	if err != nil {
		t.Fatal(err)
	}
	statsBody, _ := io.ReadAll(statsRes.Body)
	statsRes.Body.Close()
	if statsRes.StatusCode != 401 {
		t.Fatalf("stats without cookie status=%d body=%s", statsRes.StatusCode, statsBody)
	}

	// 聊天 cookie 不能访问管理接口
	chatJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	chatClient := &http.Client{Jar: chatJar}
	chatLogin, err := chatClient.Post(srv.URL+"/api/session", "application/json",
		strings.NewReader(`{"key":"admin-user-key","nickname":"乙"}`))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, chatLogin.Body)
	chatLogin.Body.Close()
	if chatLogin.StatusCode != 200 {
		t.Fatalf("chat login status=%d", chatLogin.StatusCode)
	}
	chatStats, err := chatClient.Get(srv.URL + "/api/admin/stats")
	if err != nil {
		t.Fatal(err)
	}
	chatStatsBody, _ := io.ReadAll(chatStats.Body)
	chatStats.Body.Close()
	if chatStats.StatusCode != 401 {
		t.Fatalf("chat cookie stats status=%d body=%s", chatStats.StatusCode, chatStatsBody)
	}

	// 管理员登录
	adminJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	adminClient := &http.Client{Jar: adminJar}
	adminPayload, _ := json.Marshal(map[string]string{"username": username, "password": password})
	adminLogin, err := adminClient.Post(srv.URL+"/api/admin/session", "application/json", bytes.NewReader(adminPayload))
	if err != nil {
		t.Fatal(err)
	}
	adminLoginBody, _ := io.ReadAll(adminLogin.Body)
	adminLogin.Body.Close()
	if adminLogin.StatusCode != 200 {
		t.Fatalf("admin login status=%d body=%s", adminLogin.StatusCode, adminLoginBody)
	}
	setCookie := adminLogin.Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "sl_admin=") {
		t.Fatalf("Set-Cookie missing sl_admin: %q", setCookie)
	}
	if !strings.Contains(setCookie, "HttpOnly") || !strings.Contains(strings.ToLower(setCookie), "samesite=lax") {
		t.Fatalf("admin cookie flags: %q", setCookie)
	}

	okStats, err := adminClient.Get(srv.URL + "/api/admin/stats")
	if err != nil {
		t.Fatal(err)
	}
	okStatsBody, _ := io.ReadAll(okStats.Body)
	okStats.Body.Close()
	if okStats.StatusCode != 200 {
		t.Fatalf("admin stats status=%d body=%s", okStats.StatusCode, okStatsBody)
	}
	var stats map[string]any
	if err := json.Unmarshal(okStatsBody, &stats); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"users", "online", "banned", "messages", "fileBytes"} {
		if _, ok := stats[key]; !ok {
			t.Fatalf("stats missing %s: %s", key, okStatsBody)
		}
	}
	if stats["online"] != float64(0) {
		t.Fatalf("online with nil Hub = %#v, want 0", stats["online"])
	}

	patchBody := `{"banned":false,"canSendMessage":false,"canSendFile":true,"canCreateGroup":true}`
	patchReq, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/admin/users/"+user.ID, strings.NewReader(patchBody))
	if err != nil {
		t.Fatal(err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes, err := adminClient.Do(patchReq)
	if err != nil {
		t.Fatal(err)
	}
	patchResBody, _ := io.ReadAll(patchRes.Body)
	patchRes.Body.Close()
	if patchRes.StatusCode != 200 {
		t.Fatalf("patch status=%d body=%s", patchRes.StatusCode, patchResBody)
	}

	usersRes, err := adminClient.Get(srv.URL + "/api/admin/users")
	if err != nil {
		t.Fatal(err)
	}
	usersBody, _ := io.ReadAll(usersRes.Body)
	usersRes.Body.Close()
	if usersRes.StatusCode != 200 {
		t.Fatalf("users status=%d body=%s", usersRes.StatusCode, usersBody)
	}
	var users []map[string]any
	if err := json.Unmarshal(usersBody, &users); err != nil {
		// 也可能是 {"users":[...]}
		var wrap struct {
			Users []map[string]any `json:"users"`
		}
		if err2 := json.Unmarshal(usersBody, &wrap); err2 != nil || len(wrap.Users) == 0 {
			t.Fatalf("users json: %v / %v body=%s", err, err2, usersBody)
		}
		users = wrap.Users
	}
	found := false
	for _, u := range users {
		if u["id"] == user.ID {
			found = true
			if u["key"] != "admin-user-key" {
				t.Fatalf("user key = %#v", u["key"])
			}
			if _, ok := u["lastSeenAt"]; !ok {
				t.Fatal("missing lastSeenAt")
			}
			if _, ok := u["online"]; !ok {
				t.Fatal("missing online")
			}
			if u["canSendMessage"] != false {
				t.Fatalf("canSendMessage = %#v, want false", u["canSendMessage"])
			}
		}
	}
	if !found {
		t.Fatalf("user %s not in list: %s", user.ID, usersBody)
	}

	pwRes, err := adminClient.Post(srv.URL+"/api/admin/password", "application/json",
		strings.NewReader(`{"password":"new-password-16"}`))
	if err != nil {
		t.Fatal(err)
	}
	pwBody, _ := io.ReadAll(pwRes.Body)
	pwRes.Body.Close()
	if pwRes.StatusCode != 200 {
		t.Fatalf("password change status=%d body=%s", pwRes.StatusCode, pwBody)
	}
	if err := auth.CheckAdmin(db, username, password); err == nil {
		t.Fatal("old password should fail")
	}
	if err := auth.CheckAdmin(db, username, "new-password-16"); err != nil {
		t.Fatal(err)
	}
	// 当前 sl_admin 仍可访问
	afterPW, err := adminClient.Get(srv.URL + "/api/admin/stats")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, afterPW.Body)
	afterPW.Body.Close()
	if afterPW.StatusCode != 200 {
		t.Fatalf("session after password change status=%d", afterPW.StatusCode)
	}

	groupsRes, err := adminClient.Get(srv.URL + "/api/admin/groups")
	if err != nil {
		t.Fatal(err)
	}
	groupsBody, _ := io.ReadAll(groupsRes.Body)
	groupsRes.Body.Close()
	if groupsRes.StatusCode != 200 {
		t.Fatalf("groups status=%d body=%s", groupsRes.StatusCode, groupsBody)
	}
	var groups []map[string]any
	if err := json.Unmarshal(groupsBody, &groups); err != nil {
		var wrap struct {
			Groups []map[string]any `json:"groups"`
		}
		if err2 := json.Unmarshal(groupsBody, &wrap); err2 != nil || len(wrap.Groups) == 0 {
			t.Fatalf("groups json: %v / %v body=%s", err, err2, groupsBody)
		}
		groups = wrap.Groups
	}
	gFound := false
	for _, g := range groups {
		if g["id"] == groupID {
			gFound = true
			if g["name"] != "测试群" {
				t.Fatalf("group name = %#v", g["name"])
			}
			if g["memberCount"] != float64(2) {
				t.Fatalf("memberCount = %#v", g["memberCount"])
			}
			if _, ok := g["createdAt"]; !ok {
				t.Fatal("missing createdAt")
			}
		}
	}
	if !gFound {
		t.Fatalf("group %s not listed: %s", groupID, groupsBody)
	}

	delReq, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/admin/groups/"+groupID, nil)
	if err != nil {
		t.Fatal(err)
	}
	delRes, err := adminClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delBody, _ := io.ReadAll(delRes.Body)
	delRes.Body.Close()
	if delRes.StatusCode != 200 {
		t.Fatalf("delete group status=%d body=%s", delRes.StatusCode, delBody)
	}
	var memberCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ?`, groupID).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 0 {
		t.Fatalf("members after dissolve = %d", memberCount)
	}

}

func openHTTPDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.SQL().Close()
	})
	return db
}
