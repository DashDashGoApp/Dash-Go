package weather

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var errUnsafeWeatherEndpoint = errors.New("custom weather endpoint is not permitted")

var weatherPrivatePrefixes = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("fc00::/7"),
}

// Non-public special-use destinations stay blocked even when the user opts in
// to an RFC1918/ULA custom provider. In particular, private opt-in must never
// become access to loopback, link-local metadata, CGNAT, benchmark, or
// documentation ranges.
var weatherDeniedPrefixes = []netip.Prefix{
	// IPv4 special-purpose, non-routable, translation, documentation, and
	// infrastructure-only ranges. Private RFC1918 ranges are handled separately
	// so the explicit private-endpoint opt-in can allow only those ranges.
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.31.196.0/24"),
	netip.MustParsePrefix("192.52.193.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.175.48.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),

	// IPv6 special-purpose and transition ranges. Known IPv4 translation
	// prefixes are denied wholesale so a translated metadata/link-local address
	// cannot bypass the corresponding IPv4 policy.
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("100:0:0:1::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("2620:4f:8000::/48"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func customWeatherHTTPClient(target *url.URL, allowPrivate bool) (*http.Client, error) {
	if err := validateCustomWeatherURL(target, allowPrivate); err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("%w: no resolved address", errUnsafeWeatherEndpoint)
		}
		for _, item := range ips {
			if !weatherDestinationAllowed(item.IP, allowPrivate) {
				return nil, fmt.Errorf("%w: resolved address %s", errUnsafeWeatherEndpoint, item.IP)
			}
		}

		// Every candidate above was validated before the first connection is
		// attempted, preserving the rebinding defense while retaining normal
		// IPv6/IPv4 fallback on mixed home networks.
		var failures []error
		for _, item := range ips {
			attemptCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			conn, dialErr := dialer.DialContext(attemptCtx, network, net.JoinHostPort(item.IP.String(), port))
			cancel()
			if dialErr == nil {
				return conn, nil
			}
			failures = append(failures, fmt.Errorf("%s: %w", item.IP, dialErr))
		}
		return nil, fmt.Errorf("weather endpoint connection failed: %w", errors.Join(failures...))
	}
	originalHost := strings.ToLower(target.Hostname())
	client := &http.Client{Timeout: 20 * time.Second, Transport: transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many weather endpoint redirects")
		}
		if strings.ToLower(req.URL.Hostname()) != originalHost {
			return errors.New("custom weather endpoint redirected to a different host")
		}
		return validateCustomWeatherURL(req.URL, allowPrivate)
	}
	return client, nil
}

func validateCustomWeatherURL(u *url.URL, allowPrivate bool) error {
	if u == nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errUnsafeWeatherEndpoint
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && !(allowPrivate && scheme == "http") {
		return fmt.Errorf("%w: HTTPS is required", errUnsafeWeatherEndpoint)
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !weatherDestinationAllowed(ip, allowPrivate) {
		return fmt.Errorf("%w: address %s", errUnsafeWeatherEndpoint, ip)
	}
	return nil
}

func weatherDestinationAllowed(ip net.IP, allowPrivate bool) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range weatherDeniedPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	for _, prefix := range weatherPrivatePrefixes {
		if prefix.Contains(addr) {
			return allowPrivate
		}
	}
	return addr.IsGlobalUnicast()
}
