package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"superlocal/internal/auth"
	"superlocal/internal/httpapi"
)

// tinyPNG 是 1x1 透明 PNG 的完整最小文件。
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func TestChatHTTP(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	dir := t.TempDir()
	limiter := auth.NewLimiter()
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		Hub:       nil,
		DataDir:   dir,
		MaxUpload: 1024 * 1024,
		Limiter:   limiter,
		Now:       func() time.Time { return now },
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "chat-key-jia", "甲")
	yi := loginClient(t, srv.URL, "chat-key-yi", "乙")
	bing := loginClient(t, srv.URL, "chat-key-bing", "丙")

	jiaMe := getMe(t, jia.client, srv.URL)
	yiMe := getMe(t, yi.client, srv.URL)
	bingMe := getMe(t, bing.client, srv.URL)

	// Hub 为 nil 时在线列表为空，元素不含 key。
	onlineRes, err := jia.client.Get(srv.URL + "/api/users/online")
	if err != nil {
		t.Fatal(err)
	}
	onlineBody, _ := io.ReadAll(onlineRes.Body)
	onlineRes.Body.Close()
	if onlineRes.StatusCode != 200 {
		t.Fatalf("online status=%d body=%s", onlineRes.StatusCode, onlineBody)
	}
	var online []map[string]any
	if err := json.Unmarshal(onlineBody, &online); err != nil {
		t.Fatalf("online json: %v body=%s", err, onlineBody)
	}
	if online == nil {
		t.Fatal("online must be [] not null")
	}
	if len(online) != 0 {
		t.Fatalf("online with nil Hub = %s, want []", onlineBody)
	}
	if strings.Contains(string(onlineBody), `"key"`) {
		t.Fatalf("online JSON must not contain key: %s", onlineBody)
	}

	// 搜索用户返回昵称，不含 key。
	usersRes, err := jia.client.Get(srv.URL + "/api/users?q=" + "乙")
	if err != nil {
		t.Fatal(err)
	}
	usersBody, _ := io.ReadAll(usersRes.Body)
	usersRes.Body.Close()
	if usersRes.StatusCode != 200 {
		t.Fatalf("users status=%d body=%s", usersRes.StatusCode, usersBody)
	}
	if strings.Contains(string(usersBody), "chat-key-yi") || strings.Contains(string(usersBody), `"key"`) {
		t.Fatalf("users JSON must not contain key: %s", usersBody)
	}
	var users []map[string]any
	if err := json.Unmarshal(usersBody, &users); err != nil {
		t.Fatalf("users json: %v body=%s", err, usersBody)
	}
	foundYi := false
	for _, u := range users {
		if u["nickname"] == "乙" {
			foundYi = true
			if u["id"] != yiMe["id"] {
				t.Fatalf("yi id = %#v", u["id"])
			}
		}
	}
	if !foundYi {
		t.Fatalf("search missing 乙: %s", usersBody)
	}

	// 打开一对一。
	directPayload, _ := json.Marshal(map[string]string{"userId": yiMe["id"].(string)})
	directRes, err := jia.client.Post(srv.URL+"/api/conversations/direct", "application/json", bytes.NewReader(directPayload))
	if err != nil {
		t.Fatal(err)
	}
	directBody, _ := io.ReadAll(directRes.Body)
	directRes.Body.Close()
	if directRes.StatusCode != 200 {
		t.Fatalf("direct status=%d body=%s", directRes.StatusCode, directBody)
	}
	var direct map[string]any
	if err := json.Unmarshal(directBody, &direct); err != nil {
		t.Fatal(err)
	}
	convID, _ := direct["id"].(string)
	if convID == "" {
		t.Fatalf("direct id empty: %s", directBody)
	}

	// 上传小文本。
	fileID, fileBytes := uploadFile(t, jia.client, srv.URL, "hello.txt", "text/plain", []byte("hello"))
	if fileID == "" {
		t.Fatal("file id empty")
	}

	// 文字与文件消息由测试直接 PostMessage（本任务无 WebSocket）。
	textMsg, err := db.PostMessage(jiaMe["id"].(string), convID, "text", "你好", "", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	fileMsg, err := db.PostMessage(jiaMe["id"].(string), convID, "file", "", fileID, now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	// 消息列表含 senderNickname，不含发送者 key。
	msgRes, err := yi.client.Get(srv.URL + "/api/conversations/" + convID + "/messages?after=")
	if err != nil {
		t.Fatal(err)
	}
	msgBody, _ := io.ReadAll(msgRes.Body)
	msgRes.Body.Close()
	if msgRes.StatusCode != 200 {
		t.Fatalf("messages status=%d body=%s", msgRes.StatusCode, msgBody)
	}
	if strings.Contains(string(msgBody), "chat-key-jia") || strings.Contains(string(msgBody), `"key"`) {
		t.Fatalf("messages JSON must not contain key: %s", msgBody)
	}
	var messages []map[string]any
	if err := json.Unmarshal(msgBody, &messages); err != nil {
		t.Fatalf("messages json: %v body=%s", err, msgBody)
	}
	if len(messages) < 2 {
		t.Fatalf("messages count=%d body=%s", len(messages), msgBody)
	}
	hasNick := false
	for _, m := range messages {
		if m["senderNickname"] == "甲" {
			hasNick = true
		}
	}
	if !hasNick {
		t.Fatalf("missing senderNickname: %s", msgBody)
	}
	var fileView map[string]any
	for _, m := range messages {
		if m["id"] == fileMsg.ID {
			fileView = m
		}
	}
	if fileView == nil || fileView["fileName"] != "hello.txt" || fileView["fileSize"] != float64(len("hello")) {
		t.Fatalf("file message JSON fileName/fileSize = %#v, want hello.txt/%d", fileView, len("hello"))
	}
	fileRaw, err := json.Marshal(fileView)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(fileRaw), "chat-key-jia") || strings.Contains(string(fileRaw), "chat-key-yi") || strings.Contains(string(fileRaw), `"key"`) {
		t.Fatalf("file message JSON must not contain another user's key: %s", fileRaw)
	}

	// 建群；非成员下载 403；成员可下载字节。
	groupPayload, _ := json.Marshal(map[string]any{
		"name":      "测试群聊",
		"memberIds": []string{yiMe["id"].(string)},
	})
	groupRes, err := jia.client.Post(srv.URL+"/api/groups", "application/json", bytes.NewReader(groupPayload))
	if err != nil {
		t.Fatal(err)
	}
	groupBody, _ := io.ReadAll(groupRes.Body)
	groupRes.Body.Close()
	if groupRes.StatusCode != 200 {
		t.Fatalf("groups status=%d body=%s", groupRes.StatusCode, groupBody)
	}
	var group map[string]any
	if err := json.Unmarshal(groupBody, &group); err != nil {
		t.Fatal(err)
	}
	groupID, _ := group["id"].(string)
	if groupID == "" {
		t.Fatalf("group id empty: %s", groupBody)
	}
	groupViews := listConversations(t, yi.client, srv.URL)
	var groupView map[string]any
	for _, c := range groupViews {
		if c["id"] == groupID {
			groupView = c
		}
	}
	if groupView == nil || groupView["type"] != "group" || groupView["ownerId"] != jiaMe["id"] {
		t.Fatalf("group conversation JSON = %#v, want ownerId %v", groupView, jiaMe["id"])
	}
	groupRaw, err := json.Marshal(groupView)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(groupRaw), `"ownerId"`) {
		t.Fatalf("group conversation JSON missing ownerId: %s", groupRaw)
	}
	if strings.Contains(string(groupRaw), "chat-key-jia") || strings.Contains(string(groupRaw), "chat-key-yi") || strings.Contains(string(groupRaw), `"key"`) {
		t.Fatalf("group conversation JSON must not contain key: %s", groupRaw)
	}

	groupFileID, groupFileBytes := uploadFile(t, jia.client, srv.URL, "group.txt", "text/plain", []byte("group-data"))
	if _, err := db.PostMessage(jiaMe["id"].(string), groupID, "file", "group.txt", groupFileID, now.Add(3*time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	forbidRes, err := bing.client.Get(srv.URL + "/api/files/" + groupFileID)
	if err != nil {
		t.Fatal(err)
	}
	forbidBody, _ := io.ReadAll(forbidRes.Body)
	forbidRes.Body.Close()
	if forbidRes.StatusCode != 403 {
		t.Fatalf("non-member download status=%d body=%s", forbidRes.StatusCode, forbidBody)
	}
	_ = bingMe

	okRes, err := yi.client.Get(srv.URL + "/api/files/" + groupFileID)
	if err != nil {
		t.Fatal(err)
	}
	okBody, _ := io.ReadAll(okRes.Body)
	okRes.Body.Close()
	if okRes.StatusCode != 200 {
		t.Fatalf("member download status=%d body=%s", okRes.StatusCode, okBody)
	}
	if !bytes.Equal(okBody, groupFileBytes) {
		t.Fatalf("download body = %q, want %q", okBody, groupFileBytes)
	}

	// 一对一文件成员也可下载。
	directFileRes, err := yi.client.Get(srv.URL + "/api/files/" + fileID)
	if err != nil {
		t.Fatal(err)
	}
	directFileBody, _ := io.ReadAll(directFileRes.Body)
	directFileRes.Body.Close()
	if directFileRes.StatusCode != 200 {
		t.Fatalf("direct file status=%d body=%s", directFileRes.StatusCode, directFileBody)
	}
	if !bytes.Equal(directFileBody, fileBytes) {
		t.Fatalf("direct file body = %q", directFileBody)
	}

	// 乙未读应为 2（甲的两条消息）；标记已读后自己未读为 0，甲视角仍可读到未读。
	yiConvs := listConversations(t, yi.client, srv.URL)
	yiUnread := unreadFor(yiConvs, convID)
	if yiUnread < 1 {
		t.Fatalf("yi unread before read = %d, want >= 1", yiUnread)
	}

	// 乙再发一条，让甲有未读。
	peerMsg, err := db.PostMessage(yiMe["id"].(string), convID, "text", "收到", "", now.Add(4*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_ = peerMsg
	_ = textMsg
	_ = fileMsg

	readPayload, _ := json.Marshal(map[string]string{"messageId": fileMsg.ID})
	readRes, err := yi.client.Post(srv.URL+"/api/conversations/"+convID+"/read", "application/json", bytes.NewReader(readPayload))
	if err != nil {
		t.Fatal(err)
	}
	readBody, _ := io.ReadAll(readRes.Body)
	readRes.Body.Close()
	if readRes.StatusCode != 200 {
		t.Fatalf("read status=%d body=%s", readRes.StatusCode, readBody)
	}

	yiConvsAfter := listConversations(t, yi.client, srv.URL)
	if got := unreadFor(yiConvsAfter, convID); got != 0 {
		t.Fatalf("yi unread after read = %d, want 0", got)
	}

	jiaConvs := listConversations(t, jia.client, srv.URL)
	if got := unreadFor(jiaConvs, convID); got < 1 {
		t.Fatalf("jia (peer) unread = %d, want >= 1", got)
	}
}

func TestFileHTTP(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	dir := t.TempDir()
	limiter := auth.NewLimiter()
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		DataDir:   dir,
		MaxUpload: 1024 * 1024,
		Limiter:   limiter,
		Now:       func() time.Time { return now },
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "file-key-jia", "甲")
	yi := loginClient(t, srv.URL, "file-key-yi", "乙")
	jiaMe := getMe(t, jia.client, srv.URL)

	avatarBody := &bytes.Buffer{}
	aw := multipart.NewWriter(avatarBody)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="avatar.png"`)
	h.Set("Content-Type", "image/png")
	part, err := aw.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(tinyPNG); err != nil {
		t.Fatal(err)
	}
	if err := aw.Close(); err != nil {
		t.Fatal(err)
	}
	avatarReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/me/avatar", avatarBody)
	if err != nil {
		t.Fatal(err)
	}
	avatarReq.Header.Set("Content-Type", aw.FormDataContentType())
	avatarRes, err := jia.client.Do(avatarReq)
	if err != nil {
		t.Fatal(err)
	}
	avatarResBody, _ := io.ReadAll(avatarRes.Body)
	avatarRes.Body.Close()
	if avatarRes.StatusCode != 200 {
		t.Fatalf("avatar upload status=%d body=%s", avatarRes.StatusCode, avatarResBody)
	}

	meAfter := getMe(t, jia.client, srv.URL)
	avatarURL, _ := meAfter["avatarUrl"].(string)
	if !strings.HasPrefix(avatarURL, "/api/avatars/") {
		t.Fatalf("avatarUrl = %q, want prefix /api/avatars/", avatarURL)
	}

	imgRes, err := yi.client.Get(srv.URL + avatarURL)
	if err != nil {
		t.Fatal(err)
	}
	imgBody, _ := io.ReadAll(imgRes.Body)
	imgRes.Body.Close()
	if imgRes.StatusCode != 200 {
		t.Fatalf("avatar get status=%d body=%s", imgRes.StatusCode, imgBody)
	}
	if !bytes.Equal(imgBody, tinyPNG) {
		t.Fatalf("avatar bytes mismatch len=%d", len(imgBody))
	}

	// 上传响应字段。
	fileID, content := uploadFile(t, jia.client, srv.URL, "note.txt", "text/plain", []byte("note"))
	if fileID == "" || !bytes.Equal(content, []byte("note")) {
		t.Fatalf("upload fileID=%q content=%q", fileID, content)
	}
	_ = jiaMe
}

func TestAvatarJPEG(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	dir := t.TempDir()
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		DataDir:   dir,
		MaxUpload: 1024 * 1024,
		Limiter:   auth.NewLimiter(),
		Now:       func() time.Time { return now },
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "jpeg-key", "甲")
	jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
	postAvatar(t, jia.client, srv.URL, "photo.jpg", "image/jpeg", jpeg)

	me := getMe(t, jia.client, srv.URL)
	avatarURL, _ := me["avatarUrl"].(string)
	if !strings.HasPrefix(avatarURL, "/api/avatars/") {
		t.Fatalf("avatarUrl = %q", avatarURL)
	}
	res, err := jia.client.Get(srv.URL + avatarURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("avatar get status=%d body=%s", res.StatusCode, body)
	}
	if ct := res.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want image/jpeg", ct)
	}
	if !bytes.Equal(body, jpeg) {
		t.Fatalf("avatar bytes mismatch")
	}
}

func TestPatchMe(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:      db,
		Limiter: auth.NewLimiter(),
		Now:     func() time.Time { return now },
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "patch-key-jia", "甲")
	yi := loginClient(t, srv.URL, "patch-key-yi", "乙")

	me := patchMe(t, jia.client, srv.URL, `{"nickname":"  新昵称  "}`, 200)
	if me["nickname"] != "新昵称" {
		t.Fatalf("nickname = %v", me["nickname"])
	}
	if me["key"] != "patch-key-jia" {
		t.Fatalf("key = %v", me["key"])
	}
	if me["id"] == "" || me["id"] == nil {
		t.Fatalf("id = %v", me["id"])
	}
	if _, ok := me["avatarUrl"]; !ok {
		t.Fatalf("missing avatarUrl: %v", me)
	}
	for _, flag := range []string{"canSendMessage", "canSendFile", "canCreateGroup"} {
		if me[flag] != true {
			t.Fatalf("%s = %v, want true", flag, me[flag])
		}
	}

	got := getMe(t, jia.client, srv.URL)
	if got["nickname"] != "新昵称" || got["key"] != "patch-key-jia" {
		t.Fatalf("stored me = %v", got)
	}
	if got["canSendMessage"] != true || got["canSendFile"] != true || got["canCreateGroup"] != true {
		t.Fatalf("flags = %v", got)
	}
	yiMe := getMe(t, yi.client, srv.URL)
	if yiMe["nickname"] != "乙" || yiMe["key"] != "patch-key-yi" {
		t.Fatalf("other user changed: %v", yiMe)
	}

	longNick := strings.Repeat("字", 33)
	for _, body := range []string{
		`{"nickname":"a\nb"}`,
		`{"nickname":"a\rb"}`,
		`{"nickname":"   "}`,
		`{"nickname":"` + longNick + `"}`,
	} {
		patchMe(t, jia.client, srv.URL, body, 400)
	}
	if again := getMe(t, jia.client, srv.URL); again["nickname"] != "新昵称" {
		t.Fatalf("nickname changed after invalid patch: %v", again["nickname"])
	}
}

func TestDownloadFilename(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	dir := t.TempDir()
	handler := httpapi.New(httpapi.Deps{
		DB:        db,
		DataDir:   dir,
		MaxUpload: 1024 * 1024,
		Limiter:   auth.NewLimiter(),
		Now:       func() time.Time { return now },
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "dl-key-jia", "甲")
	yi := loginClient(t, srv.URL, "dl-key-yi", "乙")
	jiaMe := getMe(t, jia.client, srv.URL)
	yiMe := getMe(t, yi.client, srv.URL)

	directPayload, _ := json.Marshal(map[string]string{"userId": yiMe["id"].(string)})
	directRes, err := jia.client.Post(srv.URL+"/api/conversations/direct", "application/json", bytes.NewReader(directPayload))
	if err != nil {
		t.Fatal(err)
	}
	directBody, _ := io.ReadAll(directRes.Body)
	directRes.Body.Close()
	if directRes.StatusCode != 200 {
		t.Fatalf("direct status=%d body=%s", directRes.StatusCode, directBody)
	}
	var direct map[string]any
	if err := json.Unmarshal(directBody, &direct); err != nil {
		t.Fatal(err)
	}
	convID, _ := direct["id"].(string)
	if convID == "" {
		t.Fatalf("direct id empty: %s", directBody)
	}

	payload := []byte("safe-body")
	rel := "files/dl-name"
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	rawName := "notes/evil\"\r\nname.txt"
	fileID, err := db.InsertFile(jiaMe["id"].(string), rawName, "text/plain", rel, int64(len(payload)), now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.PostMessage(jiaMe["id"].(string), convID, "file", "x", fileID, now.Add(time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+fileID, nil)
	base, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range yi.client.Jar.Cookies(base) {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download status=%d body=%s", rec.Code, rec.Body.Bytes())
	}
	if !bytes.Equal(rec.Body.Bytes(), payload) {
		t.Fatalf("body = %q", rec.Body.Bytes())
	}
	cd := rec.Header().Get("Content-Disposition")
	if strings.Contains(cd, "\n") || strings.Contains(cd, "\r") || (strings.Contains(cd, `"`) && strings.Count(cd, `"`) != 2) {
		t.Fatalf("Content-Disposition leaked quote or newline: %q", cd)
	}
	if strings.Contains(cd, "evil\"") || strings.Contains(cd, "notes/") {
		t.Fatalf("Content-Disposition leaked raw filename: %q", cd)
	}
	if cd != `inline; filename="evilname.txt"` {
		t.Fatalf("Content-Disposition = %q", cd)
	}
}

func TestUploadRejectsOversizedBody(t *testing.T) {
	now := time.Unix(1_700_000_500, 0)
	db := openHTTPDB(t)
	dir := t.TempDir()
	const maxUpload = 256
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		Hub:       nil,
		DataDir:   dir,
		MaxUpload: maxUpload,
		Limiter:   auth.NewLimiter(),
		Now:       func() time.Time { return now },
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "upload-oversize-jia", "甲")
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "big.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(bytes.Repeat([]byte("a"), maxUpload+1024)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/files", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := jia.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatalf("expected oversized upload to fail, body=%s", resBody)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", res.StatusCode, resBody)
	}
}

func postAvatar(t *testing.T, c *http.Client, base, filename, mime string, data []byte) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", mime)
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/me/avatar", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("avatar upload status=%d body=%s", res.StatusCode, resBody)
	}
}

func patchMe(t *testing.T, c *http.Client, base, payload string, want int) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, base+"/api/me", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != want {
		t.Fatalf("patch me status=%d body=%s", res.StatusCode, resBody)
	}
	if want != 200 {
		return nil
	}
	var me map[string]any
	if err := json.Unmarshal(resBody, &me); err != nil {
		t.Fatal(err)
	}
	return me
}

type apiClient struct {
	client *http.Client
}

func loginClient(t *testing.T, base, key, nickname string) apiClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &http.Client{Jar: jar}
	payload, _ := json.Marshal(map[string]string{"key": key, "nickname": nickname})
	res, err := c.Post(base+"/api/session", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("login %s status=%d body=%s", key, res.StatusCode, body)
	}
	return apiClient{client: c}
}

func getMe(t *testing.T, c *http.Client, base string) map[string]any {
	t.Helper()
	res, err := c.Get(base + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("me status=%d body=%s", res.StatusCode, body)
	}
	var me map[string]any
	if err := json.Unmarshal(body, &me); err != nil {
		t.Fatal(err)
	}
	return me
}

func uploadFile(t *testing.T, c *http.Client, base, filename, mime string, data []byte) (string, []byte) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	_ = mime
	req, err := http.NewRequest(http.MethodPost, base+"/api/files", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("upload status=%d body=%s", res.StatusCode, resBody)
	}
	var out map[string]any
	if err := json.Unmarshal(resBody, &out); err != nil {
		t.Fatal(err)
	}
	id, _ := out["id"].(string)
	name, _ := out["name"].(string)
	if id == "" || name == "" {
		t.Fatalf("upload response missing id/name: %s", resBody)
	}
	if _, ok := out["size"]; !ok {
		t.Fatalf("upload missing size: %s", resBody)
	}
	if _, ok := out["mime"]; !ok {
		t.Fatalf("upload missing mime: %s", resBody)
	}
	return id, data
}

func listConversations(t *testing.T, c *http.Client, base string) []map[string]any {
	t.Helper()
	res, err := c.Get(base + "/api/conversations")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("conversations status=%d body=%s", res.StatusCode, body)
	}
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("conversations json: %v body=%s", err, body)
	}
	return list
}

func unreadFor(list []map[string]any, convID string) int {
	for _, c := range list {
		if c["id"] == convID {
			switch v := c["unread"].(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
		}
	}
	return -1
}
