package proxy

import (
	"bufio"
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func HandleHTTP(conn net.Conn, dns *DNSCache) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	// Basic auth check if proxy auth configured
	if proxyAuth != "" {
		auth := req.Header.Get("Proxy-Authorization")
		if auth == "" || !checkProxyAuth(auth) {
			resp := &http.Response{
				StatusCode: 407,
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header:     http.Header{"Proxy-Authenticate": []string{"Basic realm=\"netproxy\""}},
			}
			resp.Write(conn)
			return
		}
	}

	if req.Method == http.MethodConnect {
		handleConnect(conn, req, dns)
	} else {
		handleHTTP(conn, req, dns)
	}
}

func handleConnect(conn net.Conn, req *http.Request, dns *DNSCache) {
	host := req.Host
	if !hasPort(host) {
		host = host + ":443"
	}

	target, err := dialWithCache(host, dns)
	if err != nil {
		log.Printf("CONNECT dial %s: %v", host, err)
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer target.Close()

	conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { io.Copy(target, conn); wg.Done() }()
	go func() { io.Copy(conn, target); wg.Done() }()
	wg.Wait()
}

func handleHTTP(conn net.Conn, req *http.Request, dns *DNSCache) {
	host := req.URL.Host
	if !hasPort(host) {
		port := "80"
		if req.URL.Scheme == "https" {
			port = "443"
		}
		host = host + ":" + port
	}

	target, err := dialWithCache(host, dns)
	if err != nil {
		log.Printf("HTTP dial %s: %v", host, err)
		resp := &http.Response{
			StatusCode: 502,
			ProtoMajor: 1,
			ProtoMinor: 1,
			Body:       io.NopCloser(strings.NewReader("Bad Gateway")),
		}
		resp.Write(conn)
		return
	}
	defer target.Close()

	// Rewrite URL to remove scheme+host for proxy request
	req.RequestURI = req.URL.RequestURI()
	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authorization")
	req.Write(target)

	resp, err := http.ReadResponse(bufio.NewReader(target), req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	resp.Write(conn)
}

func dialWithCache(host string, dns *DNSCache) (net.Conn, error) {
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		return nil, err
	}

	// Try DNS cache
	if ip := dns.Lookup(hostname); ip != "" {
		addr := net.JoinHostPort(ip, port)
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err == nil {
			return conn, nil
		}
	}

	// Fallback to normal DNS
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err == nil {
		// Cache the resolved IP asynchronously
		go dns.CacheLookup(hostname)
	}
	return conn, err
}

func hasPort(host string) bool {
	return strings.LastIndex(host, ":") > strings.LastIndex(host, "]")
}

var proxyAuth string

func SetProxyAuth(auth string) {
	proxyAuth = auth
}

func checkProxyAuth(authHeader string) bool {
	if !strings.HasPrefix(authHeader, "Basic ") {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(authHeader[6:])
	if err != nil {
		return false
	}
	return string(decoded) == proxyAuth
}
