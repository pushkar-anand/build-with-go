package http

import (
	"github.com/pushkar-anand/build-with-go/http/request"
	"github.com/pushkar-anand/build-with-go/http/response"
	"github.com/pushkar-anand/build-with-go/validator"
)

var _ response.Problem = (*request.ReadError)(nil)
var _ response.Problem = (*request.ValidationError)(nil)

// request no longer imports validator; this keeps the two in step.
var _ request.Validator = (*validator.Validator)(nil)
