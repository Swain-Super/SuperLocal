package dualserve

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
)

// Serve 在 ln 上同时接受 HTTP 和 HTTPS。第一条字节是 0x16 时按 TLS 处理，否则按 HTTP 处理。
// 读失败或没有字节时关闭这条连接。cert 是已经准备好的证书。
func Serve(ln net.Listener, cert tls.Certificate, h http.Handler) error {
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go serveConn(conn, cfg, h)
	}
}

func serveConn(conn net.Conn, cfg *tls.Config, h http.Handler) {
	defer conn.Close()
	buf := make([]byte, 1)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return
	}
	shot := newOneShot(conn.LocalAddr())
	base := &closeOnceConn{
		Conn:    &prefixConn{Conn: conn, rest: append([]byte(nil), buf[:n]...)},
		onClose: shot.Close,
	}
	var use net.Conn = base
	if buf[0] == 0x16 {
		use = tls.Server(base, cfg)
	}
	shot.conn = use
	_ = http.Serve(shot, h)
}

type prefixConn struct {
	net.Conn
	rest []byte
}

func (p *prefixConn) Read(b []byte) (int, error) {
	if len(p.rest) > 0 {
		n := copy(b, p.rest)
		p.rest = p.rest[n:]
		return n, nil
	}
	return p.Conn.Read(b)
}

type oneShot struct {
	conn   net.Conn
	addr   net.Addr
	closed chan struct{}
	once   sync.Once
}

func newOneShot(addr net.Addr) *oneShot {
	return &oneShot{addr: addr, closed: make(chan struct{})}
}

func (l *oneShot) Accept() (net.Conn, error) {
	if l.conn != nil {
		raw := l.conn
		l.conn = nil
		return raw, nil
	}
	<-l.closed
	return nil, net.ErrClosed
}

func (l *oneShot) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *oneShot) Addr() net.Addr {
	return l.addr
}

type closeOnceConn struct {
	net.Conn
	once    sync.Once
	onClose func() error
}

func (c *closeOnceConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { _ = c.onClose() })
	return err
}
