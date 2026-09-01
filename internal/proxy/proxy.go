package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Server represents an in-process HTTP/HTTPS forward proxy.
type Server struct {
	server   *http.Server
	listener net.Listener
	addr     string
}

// Start launches an ephemeral forward proxy on 127.0.0.1.
func Start() (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start proxy listener: %w", err)
	}

	s := &Server{
		listener: listener,
		addr:     listener.Addr().String(),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			handleConnect(w, r)
		} else {
			handleHTTP(w, r)
		}
	})

	s.server = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// ignore or log server error
		}
	}()

	return s, nil
}

// Addr returns the host:port address of the proxy.
func (s *Server) Addr() string {
	return s.addr
}

// URL returns the full http://host:port URL of the proxy.
func (s *Server) URL() string {
	return "http://" + s.addr
}

// Close gracefully stops the proxy server.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	// Prefer IPv4 to avoid ENETUNREACH in environments without default IPv6 routes
	destConn, err := dialer.Dial("tcp4", r.Host)
	if err != nil {
		destConn, err = dialer.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "HTTP Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(destConn, clientConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, destConn)
		done <- struct{}{}
	}()
	<-done
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Scheme == "" {
		r.URL.Scheme = "http"
	}
	if r.URL.Host == "" {
		r.URL.Host = r.Host
	}

	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for k, vv := range r.Header {
		if k == "Proxy-Connection" || k == "Proxy-Authenticate" || k == "Proxy-Authorization" {
			continue
		}
		for _, v := range vv {
			outReq.Header.Add(k, v)
		}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 15 * time.Second}
			conn, err := dialer.DialContext(ctx, "tcp4", addr)
			if err != nil {
				return dialer.DialContext(ctx, "tcp", addr)
			}
			return conn, nil
		},
	}

	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		// Log or ignore if connection is closed
	}
}
