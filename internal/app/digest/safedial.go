package digest

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// checkPublicIP rejects addresses on the local machine or private
// networks, so feed content cannot make skim fetch internal services.
func checkPublicIP(host string) error {
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("not an IP: %q", host)
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || cgnat.Contains(ip) {
		return fmt.Errorf("refusing non-public address %s", ip)
	}
	return nil
}

// newSafeClient returns an HTTP client that only connects to public
// addresses (checked at dial time, so redirects and DNS rebinding are
// covered) and never uses a proxy.
func newSafeClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			return checkPublicIP(host)
		},
	}
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        10,
		},
	}
}
