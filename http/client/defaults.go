package client

import "time"

// A timeout long enough for a slow upstream, but short enough that a call
// which will never answer does not hold its caller for minutes. A request that
// needs longer can set a deadline on its context, or the client WithTimeout.
const defaultTimeout = 30 * time.Second

// How long a streamed call waits for its response to start. A stream can run
// for minutes once started, but an upstream that has not answered in this
// time is not going to.
const defaultResponseHeaderTimeout = 30 * time.Second

// The longest line of a server-sent event stream read. An LLM can put a large
// tool call in one event, but a line without end is not an event.
const maxEventLineSize = 1 << 20

// The most of an error response body kept on a StatusError. Error bodies are
// for diagnosis, and an upstream can send an arbitrarily large one.
const defaultMaxErrorBodySize = 64 << 10

// The most of an unwanted response body read to free its connection for
// reuse. A larger body is not worth the wait; its connection is dropped.
const maxDrainSize = 4 << 10
