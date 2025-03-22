package response

import (
	"net/http"
	"strings"
)

type (
	Problem interface {
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

func (pb *ProblemBuilder) Build() Problem {
	return &customProblem{
		problemType:   pb.problemType,
		problemTitle:  pb.problemTitle,
		problemStatus: pb.problemStatus,
		problemDetail: pb.problemDetail,
		customMembers: pb.customMembers,
	}
}

func (cp *customProblem) Type() string {
	return cp.problemType
}

func (cp *customProblem) Title() string {
	return cp.problemTitle
}

func (cp *customProblem) Status() int {
	return cp.problemStatus
}

func (cp *customProblem) Detail() string {
	return cp.problemDetail
}

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
		m["type"] = "about:blank"
		m["title"] = http.StatusText(p.Status())
	}

	for k, v := range p.CustomMembers() {
		m[k] = v
	}

	return m
}

var _ Problem = (*customProblem)(nil)
