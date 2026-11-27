package httpapi

import (
	"fmt"
	"net"
	"net/http"
)

func (s *server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "只支持 GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": JoinURL(r.Host, shareableIPv4())})
}

func JoinURL(requestHost string, ips []string) string {
	host, port, err := net.SplitHostPort(requestHost)
	if err != nil || port == "" {
		return ""
	}
	chosen := ""
	if ip := net.ParseIP(host); isShareableIPv4(ip) {
		chosen = ip.String()
	}
	if chosen == "" {
		for _, raw := range ips {
			if isShareableIPv4(net.ParseIP(raw)) {
				chosen = net.ParseIP(raw).String()
				break
			}
		}
	}
	if chosen == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:%s/", chosen, port)
}

func shareableIPv4() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	out := []string{}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || !isShareableIPv4(ipnet.IP) {
			continue
		}
		out = append(out, ipnet.IP.String())
	}
	return out
}

func isShareableIPv4(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() {
		return false
	}
	return true
}
