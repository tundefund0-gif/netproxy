package proxy

import "sync/atomic"

var (
	ConnTotal  atomic.Int64
	ConnActive atomic.Int64
	BytesSent  atomic.Int64
	BytesRecv  atomic.Int64
	Verbose    atomic.Bool
)
