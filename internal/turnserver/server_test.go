package turnserver

import (
	"net"
	"testing"
)

func TestListenOpensAndCloses(t *testing.T) {
	server, err := Listen(":0", net.ParseIP("127.0.0.1"), []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}
