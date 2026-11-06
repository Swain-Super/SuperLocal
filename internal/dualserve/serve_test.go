package dualserve_test

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"superlocal/internal/dualserve"
	"superlocal/internal/tlscert"
)

func TestServePlainAndTLS(t *testing.T) {
	cert, err := tlscert.LoadOrCreate(t.TempDir(), nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		_ = dualserve.Serve(ln, cert, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS != nil {
				_, _ = io.WriteString(w, "tls")
				return
			}
			_, _ = io.WriteString(w, "plain")
		}))
	}()

	base := ln.Addr().String()
	plain, err := http.Get("http://" + base + "/")
	if err != nil {
		t.Fatal(err)
	}
	plainBody, _ := io.ReadAll(plain.Body)
	plain.Body.Close()
	if plain.StatusCode != 200 || string(plainBody) != "plain" {
		t.Fatalf("plain status=%d body=%q", plain.StatusCode, plainBody)
	}

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	secure, err := client.Get("https://" + base + "/")
	if err != nil {
		t.Fatal(err)
	}
	secureBody, _ := io.ReadAll(secure.Body)
	secure.Body.Close()
	if secure.StatusCode != 200 || string(secureBody) != "tls" {
		t.Fatalf("tls status=%d body=%q", secure.StatusCode, secureBody)
	}
}

func TestServeEmptyConnectionCloses(t *testing.T) {
	cert, err := tlscert.LoadOrCreate(t.TempDir(), nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		_ = dualserve.Serve(ln, cert, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	}()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}
