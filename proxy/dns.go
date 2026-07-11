package proxy

import (
	"encoding/binary"
	"math/rand"
	"net"
	"sync"
	"time"
)

type DNSCache struct {
	upstream string
	mu       sync.RWMutex
	entries  [8192]dnsSlot
}

type dnsSlot struct {
	hash   uint64
	name   string
	ip     string
	expiry int64
}

func NewDNSCache(upstream string) *DNSCache {
	return &DNSCache{upstream: upstream}
}

func (c *DNSCache) Prewarm() {
	domains := []string{"google.com", "youtube.com", "facebook.com", "wikipedia.org", "amazon.com", "cloudflare.com", "github.com"}
	for _, d := range domains {
		c.CacheLookup(d)
		time.Sleep(50 * time.Millisecond)
	}
}

func (c *DNSCache) Lookup(host string) string {
	h := hashStr(host)
	now := time.Now().UnixNano()
	c.mu.RLock()
	for i := 0; i < 8; i++ {
		s := &c.entries[(int(h)+i)&8191]
		if s.hash == h && s.name == host && now < s.expiry {
			ip := s.ip
			c.mu.RUnlock()
			return ip
		}
	}
	c.mu.RUnlock()
	return ""
}

func (c *DNSCache) CacheLookup(host string) {
	ip, ttl, err := c.resolve(host)
	if err != nil || ip == "" || ttl < 1 {
		return
	}
	h := hashStr(host)
	exp := time.Now().UnixNano() + int64(ttl)*1e9
	c.mu.Lock()
	for i := 0; i < 8; i++ {
		s := &c.entries[(int(h)+i)&8191]
		if s.hash == 0 && len(s.name) == 0 {
			s.hash = h; s.name = host; s.ip = ip; s.expiry = exp
			c.mu.Unlock()
			return
		}
		if s.hash == h && s.name == host {
			s.ip = ip; s.expiry = exp
			c.mu.Unlock()
			return
		}
	}
	idx := int(h) & 8191
	c.entries[idx].hash = h; c.entries[idx].name = host; c.entries[idx].ip = ip; c.entries[idx].expiry = exp
	c.mu.Unlock()
}

func hashStr(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func (c *DNSCache) resolve(host string) (string, uint32, error) {
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), 3600, nil
	}

	id := uint16(rand.Intn(65536))
	q := dnsBufPool.Get().(*[]byte)
	defer dnsBufPool.Put(q)
	buf := *q

	binary.BigEndian.PutUint16(buf[0:2], id)
	buf[2] = 1
	binary.BigEndian.PutUint16(buf[4:6], 1)

	pos := 12
	for _, label := range splitLabels(host) {
		buf[pos] = byte(len(label))
		pos++
		copy(buf[pos:], label)
		pos += len(label)
	}
	buf[pos] = 0; pos++
	binary.BigEndian.PutUint16(buf[pos:pos+2], 1)
	binary.BigEndian.PutUint16(buf[pos+2:pos+4], 1)
	pos += 4

	conn, err := net.DialTimeout("udp", c.upstream, 3*time.Second)
	if err != nil {
		return "", 0, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(buf[:pos]); err != nil {
		return "", 0, err
	}

	n, err := conn.Read(buf)
	if err != nil || n < 12 {
		return "", 0, err
	}

	anCount := int(binary.BigEndian.Uint16(buf[6:8]))
	if anCount == 0 {
		return "", 0, nil
	}

	off := 12
	off = skipDNSName(buf, off, n)
	off += 4
	for i := 0; i < anCount; i++ {
		if off >= n {
			break
		}
		off = skipDNSName(buf, off, n)
		if off+10 > n {
			break
		}
		rtype := int(binary.BigEndian.Uint16(buf[off:]))
		rclass := int(binary.BigEndian.Uint16(buf[off+2:]))
		ttl := binary.BigEndian.Uint32(buf[off+4:])
		rdlen := int(binary.BigEndian.Uint16(buf[off+8:]))
		off += 10
		if rtype == 1 && rclass == 1 && rdlen == 4 && off+4 <= n {
			return net.IP(buf[off:off+4]).String(), ttl, nil
		}
		off += rdlen
	}
	return "", 0, nil
}

var dnsBufPool = sync.Pool{
	New: func() any { b := make([]byte, 512); return &b },
}

func splitLabels(host string) []string {
	var labels []string
	start := 0
	for i := 0; i <= len(host); i++ {
		if i == len(host) || host[i] == '.' {
			if i > start {
				labels = append(labels, host[start:i])
			}
			start = i + 1
		}
	}
	return labels
}

func skipDNSName(data []byte, off, max int) int {
	for off < max {
		b := data[off]
		if b == 0 {
			return off + 1
		}
		if b&0xC0 == 0xC0 {
			return off + 2
		}
		off += int(b) + 1
	}
	return off
}
