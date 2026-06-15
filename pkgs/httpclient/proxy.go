package httpclient

import (
	"net/http"
	"net/url"
	"strings"
)

// ClientWithOptionalProxy returns an http.Client that sends requests through the given proxy URL.
// proxyURL must include a scheme (e.g. http://127.0.0.1:7890). Returns nil if proxyURL is empty or invalid.
func ClientWithOptionalProxy(proxyURL string) *http.Client {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil || u.Scheme == "" {
		return nil
	}
	var t *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		t = dt.Clone()
	} else {
		t = &http.Transport{}
	}
	t.Proxy = http.ProxyURL(u)
	return &http.Client{Transport: t}
}
