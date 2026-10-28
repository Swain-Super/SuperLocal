package httpapi

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"superlocal/internal/web"
)

// WithStatic 在 API 之外提供嵌入的页面。非 /api/ 且 dist 中没有对应文件时返回 index.html。
func WithStatic(next http.Handler) http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if !distFile(dist, r.URL.Path) {
			serveIndex(w, r, dist)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func distFile(dist fs.FS, urlPath string) bool {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" || name == "." {
		return false
	}
	info, err := fs.Stat(dist, name)
	return err == nil && !info.IsDir()
}

func serveIndex(w http.ResponseWriter, r *http.Request, dist fs.FS) {
	f, err := dist.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "页面不存在", http.StatusInternalServerError)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "页面不存在", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, "index.html", info.ModTime(), rs)
}
