package response

import (
	"maps"
	"net/http"
	"strings"
)

type (
	Problem interface {
		error

		Type() string
		Title() string
		Status() int
		Detail() string
		CustomMembers() map[string]any
	}

	ProblemBuilder struct {
		problemType   string
		problemTitle  string
		problemStatus int
		problemDetail string
		customMembers map[string]any
	}

	// customProblem implements the Problem interface
	customProblem struct {
		problemType   string
		problemTitle  string
		problemStatus int
		problemDetail string
		customMembers map[string]any
	}
)

var defaultProblem = NewProblem().Build()

// NewProblem starts building a Problem. It defaults to a 500 with a blank
// type, so only the parts that differ need setting.
func NewProblem() *ProblemBuilder {
	return &ProblemBuilder{
		problemType:   "about:blank",
		problemStatus: http.StatusInternalServerError,
		problemTitle:  http.StatusText(http.StatusInternalServerError),
		problemDetail: http.StatusText(http.StatusInternalServerError),
		customMembers: make(map[string]any),
	}
}

// WithType sets the problem type URI
func (pb *ProblemBuilder) WithType(typeURI string) *ProblemBuilder {
	pb.problemType = typeURI
	return pb
}

// WithTitle sets the problem title
func (pb *ProblemBuilder) WithTitle(title string) *ProblemBuilder {
	pb.problemTitle = title
	return pb
}

// WithStatus sets the HTTP status code
func (pb *ProblemBuilder) WithStatus(statusCode int) *ProblemBuilder {
	pb.problemStatus = statusCode
	// If title is still the default, update it to match the new status
	if pb.problemTitle == http.StatusText(http.StatusInternalServerError) {
		pb.problemTitle = http.StatusText(statusCode)
	}
	return pb
}

// WithDetail sets the detailed error message
func (pb *ProblemBuilder) WithDetail(detail string) *ProblemBuilder {
	pb.problemDetail = detail
	return pb
}

// WithCustomMember adds a custom property to the problem object
func (pb *ProblemBuilder) WithCustomMember(key string, value any) *ProblemBuilder {
	pb.customMembers[key] = value
	return pb
}

// Build returns the Problem described by the builder.
func (pb *ProblemBuilder) Build() Problem {
	return &customProblem{
		problemType:   pb.problemType,
		problemTitle:  pb.problemTitle,
		problemStatus: pb.problemStatus,
		problemDetail: pb.problemDetail,
		customMembers: pb.customMembers,
	}
}

// Error implements error, returning the detail so a Problem built here can be
// returned as an error from a handler and still be recognised as a Problem.
func (cp *customProblem) Error() string {
	return cp.problemDetail
}

// Type implements Problem, returning the problem type URI.
func (cp *customProblem) Type() string {
	return cp.problemType
}

// Title implements Problem, returning the short human-readable summary.
func (cp *customProblem) Title() string {
	return cp.problemTitle
}

// Status implements Problem, returning the HTTP status code to send.
func (cp *customProblem) Status() int {
	return cp.problemStatus
}

// Detail implements Problem, returning the explanation for this occurrence.
func (cp *customProblem) Detail() string {
	return cp.problemDetail
}

// CustomMembers implements Problem, returning the extra members to merge into
// the problem document.
func (cp *customProblem) CustomMembers() map[string]any {
	return cp.customMembers
}

func buildProblemJSON(r *http.Request, p Problem) map[string]any {
	m := make(map[string]any)

	m["title"] = p.Title()
	m["status"] = p.Status()
	m["detail"] = p.Detail()
	m["instance"] = r.RequestURI

	if t := p.Type(); t == "" || strings.EqualFold(t, "about:blank") {
		// RFC 9457: when type is about:blank the title should be the status text.
		m["type"] = "about:blank"
		m["title"] = http.StatusText(p.Status())
	} else {
		m["type"] = t
	}

	maps.Copy(m, p.CustomMembers())

	return m
}

var _ Problem = (*customProblem)(nil)
