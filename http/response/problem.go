package response

import (
	"net/http"
	"strings"
)

type Problem interface {
	Type() string
	Title() string
	Status() int
	Detail() string
	CustomMembers() map[string]any
}

type defaultProblem struct{}

func (d *defaultProblem) Type() string {
	return "about:blank"
}

func (d *defaultProblem) Title() string {
	return http.StatusText(http.StatusInternalServerError)
}

func (d *defaultProblem) Status() int {
	return http.StatusInternalServerError
}

func (d *defaultProblem) Detail() string {
	return http.StatusText(http.StatusInternalServerError)
}

func (d *defaultProblem) CustomMembers() map[string]any {
	return nil
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
