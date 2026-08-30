// Package http holds the contracts binding the http subpackages together.
//
// It declares no API of its own. The assertions here fail at compile time if
// request's error types stop satisfying response.Problem, or if the validator
// stops satisfying the interface request expects — relationships the packages
// depend on but, by design, cannot import each other to state.
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
