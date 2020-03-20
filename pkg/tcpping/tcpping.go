package tcpping

import (
	"net"
	"time"
)

// TCPPinger repeatedly attempts to open a TCP connection
type TCPPinger struct {
	Count  int
	Period time.Duration
}

// WaitPort repeatedly attempts to open a TCP connection
func (p TCPPinger) WaitPort(ip net.IP, port string) error {
	address := net.JoinHostPort(ip.String(), port)

	ticker := time.NewTicker(p.Period)
	defer ticker.Stop()

	var lastErr error
	for i := 0; i < p.Count; i++ {
		conn, err := net.DialTimeout("tcp", address, p.Period)
		if err == nil {
			defer conn.Close()
			return nil
		}
		if i < p.Count-1 {
			<-ticker.C
		}
		lastErr = err
	}

	return lastErr
}
