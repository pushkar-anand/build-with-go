package transport

// Header names the transports read or redact.
const (
	headerAPIKey             = "X-Api-Key"
	headerAuthorization      = "Authorization"
	headerCookie             = "Cookie"
	headerProxyAuthorization = "Proxy-Authorization"
	headerSetCookie          = "Set-Cookie"
)

// bearerPrefix starts an Authorization header carrying a bearer token.
const bearerPrefix = "Bearer "
