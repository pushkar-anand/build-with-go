package client

// Header names the client sets.
const (
	headerAccept        = "Accept"
	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"
	headerRequestID     = "X-Request-Id"
	headerUserAgent     = "User-Agent"
)

// Media types the client sends and recognises.
const (
	mediaTypeEventStream = "text/event-stream"
	mediaTypeForm        = "application/x-www-form-urlencoded"
	mediaTypeJSON        = "application/json"
	mediaTypeOctetStream = "application/octet-stream"
	mediaTypeProblemJSON = "application/problem+json"
	mediaTypeText        = "text/plain; charset=utf-8"
)

// bearerPrefix starts an Authorization header carrying a bearer token.
const bearerPrefix = "Bearer "
