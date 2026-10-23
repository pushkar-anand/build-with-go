package client_test

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/pushkar-anand/build-with-go/http/client"
	"github.com/pushkar-anand/build-with-go/http/transport"
	"github.com/pushkar-anand/build-with-go/secret"
)

type post struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type newPost struct {
	Title string `json:"title"`
}

// newUpstream stands in for the API being called.
func newUpstream() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "1" {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"title":"Not Found","status":404,"detail":"no post with that id"}`)

			return
		}

		_ = json.MarshalWrite(w, post{ID: 1, Title: "Hello"})
	})

	mux.HandleFunc("POST /v1/posts", func(w http.ResponseWriter, r *http.Request) {
		var in newPost
		_ = json.UnmarshalRead(r.Body, &in)

		w.WriteHeader(http.StatusCreated)
		_ = json.MarshalWrite(w, post{ID: 2, Title: in.Title})
	})

	mux.HandleFunc("POST /v1/chat", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		for _, word := range []string{"Hello", ", ", "world"} {
			_, _ = fmt.Fprintf(w, "event: delta\ndata: {\"text\":%q}\n\n", word)
			w.(http.Flusher).Flush()
		}

		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})

	return httptest.NewServer(mux)
}

// Build a Client once per upstream and share it. Each call names the type it
// wants back; the request body's type is inferred.
func Example() {
	upstream := newUpstream()
	defer upstream.Close()

	c, err := client.New(
		client.WithBaseURL(upstream.URL+"/v1"),
		client.WithTimeout(5*time.Second),
		client.WithTokenSource(transport.StaticToken(secret.New("token"))),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	got, err := c.Get[post](ctx, "/posts/1")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(got.Title)

	created, err := c.Post[post](ctx, "/posts", newPost{Title: "Second"})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(created.ID, created.Title)

	// Output:
	// Hello
	// 2 Second
}

// A non-2xx response is an error. When the upstream sends a problem document,
// the error carries it decoded.
func Example_statusError() {
	upstream := newUpstream()
	defer upstream.Close()

	c, err := client.New(client.WithBaseURL(upstream.URL + "/v1"))
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Get[post](context.Background(), "/posts/9")

	if se, ok := errors.AsType[*client.StatusError](err); ok {
		fmt.Println(se.StatusCode, se.Problem.Detail)
	}

	// Output:
	// 404 no post with that id
}

// Asking for a string, []byte or *http.Response skips JSON decoding. The
// package-level functions use a shared default Client, for one-off calls to
// absolute URLs.
func Example_rawBody() {
	upstream := newUpstream()
	defer upstream.Close()

	body, err := client.Get[string](context.Background(), upstream.URL+"/v1/posts/1")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(body)

	// Output:
	// {"id":1,"title":"Hello"}
}

type delta struct {
	Text string `json:"text"`
}

// Stream reads a response of server-sent events, as LLM APIs stream their
// output, yielding each event as it arrives. The client's timeout does not
// cut a long stream off; bound it with the context instead.
func Example_stream() {
	upstream := newUpstream()
	defer upstream.Close()

	c, err := client.New(client.WithBaseURL(upstream.URL + "/v1"))
	if err != nil {
		log.Fatal(err)
	}

	for ev, err := range c.Stream[delta](context.Background(), http.MethodPost, "/chat", newPost{Title: "hi"}) {
		if err != nil {
			log.Fatal(err)
		}

		fmt.Print(ev.Data.Text)
	}

	fmt.Println()

	// Output:
	// Hello, world
}

// WithDebug dumps each request and response, with credentials redacted.
func Example_debug() {
	c, err := client.New(
		client.WithBaseURL("https://api.example.com"),
		client.WithDebug(os.Stderr),
	)
	if err != nil {
		log.Fatal(err)
	}

	_, _ = c.Get[struct{}](context.Background(), "/healthz")
}
