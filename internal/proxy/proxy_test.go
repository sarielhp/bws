package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestProxyConnectAndHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("proxied-response"))
	}))
	defer ts.Close()

	p, err := Start()
	if err != nil {
		t.Fatalf("failed to start proxy: %v", err)
	}
	defer p.Close()

	proxyURL, err := url.Parse(p.URL())
	if err != nil {
		t.Fatalf("failed to parse proxy URL: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed GET request through proxy: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	if string(body) != "proxied-response" {
		t.Errorf("expected proxied-response, got %s", string(body))
	}
}
