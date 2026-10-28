package httpapi

import (
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
	if err := r.ParseMultipartForm(max + 1024); err != nil {
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
	if err := s.db.SetAvatar(user.ID, rel); err != nil {
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
	serveDiskFile(w, path, "image/png", "avatar")
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
		w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func sniffImageMIME(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
