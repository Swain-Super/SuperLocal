package turnserver

import (
	"net"
	"time"

	"github.com/pion/turn/v4"

	"superlocal/internal/turncred"
)

func Listen(addr string, relayIP net.IP, secret []byte) (*turn.Server, error) {
	if relayIP == nil {
		relayIP = net.ParseIP("127.0.0.1")
	}
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	server, err := turn.NewServer(turn.ServerConfig{
		Realm: "superlocal",
		AuthHandler: func(username, realm string, srcAddr net.Addr) ([]byte, bool) {
			if !turncred.Verify(secret, username, turncred.Password(secret, username), time.Now()) {
				return nil, false
			}
			return turn.GenerateAuthKey(username, realm, turncred.Password(secret, username)), true
		},
		PacketConnConfigs: []turn.PacketConnConfig{{
			PacketConn: conn,
			RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
				RelayAddress: relayIP,
				Address:      "0.0.0.0",
			},
		}},
	})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return server, nil
}
