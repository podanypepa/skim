package digest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSafeDialRejectsLocal(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "172.16.0.5", "169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "fc00::1", "fe80::1"} {
		if err := checkPublicIP(ip); err == nil {
			t.Errorf("%s should be rejected", ip)
		}
	}
	for _, ip := range []string{"93.184.216.34", "2606:4700::6810:84e5"} {
		if err := checkPublicIP(ip); err != nil {
			t.Errorf("%s should be allowed: %v", ip, err)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer s.Close()
	if _, err := newSafeClient().Get(s.URL); err == nil {
		t.Fatal("safe client must refuse loopback")
	}
}
