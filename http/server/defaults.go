package server

import (
	"time"
)

const (
	defaultReadTimeout  = 1 * time.Second
	defaultWriteTimeout = 1 * time.Second
	defaultIdleTimout   = 1 * time.Second
)

const (
	defaultHost = "0.0.0.0"
	defaultPort = 8080
)
