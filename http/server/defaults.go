package server

import (
	"time"
)

// Timeouts generous enough for a handler that talks to a database, but tight
// enough to shed connections that are stuck or slow-reading. Endpoints that
// stream, such as SSE, should disable the write timeout with WithWriteTimeout(0).
const (
	defaultReadTimeout       = 15 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second

	defaultShutdownTimeout = 5 * time.Second
)

const (
	defaultHost = "0.0.0.0"
	defaultPort = 8080
)
