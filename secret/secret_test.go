package secret

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const raw = "hunter2"

func TestReveal(t *testing.T) {
	if got := New(raw).Reveal(); got != raw {
		t.Errorf("expected %q, got %q", raw, got)
	}

	if got := New(42).Reveal(); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}

	var zero Secret[string]
	if got := zero.Reveal(); got != "" {
		t.Errorf("expected empty string from zero value, got %q", got)
	}
}

func TestFormat(t *testing.T) {
	s := New(raw)

	type outer struct {
		Exported   Secret[string]
		unexported Secret[string]
	}
	o := outer{Exported: s, unexported: s}

	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%d", "%x", "%10s"} {
		for name, v := range map[string]any{"value": s, "pointer": &s, "nested": o, "nested pointer": &o} {
			if got := fmt.Sprintf(verb, v); strings.Contains(got, raw) || strings.Contains(got, fmt.Sprintf("%x", raw)) {
				t.Errorf("%s %s leaked the value: %s", name, verb, got)
			}
		}
	}

	if got := fmt.Sprint(s); got != Redacted {
		t.Errorf("expected %q, got %q", Redacted, got)
	}
}

func TestLog(t *testing.T) {
	s := New(raw)

	for name, h := range map[string]func(*bytes.Buffer) slog.Handler{
		"text": func(b *bytes.Buffer) slog.Handler { return slog.NewTextHandler(b, nil) },
		"json": func(b *bytes.Buffer) slog.Handler { return slog.NewJSONHandler(b, nil) },
	} {
		var buf bytes.Buffer
		slog.New(h(&buf)).Info("login", "password", s, "group", struct{ Password Secret[string] }{s})

		if got := buf.String(); strings.Contains(got, raw) || !strings.Contains(got, Redacted) {
			t.Errorf("%s handler: expected redacted output, got %s", name, got)
		}
	}
}

func TestMarshal(t *testing.T) {
	s := New(raw)

	type body struct {
		Password Secret[string] `json:"password" xml:"password"`
	}

	got, err := json.Marshal(body{Password: s})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"password":"[redacted]"}`; string(got) != want {
		t.Errorf("expected %s, got %s", want, got)
	}

	got, err = xml.Marshal(body{Password: s})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(got), raw) {
		t.Errorf("xml leaked the value: %s", got)
	}
}
