package client

import (
	json "encoding/json/v2"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
)

type (
	// StatusError is returned for a response outside the 2xx range.
	//
	// It deliberately does not implement response.Problem. Returned from a
	// handler unmapped, it becomes a 500: an upstream's 401 or 404 says
	// nothing about the request your service received.
	StatusError struct {
		// Method and URL identify the request. The URL has its query and any
		// credentials removed, since the error is likely to be logged.
		Method string
		URL    string

		StatusCode int
		Header     http.Header

		// Body holds the start of the response body, up to the client's
		// WithMaxErrorBodySize.
		Body []byte

		// Problem is the decoded body when the upstream sent an RFC 9457
		// problem document, and nil otherwise.
		Problem *Problem
	}

	// Problem is an RFC 9457 problem document sent by an upstream.
	Problem struct {
		Type     string `json:"type,omitzero"`
		Title    string `json:"title,omitzero"`
		Status   int    `json:"status,omitzero"`
		Detail   string `json:"detail,omitzero"`
		Instance string `json:"instance,omitzero"`

		// Extensions holds every member not named above.
		Extensions map[string]any `json:",embed"`
	}
)

// Error implements error, adding the problem's detail when there is one.
func (e *StatusError) Error() string {
	msg := fmt.Sprintf("client: %s %s: %d %s", e.Method, e.URL, e.StatusCode, http.StatusText(e.StatusCode))

	if e.Problem != nil && e.Problem.Detail != "" {
		msg += ": " + e.Problem.Detail
	}

	return msg
}

// newStatusError reads what it keeps of resp's body. It leaves closing the
// body to the caller.
func newStatusError(req *http.Request, resp *http.Response, maxBody int64) *StatusError {
	// A failed read leaves a partial body. The status is what matters, so the
	// error is returned with whatever was read.
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))

	e := &StatusError{
		Method:     req.Method,
		URL:        displayURL(req.URL),
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       data,
	}

	if mediaType, _, err := mime.ParseMediaType(resp.Header.Get(headerContentType)); err == nil && mediaType == mediaTypeProblemJSON {
		p := new(Problem)
		if json.Unmarshal(data, p) == nil {
			e.Problem = p
		}
	}

	return e
}

// displayURL returns u without its query, fragment or credentials.
func displayURL(u *url.URL) string {
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path, RawPath: u.RawPath}).String()
}
