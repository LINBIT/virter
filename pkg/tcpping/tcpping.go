package tcpping

import (
	"net"
	"time"

	"github.com/LINBIT/virter/pkg/actualtime"
)

// TCPPinger repeatedly attempts to open a TCP connection
type TCPPinger struct {
	Count  int
	Period time.Duration
}

// WaitPort repeatedly attempts to open a TCP connection
func (p TCPPinger) WaitPort(ip net.IP, port string) error {
	address := net.JoinHostPort(ip.String(), port)

	sshTry := func() error {
		conn, err := net.DialTimeout("tcp", address, p.Period)
		if err == nil {
			conn.Close()
		}
		return err
	}

	return actualtime.ActualTime{}.Ping(p.Count, p.Period, sshTry)
}
