package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/tundefund0-gif/netproxy/proxy"
)

func main() {
	httpPort := flag.Int("http", 8080, "HTTP proxy port")
	socksPort := flag.Int("socks", 1080, "SOCKS5 proxy port")
	dnsAddr := flag.String("dns", "1.1.1.1:53", "Upstream DNS")
	bind := flag.String("bind", "0.0.0.0", "Bind address")
	auth := flag.String("auth", "", "Proxy auth user:pass")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime)

	if *auth != "" {
		parts := strings.SplitN(*auth, ":", 2)
		if len(parts) != 2 {
			log.Fatal("auth must be user:pass")
		}
		authStr := parts[0] + ":" + parts[1]
		proxy.SetProxyAuth(authStr)
		proxy.SetSocksAuth(authStr)
	}

	dnsCache := proxy.NewDNSCache(*dnsAddr)
	go dnsCache.Prewarm()

	startProxy("HTTP", *bind, *httpPort, func(conn net.Conn) {
		proxy.HandleHTTP(conn, dnsCache)
	})

	startProxy("SOCKS5", *bind, *socksPort, func(conn net.Conn) {
		proxy.HandleSOCKS5(conn, dnsCache)
	})

	log.Printf("netproxy ready — http :%d, socks5 :%d, dns %s", *httpPort, *socksPort, *dnsAddr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	log.Println("shutdown")
	time.Sleep(100 * time.Millisecond)
}

func startProxy(name, bind string, port int, handler func(net.Conn)) {
	addr := fmt.Sprintf("%s:%d", bind, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("%s on %s: %v", name, addr, err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("%s accept: %v", name, err)
				continue
			}
			go handler(conn)
		}
	}()
}
