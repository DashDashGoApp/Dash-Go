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
	if weatherDestinationAllowed(net.ParseIP("127.0.0.1"), true) {
		t.Fatal("loopback allowed")
	}
	if weatherDestinationAllowed(net.ParseIP("169.254.1.1"), true) {
		t.Fatal("link-local allowed")
	}
	if weatherDestinationAllowed(net.ParseIP("192.168.1.1"), false) {
		t.Fatal("private allowed by default")
	}
	if !weatherDestinationAllowed(net.ParseIP("192.168.1.1"), true) {
		t.Fatal("private opt-in rejected")
	}
	if !weatherDestinationAllowed(net.ParseIP("8.8.8.8"), false) {
		t.Fatal("public address rejected")
	}
}
