package proxy

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	socksVer5         = 5
	socksCmdConnect   = 1
	socksAtypIPv4    = 1
	socksAtypDomain  = 3
	socksAtypIPv6    = 4
	socksAuthNone    = 0
	socksAuthUserPass = 2
)

var socksAuth string // "user:pass" or empty

func SetSocksAuth(auth string) {
	socksAuth = auth
}

func HandleSOCKS5(conn net.Conn, dns *DNSCache) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	buf := make([]byte, 257)
	_, err := io.ReadFull(conn, buf[:2])
	if err != nil || buf[0] != socksVer5 {
		return
	}
	nMethods := int(buf[1])
	if nMethods < 1 || nMethods > 255 {
		return
	}
	_, err = io.ReadFull(conn, buf[:nMethods])
	if err != nil {
		return
	}

	hasUserPass := false
	hasNone := false
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
		_, err = io.ReadFull(conn, buf[:2])
		if err != nil || buf[0] != 1 {
			return
		}
		ulen := int(buf[1])
		if ulen < 1 || ulen > 255 {
			return
		}
		_, err = io.ReadFull(conn, buf[:ulen])
		if err != nil {
			return
		}
		user := string(buf[:ulen])
		_, err = io.ReadFull(conn, buf[:1])
		if err != nil {
			return
		}
		plen := int(buf[0])
		if plen < 1 || plen > 255 {
			return
		}
		_, err = io.ReadFull(conn, buf[:plen])
		if err != nil {
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

	_, err = io.ReadFull(conn, buf[:4])
	if err != nil {
		return
	}
	ver, cmd, atyp := buf[0], buf[1], buf[3]
	if ver != socksVer5 || cmd != socksCmdConnect {
		return
	}

	var host string
	var port int

	switch atyp {
	case socksAtypIPv4:
		_, err = io.ReadFull(conn, buf[:4])
		if err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
	case socksAtypDomain:
		_, err = io.ReadFull(conn, buf[:1])
		if err != nil {
			return
		}
		domainLen := int(buf[0])
		if domainLen > 255 {
			return
		}
		_, err = io.ReadFull(conn, buf[:domainLen])
		if err != nil {
			return
		}
		host = string(buf[:domainLen])
	case socksAtypIPv6:
		_, err = io.ReadFull(conn, buf[:16])
		if err != nil {
			return
		}
		host = net.IP(buf[:16]).String()
	default:
		return
	}

	_, err = io.ReadFull(conn, buf[:2])
	if err != nil {
		return
	}
	port = int(binary.BigEndian.Uint16(buf[:2]))

	addr := net.JoinHostPort(host, strconv.Itoa(port))

	if atyp == socksAtypDomain {
		if ip := dns.Lookup(host); ip != "" {
			addr = net.JoinHostPort(ip, strconv.Itoa(port))
		} else {
			go dns.CacheLookup(host)
		}
	}

	target, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		log.Printf("SOCKS5 dial %s: %v", addr, err)
		conn.Write([]byte{socksVer5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	local := target.LocalAddr().(*net.TCPAddr)
	resp := []byte{socksVer5, 0, 0, socksAtypIPv4}
	resp = append(resp, local.IP.To4()...)
	resp = append(resp, byte(local.Port>>8), byte(local.Port))
	conn.Write(resp)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { io.Copy(target, conn); wg.Done() }()
	go func() { io.Copy(conn, target); wg.Done() }()
	wg.Wait()
}
