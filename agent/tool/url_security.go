package tool

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const urlLookupTimeout = 5 * time.Second

var errURLNotPublic = errors.New("requests to private, loopback, link-local, or metadata addresses are not allowed")

// urlSecurityAllowPrivate is only for tests (httptest servers listen on loopback).
var urlSecurityAllowPrivate bool

// validatePublicHTTPURL ensures the URL uses http/https and resolves to only public addresses.
func validatePublicHTTPURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errors.New("missing url")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("only http and https URLs are allowed")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, errors.New("url host is empty")
	}
	if parsed.User != nil {
		return nil, errors.New("url must not include userinfo")
	}

	if !urlSecurityAllowPrivate {
		host := parsed.Hostname()
		if isBlockedHostname(host) {
			return nil, errURLNotPublic
		}

		if ip := net.ParseIP(host); ip != nil {
			if isDisallowedIP(ip) {
				return nil, errURLNotPublic
			}
			return parsed, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), urlLookupTimeout)
		defer cancel()

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve host %q: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("resolve host %q: no addresses", host)
		}
		for _, addr := range ips {
			if isDisallowedIP(addr.IP) {
				return nil, errURLNotPublic
			}
		}
	}
	return parsed, nil
}

func isBlockedHostname(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "localhost.localdomain", "ip6-localhost", "ip6-loopback":
		return true
	default:
		return false
	}
}

func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	ip = ip.To16()
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// Common cloud metadata endpoint
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	return false
}
