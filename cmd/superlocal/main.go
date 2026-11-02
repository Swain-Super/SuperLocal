package main

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"superlocal/internal/auth"
	"superlocal/internal/call"
	"superlocal/internal/cleanup"
	"superlocal/internal/config"
	"superlocal/internal/httpapi"
	"superlocal/internal/hub"
	"superlocal/internal/store"
	"superlocal/internal/tlscert"
	"superlocal/internal/turncred"
	"superlocal/internal/turnserver"
)

func main() {
	cfg, err := config.FromArgs(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "superlocal.db"))
	if err != nil {
		log.Fatal(err)
	}
	username, password, err := auth.EnsureAdmin(db, cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	if err := cleanup.Run(db, cfg.DataDir, time.Now()); err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(48 * time.Hour)
	go func() {
		for range ticker.C {
			if err := cleanup.Run(db, cfg.DataDir, time.Now()); err != nil {
				log.Printf("清理失败: %v", err)
			}
		}
	}()

	hosts := lanIPv4()
	var calls *call.Session
	var turnServer interface{ Close() error }
	if cfg.TLS {
		if _, err := tlscert.LoadOrCreate(cfg.DataDir, hosts, time.Now()); err != nil {
			log.Fatalf("证书写入失败: %v", err)
		}
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Fatal(err)
		}
		relay := net.ParseIP("127.0.0.1")
		if len(hosts) > 0 {
			if ip := net.ParseIP(hosts[0]); ip != nil {
				relay = ip
			}
		}
		port := listenPort(cfg.Addr)
		started, err := turnserver.Listen(":"+port, relay, secret)
		if err != nil {
			log.Fatalf("TURN 端口绑定失败: %v", err)
		}
		turnServer = started
		calls = call.New(time.Now, store.NewID, func(userID string) call.Turn {
			username, credential := turncred.Mint(secret, userID, time.Now())
			return call.Turn{URLs: turncred.URLs(port, hosts), Username: username, Credential: credential}
		})
	}
	fmt.Print(banner(cfg.Addr, username, password, hosts, cfg.TLS))
	handler := httpapi.WithStatic(httpapi.New(httpapi.Deps{
		DB:           db,
		Hub:          hub.New(),
		DataDir:      cfg.DataDir,
		MaxUpload:    cfg.MaxUpload,
		Limiter:      auth.NewLimiter(),
		CallsEnabled: cfg.TLS,
		Calls:        calls,
	}))
	var serveErr error
	if cfg.TLS {
		cert, err := tlscert.LoadOrCreate(cfg.DataDir, hosts, time.Now())
		if err != nil {
			log.Fatalf("证书写入失败: %v", err)
		}
		ln, err := tls.Listen("tcp", cfg.Addr, &tls.Config{Certificates: []tls.Certificate{cert}})
		if err != nil {
			log.Fatal(err)
		}
		serveErr = http.Serve(ln, handler)
	} else {
		serveErr = http.ListenAndServe(cfg.Addr, handler)
	}
	ticker.Stop()
	if turnServer != nil {
		_ = turnServer.Close()
	}
	_ = db.SQL().Close()
	log.Fatal(serveErr)
}

// banner 列出回环地址和传入的局域网 IPv4 上的聊天页、管理页，以及管理员账号和密码。
// port 是监听地址，例如 :8080。本函数不绑定端口。
func banner(port, username, password string, addrs []string, secure bool) string {
	p := listenPort(port)
	hosts := make([]string, 0, 1+len(addrs))
	hosts = append(hosts, "127.0.0.1")
	hosts = append(hosts, addrs...)
	scheme := "http"
	if secure {
		scheme = "https"
	}

	var b strings.Builder
	for _, host := range hosts {
		fmt.Fprintf(&b, "%s://%s:%s/\n", scheme, host, p)
		fmt.Fprintf(&b, "%s://%s:%s/admin\n", scheme, host, p)
	}
	fmt.Fprintf(&b, "账号 %s\n密码 %s\n", username, password)
	return b.String()
}

func listenPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err == nil && port != "" {
		return port
	}
	return strings.TrimPrefix(addr, ":")
}

func lanIPv4() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, addr := range addrs {
		ip := ipFromAddr(addr)
		if ip == nil || ip.IsLoopback() {
			continue
		}
		v4 := ip.To4()
		if v4 == nil {
			continue
		}
		s := v4.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func ipFromAddr(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	default:
		return nil
	}
}
