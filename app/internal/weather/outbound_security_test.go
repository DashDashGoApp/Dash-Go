package weather

import (
	"net"
	"net/url"
	"testing"
)

func TestCustomWeatherURLPolicy(t *testing.T) {
	cases := []struct {
		name, raw   string
		private, ok bool
	}{
		{"public https", "https://weather.example", false, true},
		{"public http", "http://weather.example", false, false},
		{"loopback", "https://127.0.0.1", true, false},
		{"link local", "https://169.254.169.254", true, false},
		{"private default", "https://192.168.1.8", false, false},
		{"private opt in", "http://192.168.1.8", true, true},
		{"credentials", "https://user:pass@weather.example", false, false},
		{"deprecated relay", "https://192.88.99.2", true, false},
		{"well known nat64", "https://[64:ff9b::a9fe:a9fe]", true, false},
		{"local nat64", "https://[64:ff9b:1::1]", true, false},
		{"documentation v6", "https://[3fff::1]", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, _ := url.Parse(tc.raw)
			err := validateCustomWeatherURL(u, tc.private)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v ok=%v", err, tc.ok)
			}
		})
	}
}

func TestWeatherDestinationPolicy(t *testing.T) {
	allowed := []struct {
		address string
		private bool
	}{
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2606:4700:4700::1111", false},
		{"192.168.1.1", true},
		{"10.0.0.8", true},
		{"fd00::8", true},
	}
	for _, tc := range allowed {
		if !weatherDestinationAllowed(net.ParseIP(tc.address), tc.private) {
			t.Errorf("expected %s private=%v to be allowed", tc.address, tc.private)
		}
	}

	denied := []string{
		"0.0.0.0", "127.0.0.1", "169.254.169.254", "100.64.0.1",
		"192.0.0.9", "192.0.2.1", "192.31.196.1", "192.52.193.1",
		"192.88.99.2", "192.175.48.1", "198.18.0.1", "198.51.100.1",
		"203.0.113.1", "224.0.0.1", "240.0.0.1", "::1", "::192.0.2.1",
		"64:ff9b::a9fe:a9fe", "64:ff9b:1::1", "100::1", "100:0:0:1::1",
		"2001::1", "2001:db8::1", "2002:c000:0201::1", "2620:4f:8000::1",
		"3fff::1", "5f00::1", "fec0::1", "fe80::1", "ff02::1",
		"::ffff:169.254.169.254",
	}
	for _, address := range denied {
		for _, private := range []bool{false, true} {
			if weatherDestinationAllowed(net.ParseIP(address), private) {
				t.Errorf("unsafe address %s allowed with private=%v", address, private)
			}
		}
	}

	for _, address := range []string{"10.0.0.1", "172.16.0.1", "192.168.1.1", "fd00::1"} {
		if weatherDestinationAllowed(net.ParseIP(address), false) {
			t.Errorf("private address %s allowed without opt-in", address)
		}
	}
}
