package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/tundefund0-gif/netproxy/proxy"
)

func main() {
	httpPort := flag.Int("http", 8080, "HTTP proxy port")
	socksPort := flag.Int("socks", 1080, "SOCKS5 proxy port")
	dnsAddr := flag.String("dns", "1.1.1.1:53", "Upstream DNS")
	bind := flag.String("bind", "0.0.0.0", "Bind address")
	auth := flag.String("auth", "", "Proxy auth user:pass (optional)")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	if *auth != "" {
		parts := strings.SplitN(*auth, ":", 2)
		if len(parts) != 2 {
			log.Fatal("auth must be user:pass")
		}
		authStr := parts[0] + ":" + parts[1]
		proxy.SetProxyAuth(authStr)
		proxy.SetSocksAuth(authStr)
		log.Printf("Auth enabled for user %s", parts[0])
	}

	dnsCache := proxy.NewDNSCache(*dnsAddr)
	go dnsCache.Prewarm()

	// HTTP proxy
	go func() {
		addr := fmt.Sprintf("%s:%d", *bind, *httpPort)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("HTTP proxy on %s: %v", addr, err)
		}
		log.Printf("HTTP proxy on %s", addr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("HTTP accept: %v", err)
				continue
			}
			go proxy.HandleHTTP(conn, dnsCache)
		}
	}()

	// SOCKS5 proxy
	go func() {
		addr := fmt.Sprintf("%s:%d", *bind, *socksPort)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("SOCKS5 on %s: %v", addr, err)
		}
		log.Printf("SOCKS5 proxy on %s", addr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("SOCKS5 accept: %v", err)
				continue
			}
			go proxy.HandleSOCKS5(conn, dnsCache)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	log.Println("Shutdown")
}
