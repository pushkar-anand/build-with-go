package http

import (
	"github.com/pushkar-anand/build-with-go/http/request"
	"github.com/pushkar-anand/build-with-go/http/response"
)

var _ response.Problem = (*request.ReadError)(nil)
var _ response.Problem = (*request.ValidationError)(nil)
