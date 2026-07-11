package proxy

import (
	"encoding/binary"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type DNSCache struct {
	upstream string
	mu       sync.RWMutex
	entries  map[string]dnsEntry
}

type dnsEntry struct {
	ip     string
	expiry time.Time
}

func NewDNSCache(upstream string) *DNSCache {
	return &DNSCache{
		upstream: upstream,
		entries:  make(map[string]dnsEntry, 256),
	}
}

func (c *DNSCache) Prewarm() {
	domains := []string{"google.com", "youtube.com", "facebook.com", "wikipedia.org", "amazon.com"}
	for _, d := range domains {
		c.CacheLookup(d)
		time.Sleep(100 * time.Millisecond)
	}
	log.Printf("DNS cache prewarmed %d domains", len(domains))
}

func (c *DNSCache) Lookup(host string) string {
	c.mu.RLock()
	e, ok := c.entries[host]
	c.mu.RUnlock()
	if ok && time.Now().Before(e.expiry) {
		return e.ip
	}
	return ""
}

func (c *DNSCache) CacheLookup(host string) {
	ip, ttl, err := c.resolve(host)
	if err != nil || ip == "" {
		return
	}
	c.mu.Lock()
	c.entries[host] = dnsEntry{ip: ip, expiry: time.Now().Add(time.Duration(ttl) * time.Second)}
	c.mu.Unlock()
}

func (c *DNSCache) resolve(host string) (string, uint32, error) {
	// Build DNS query
	id := uint16(rand.Intn(65536))
	q := make([]byte, 12+len(host)+2+4)
	binary.BigEndian.PutUint16(q[0:2], id)
	q[2] = 1   // RD
	q[5] = 1   // QDCOUNT = 1

	// Encode domain name
	pos := 12
	for _, label := range splitLabels(host) {
		q[pos] = byte(len(label))
		pos++
		copy(q[pos:], label)
		pos += len(label)
	}
	q[pos] = 0; pos++ // root
	binary.BigEndian.PutUint16(q[pos:pos+2], 1)   // QTYPE A
	binary.BigEndian.PutUint16(q[pos+2:pos+4], 1) // QCLASS IN

	conn, err := net.DialTimeout("udp", c.upstream, 5*time.Second)
	if err != nil {
		return "", 0, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(q)
	if err != nil {
		return "", 0, err
	}

	resp := make([]byte, 512)
	n, err := conn.Read(resp)
	if err != nil || n < 12 {
		return "", 0, err
	}

	// Parse response - find first A record
	anCount := int(binary.BigEndian.Uint16(resp[6:8]))
	if anCount == 0 {
		return "", 0, nil
	}

	// Skip question section
	off := 12
	off = skipName(resp, off, n)
	off += 4 // QTYPE + QCLASS

	// Parse answer records
	for i := 0; i < anCount; i++ {
		if off >= n {
			break
		}
		off = skipName(resp, off, n)
		if off+10 > n {
			break
		}
		rtype := int(binary.BigEndian.Uint16(resp[off : off+2]))
		rclass := int(binary.BigEndian.Uint16(resp[off+2 : off+4]))
		ttl := binary.BigEndian.Uint32(resp[off+4 : off+8])
		rdlen := int(binary.BigEndian.Uint16(resp[off+8 : off+10]))
		off += 10
		if rtype == 1 && rclass == 1 && rdlen == 4 && off+4 <= n {
			ip := net.IP(resp[off : off+4]).String()
			return ip, ttl, nil
		}
		off += rdlen
	}

	return "", 0, nil
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

func skipName(data []byte, off, max int) int {
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
