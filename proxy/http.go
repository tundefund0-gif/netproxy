package proxy

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var proxyAuth string

func SetProxyAuth(auth string) { proxyAuth = auth }

var bufPool = sync.Pool{New: func() any { b := make([]byte, 32768); return &b }}

func HandleHTTP(conn net.Conn, dns *DNSCache) {
	ConnTotal.Add(1)
	ConnActive.Add(1)
	defer ConnActive.Add(-1)
	defer conn.Close()

	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetNoDelay(true)
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(30 * time.Second)
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	br := bufio.NewReaderSize(conn, 4096)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

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

	if Verbose.Load() {
		log.Printf("> HTTP %s %s", req.Method, req.Host)
	}

	// Built-in status endpoint (direct request, not proxy)
	if req.Method == "GET" && !req.URL.IsAbs() && req.URL.Path == "/status" {
		statusHandler(conn)
		return
	}

	if req.Method == http.MethodConnect {
		handleConnect(conn, req, dns)
	} else {
		handleHTTP(conn, req, dns, br)
	}
}

func statusHandler(conn net.Conn) {
	body := fmt.Sprintf(`{"connections_total":%d,"connections_active":%d,"dns_hits":%d,"dns_misses":%d}`,
		ConnTotal.Load(), ConnActive.Load(), dnsCacheHits.Load(), dnsCacheMisses.Load())
	resp := &http.Response{
		StatusCode: 200,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	resp.ContentLength = int64(len(body))
	resp.Write(conn)
}

var dnsCacheHits, dnsCacheMisses atomic.Int64

func handleConnect(conn net.Conn, req *http.Request, dns *DNSCache) {
	host := req.Host
	if !hasPort(host) {
		host += ":443"
	}

	target, err := dialTarget(host, dns)
	if err != nil {
		if Verbose.Load() {
			log.Printf("! CONNECT dial %s: %v", host, err)
		}
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer target.Close()
	target.(*net.TCPConn).SetNoDelay(true)

	conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	tunnel(conn, target)
}

func handleHTTP(conn net.Conn, req *http.Request, dns *DNSCache, br *bufio.Reader) {
	host := req.URL.Host
	if !hasPort(host) {
		port := "80"
		if req.URL.Scheme == "https" {
			port = "443"
		}
		host = host + ":" + port
	}

	target, err := dialTarget(host, dns)
	if err != nil {
		if Verbose.Load() {
			log.Printf("! HTTP dial %s: %v", host, err)
		}
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
	target.(*net.TCPConn).SetNoDelay(true)

	req.RequestURI = req.URL.RequestURI()
	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authorization")
	if err := req.Write(target); err != nil {
		return
	}

	resp, err := http.ReadResponse(bufio.NewReaderSize(target, 4096), req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	resp.Write(conn)
}

func tunnel(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		buf := *bufPool.Get().(*[]byte)
		defer bufPool.Put(&buf)
		n, _ := io.CopyBuffer(a, b, buf)
		BytesSent.Add(n)
		wg.Done()
	}()
	go func() {
		buf := *bufPool.Get().(*[]byte)
		defer bufPool.Put(&buf)
		n, _ := io.CopyBuffer(b, a, buf)
		BytesRecv.Add(n)
		wg.Done()
	}()
	wg.Wait()
}

func dialTarget(host string, dns *DNSCache) (net.Conn, error) {
	hostname, port, err := net.SplitHostPort(host)
	if err != nil {
		return nil, err
	}

	if ip := dns.Lookup(hostname); ip != "" {
		dnsCacheHits.Add(1)
		addr := net.JoinHostPort(ip, port)
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err == nil {
			return conn, nil
		}
	}

	dnsCacheMisses.Add(1)
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err == nil {
		go dns.CacheLookup(hostname)
	}
	return conn, err
}

func hasPort(host string) bool {
	return strings.LastIndex(host, ":") > strings.LastIndex(host, "]")
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
