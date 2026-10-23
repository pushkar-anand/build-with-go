package client

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// encode turns a request body into a reader and the Content-Type it is sent
// with, chosen by the body's type.
func encode[Req any](body Req) (io.Reader, string, error) {
	switch b := any(body).(type) {
	case nil:
		return nil, "", nil
	case io.Reader:
		// A *bytes.Reader or *strings.Reader can be replayed on a redirect;
		// http.NewRequest recognises them. Any other reader cannot.
		return b, "", nil
	case []byte:
		return bytes.NewReader(b), mediaTypeOctetStream, nil
	case string:
		return strings.NewReader(b), mediaTypeText, nil
	case url.Values:
		return strings.NewReader(b.Encode()), mediaTypeForm, nil
	default:
		data, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}

		return bytes.NewReader(data), mediaTypeJSON, nil
	}
}

// wantsJSON reports whether decode reads a Resp as JSON.
func wantsJSON[Resp any]() bool {
	switch any(new(Resp)).(type) {
	case *[]byte, *string, *struct{}, **http.Response:
		return false
	default:
		return true
	}
}

// decode reads resp's body into a Resp, chosen by its type. It leaves closing
// the body to the caller.
func decode[Resp any](resp *http.Response) (Resp, error) {
	var v Resp

	switch p := any(&v).(type) {
	case *[]byte:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return v, fmt.Errorf("client: read response body: %w", err)
		}

		*p = data
	case *string:
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return v, fmt.Errorf("client: read response body: %w", err)
		}

		*p = string(data)
	case *struct{}:
		// Reading to the end lets the connection be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainSize))
	default:
		// A 204, or any response declaring an empty body, has nothing to
		//  decode and leaves the zero value.
		if resp.StatusCode == http.StatusNoContent || resp.ContentLength == 0 {
			return v, nil
		}

		if err := json.UnmarshalRead(resp.Body, &v); err != nil {
			return v, fmt.Errorf("client: decode response body: %w", err)
		}
	}

	return v, nil
}
