package weather

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var errUnsafeWeatherEndpoint = errors.New("custom weather endpoint is not permitted")

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
		for _, item := range ips {
			if !weatherDestinationAllowed(item.IP, allowPrivate) {
				return nil, fmt.Errorf("%w: resolved address %s", errUnsafeWeatherEndpoint, item.IP)
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("%w: no resolved address", errUnsafeWeatherEndpoint)
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
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
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.IsPrivate() && !allowPrivate {
		return false
	}
	return true
}
