package providers

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/Autumn-27/artex/proxygraph"
)

// TorProvider exposes the local TOR SOCKS5 proxy and can request fresh circuits
// via the TOR control port.
type TorProvider struct {
	SOCKSAddr   string // default: "127.0.0.1:9050"
	ControlAddr string // default: "127.0.0.1:9051" (TOR control port)
	ControlPass string // TOR HashedControlPassword (empty = no auth)
}

const torDefaultSOCKS = "127.0.0.1:9050"
const torDefaultControl = "127.0.0.1:9051"

func (t *TorProvider) socksAddr() string {
	if t.SOCKSAddr != "" {
		return t.SOCKSAddr
	}
	return torDefaultSOCKS
}

func (t *TorProvider) controlAddr() string {
	if t.ControlAddr != "" {
		return t.ControlAddr
	}
	return torDefaultControl
}

// Proxy returns a proxygraph.Proxy pointing at the local TOR SOCKS5 listener.
func (t *TorProvider) Proxy() proxygraph.Proxy {
	return proxygraph.Proxy{
		ID:     "tor-0",
		URL:    "socks5://" + t.socksAddr(),
		Type:   "tor",
		ASN:    "tor",
		Score:  1.0,
		Active: true,
	}
}

// NewCircuit sends SIGNAL NEWNYM to the TOR control port, requesting a new
// exit circuit. Returns an error when the control port is unreachable or
// authentication fails.
func (t *TorProvider) NewCircuit(ctx context.Context) error {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", t.controlAddr())
	if err != nil {
		return fmt.Errorf("tor control: connect %s: %w", t.controlAddr(), err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	r := bufio.NewReader(conn)

	// Read the banner.
	if _, err := r.ReadString('\n'); err != nil {
		return fmt.Errorf("tor control: read banner: %w", err)
	}

	// Authenticate.
	pass := t.ControlPass
	var authCmd string
	if pass != "" {
		authCmd = fmt.Sprintf("AUTHENTICATE \"%s\"\r\n", pass)
	} else {
		authCmd = "AUTHENTICATE\r\n"
	}
	if _, err := fmt.Fprint(conn, authCmd); err != nil {
		return fmt.Errorf("tor control: send auth: %w", err)
	}
	reply, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("tor control: read auth reply: %w", err)
	}
	if len(reply) < 3 || reply[:3] != "250" {
		return fmt.Errorf("tor control: auth rejected: %s", reply)
	}

	// Request new circuit.
	if _, err := fmt.Fprint(conn, "SIGNAL NEWNYM\r\n"); err != nil {
		return fmt.Errorf("tor control: send NEWNYM: %w", err)
	}
	reply, err = r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("tor control: read NEWNYM reply: %w", err)
	}
	if len(reply) < 3 || reply[:3] != "250" {
		return fmt.Errorf("tor control: NEWNYM rejected: %s", reply)
	}
	return nil
}
