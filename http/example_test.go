package http_test

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/pushkar-anand/build-with-go/http/middleware"
	"github.com/pushkar-anand/build-with-go/http/request"
	"github.com/pushkar-anand/build-with-go/http/response"
	"github.com/pushkar-anand/build-with-go/http/server"
	"github.com/pushkar-anand/build-with-go/logger"
	"github.com/pushkar-anand/build-with-go/validator"
)

type createPost struct {
	Title string `json:"title" validate:"required,min=3"`
	Body  string `json:"body"  validate:"required"`
}

// A whole service: a logger that carries the request ID, a reader that decodes
// and validates, a writer that renders errors as problem documents, and a
// server whose lifetime follows a context.
func Example() {
	log := logger.New(logger.WithWriter(io.Discard))

	v, err := validator.New()
	if err != nil {
		log.Error("building validator", logger.Error(err))
		return
	}

	reader := request.NewReader(log, v)
	writer := response.NewJSONWriter(log)

	mux := http.NewServeMux()

	// Handlers return errors; the writer turns them into responses.
	mux.Handle("POST /posts", writer.Handle(func(w http.ResponseWriter, r *http.Request) error {
		post, err := reader.ReadAndValidateJSON[createPost](r)
		if err != nil {
			return err
		}

		writer.Write(w, r, http.StatusCreated, map[string]any{"title": post.Title})

		return nil
	}))

	// Middleware is the standard func(http.Handler) http.Handler shape.
	handler := middleware.RequestID(logger.NewHTTPLogger(log)(mux))

	srv := server.New(handler, server.WithHostPort("127.0.0.1", 0), server.WithLogger(log))
	if err := srv.Listen(); err != nil {
		log.Error("listening", logger.Error(err))
		return
	}

	ctx, stop := context.WithCancel(context.Background())

	stopped := make(chan error, 1)
	go func() { stopped <- srv.Serve(ctx) }()

	fmt.Println("valid:  ", post(srv.Addr(), `{"title":"Hello","body":"World"}`))
	fmt.Println("invalid:", post(srv.Addr(), `{"title":"no"}`))

	stop()

	if err := <-stopped; err != nil {
		log.Error("serving", logger.Error(err))
	}

	// Output:
	// valid:   201 Hello
	// invalid: 422 Request is not valid
}

func post(addr, body string) string {
	resp, err := http.Post("http://"+addr+"/posts", "application/json", strings.NewReader(body))
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = resp.Body.Close() }()

	var got map[string]any
	if err := json.UnmarshalRead(resp.Body, &got); err != nil {
		log.Fatal(err)
	}

	// A success carries the created title; a failure carries the problem detail.
	field := got["title"]
	if detail, ok := got["detail"]; ok {
		field = detail
	}

	return fmt.Sprintf("%d %v", resp.StatusCode, field)
}
