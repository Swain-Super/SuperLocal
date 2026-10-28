package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"superlocal/internal/files"
	"superlocal/internal/store"
)

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if !user.CanSendFile {
		writeError(w, http.StatusForbidden, "没有发文件权限")
		return
	}
	max := s.maxUpload
	if max <= 0 {
		max = 200 * 1024 * 1024
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	// 32MiB 只限制解析时的内存；允许的文件大小仍由 SaveUpload 按 max 拒绝。
	if err := r.ParseMultipartForm(32 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	fh, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少文件")
		return
	}
	defer fh.Close()

	name := filepath.Base(header.Filename)
	if name == "." || name == "" {
		name = "file"
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	rel, size, err := files.SaveUpload(s.dataDir, user.ID, name, mime, fh, max, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := s.db.InsertFile(user.ID, name, mime, rel, size, s.now())
	if err != nil {
		_ = os.Remove(filepath.Join(s.dataDir, filepath.FromSlash(rel)))
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":   id,
		"name": name,
		"size": size,
		"mime": mime,
	})
}

func (s *server) handleDownload(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	fileID := r.PathValue("id")
	f, err := s.db.FileVisible(user.ID, fileID)
	if err != nil {
		if strings.Contains(err.Error(), "无权") {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	path := filepath.Join(s.dataDir, filepath.FromSlash(f.Path))
	serveDiskFile(w, path, f.Mime, f.Name)
}

func (s *server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	const avatarMax = 2 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, avatarMax)
	if err := r.ParseMultipartForm(3 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	fh, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少文件")
		return
	}
	defer fh.Close()

	mime := header.Header.Get("Content-Type")
	if mime == "" || mime == "application/octet-stream" {
		mime = sniffImageMIME(header.Filename)
	}
	rel, err := files.SaveAvatar(s.dataDir, user.ID, mime, fh)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stored, err := rememberAvatarType(s.dataDir, user.ID, rel, mime)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}
	if err := s.db.SetAvatar(user.ID, stored); err != nil {
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}
	updated, err := s.db.UserByID(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, store.MeView(updated))
}

func (s *server) handleGetAvatar(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	userID := r.PathValue("id")
	u, err := s.db.UserByID(userID)
	if err != nil || u.AvatarPath == "" {
		writeError(w, http.StatusNotFound, "头像不存在")
		return
	}
	path := filepath.Join(s.dataDir, filepath.FromSlash(u.AvatarPath))
	mime := avatarMIME(u.AvatarPath, path)
	serveDiskFile(w, path, mime, "avatar")
}

// rememberAvatarType 给 SaveAvatar 的无扩展名路径补上类型后缀再落库。
// 同一用户换类型时删掉其余后缀，避免旧文件留在 avatars/。
func rememberAvatarType(dir, userID, rel, mime string) (string, error) {
	ext := avatarExt(mime)
	if ext == "" {
		return "", fmt.Errorf("不支持的头像类型")
	}
	stored := rel + ext
	if err := moveOver(dir, rel, stored); err != nil {
		return "", err
	}
	for _, other := range []string{"", ".jpg", ".jpeg", ".png", ".webp", ".gif"} {
		if other == ext {
			continue
		}
		stale := filepath.Join(dir, "avatars", userID+other)
		if err := os.Remove(stale); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return stored, nil
}

func moveOver(dir, fromRel, toRel string) error {
	if fromRel == toRel {
		return nil
	}
	src := filepath.Join(dir, filepath.FromSlash(fromRel))
	dst := filepath.Join(dir, filepath.FromSlash(toRel))
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if rmErr := os.Remove(dst); rmErr != nil && !os.IsNotExist(rmErr) {
		return err
	}
	return os.Rename(src, dst)
}

func avatarMIME(storedPath, diskPath string) string {
	if mime := imageMIMEFromName(storedPath); mime != "" {
		return mime
	}
	if mime := sniffImageFile(diskPath); mime != "" {
		return mime
	}
	return "application/octet-stream"
}

func avatarExt(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ""
	}
}

func imageMIMEFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return ""
	}
}

func sniffImageFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return ""
	}
	switch ct := http.DetectContentType(buf[:n]); ct {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return ct
	default:
		return ""
	}
}

func serveDiskFile(w http.ResponseWriter, path, mime, filename string) {
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "文件不存在")
		return
	}
	defer f.Close()
	if mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	if filename != "" {
		w.Header().Set("Content-Disposition", `inline; filename="`+downloadFilename(filename)+`"`)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// downloadFilename 只保留基名，并去掉会打断 Content-Disposition 的引号和换行。
func downloadFilename(name string) string {
	name = filepath.Base(name)
	name = strings.NewReplacer("\r", "", "\n", "", `"`, "").Replace(name)
	if name == "" {
		return "download"
	}
	return name
}

func sniffImageMIME(filename string) string {
	if mime := imageMIMEFromName(filename); mime != "" {
		return mime
	}
	return "application/octet-stream"
}
