package proxy

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"time"
)

const (
	socksVer5          = 5
	socksCmdConnect    = 1
	socksAtypIPv4     = 1
	socksAtypDomain   = 3
	socksAtypIPv6     = 4
	socksAuthNone     = 0
	socksAuthUserPass = 2
)

var socksAuth string

func SetSocksAuth(auth string) { socksAuth = auth }

func HandleSOCKS5(conn net.Conn, dns *DNSCache) {
	ConnTotal.Add(1)
	ConnActive.Add(1)
	defer ConnActive.Add(-1)
	defer conn.Close()

	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetNoDelay(true)
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(30 * time.Second)
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	buf := make([]byte, 257)

	if _, err := io.ReadFull(conn, buf[:2]); err != nil || buf[0] != socksVer5 {
		return
	}
	nMethods := int(buf[1])
	if nMethods < 1 || nMethods > 255 {
		return
	}
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil {
		return
	}

	hasUserPass, hasNone := false, false
	for i := 0; i < nMethods; i++ {
		switch buf[i] {
		case socksAuthUserPass:
			hasUserPass = true
		case socksAuthNone:
			hasNone = true
		}
	}

	if socksAuth != "" {
		if !hasUserPass {
			conn.Write([]byte{socksVer5, 0xFF})
			return
		}
		conn.Write([]byte{socksVer5, socksAuthUserPass})
		if _, err := io.ReadFull(conn, buf[:2]); err != nil || buf[0] != 1 {
			return
		}
		ulen := int(buf[1])
		if ulen < 1 || ulen > 255 {
			return
		}
		if _, err := io.ReadFull(conn, buf[:ulen]); err != nil {
			return
		}
		user := string(buf[:ulen])
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		plen := int(buf[0])
		if plen < 1 || plen > 255 {
			return
		}
		if _, err := io.ReadFull(conn, buf[:plen]); err != nil {
			return
		}
		pass := string(buf[:plen])
		if user+":"+pass != socksAuth {
			conn.Write([]byte{1, 1})
			return
		}
		conn.Write([]byte{1, 0})
	} else {
		if !hasNone {
			conn.Write([]byte{socksVer5, 0xFF})
			return
		}
		conn.Write([]byte{socksVer5, socksAuthNone})
	}

	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return
	}
	if buf[0] != socksVer5 || buf[1] != socksCmdConnect {
		return
	}
	atyp := buf[3]

	var host string
	var port int

	switch atyp {
	case socksAtypIPv4:
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
	case socksAtypDomain:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		domainLen := int(buf[0])
		if domainLen > 255 {
			return
		}
		if _, err := io.ReadFull(conn, buf[:domainLen]); err != nil {
			return
		}
		host = string(buf[:domainLen])
	case socksAtypIPv6:
		if _, err := io.ReadFull(conn, buf[:16]); err != nil {
			return
		}
		host = net.IP(buf[:16]).String()
	default:
		return
	}

	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	port = int(binary.BigEndian.Uint16(buf[:2]))
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	if Verbose.Load() {
		log.Printf("> SOCKS5 %s:%d", host, port)
	}

	if atyp == socksAtypDomain {
		if ip := dns.Lookup(host); ip != "" {
			dnsCacheHits.Add(1)
			addr = net.JoinHostPort(ip, strconv.Itoa(port))
		} else {
			dnsCacheMisses.Add(1)
			go dns.CacheLookup(host)
		}
	}

	target, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		if Verbose.Load() {
			log.Printf("! SOCKS5 dial %s: %v", addr, err)
		}
		conn.Write([]byte{socksVer5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()
	target.(*net.TCPConn).SetNoDelay(true)

	local := target.LocalAddr().(*net.TCPAddr)
	resp := []byte{socksVer5, 0, 0, socksAtypIPv4}
	resp = append(resp, local.IP.To4()...)
	resp = append(resp, byte(local.Port>>8), byte(local.Port))
	conn.Write(resp)

	tunnel(conn, target)
}
