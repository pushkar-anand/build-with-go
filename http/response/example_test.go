package response_test

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/pushkar-anand/build-with-go/http/response"
)

func newWriter(opts ...response.Option) *response.JSONWriter {
	return response.NewJSONWriter(slog.New(slog.DiscardHandler), opts...)
}

// decode reads back what a handler wrote. Examples pull out named fields rather
// than comparing raw JSON, since a document's key order is not part of its
// meaning.
func decode(body *httptest.ResponseRecorder) map[string]any {
	var got map[string]any

	if err := json.UnmarshalRead(body.Body, &got); err != nil {
		log.Fatal(err)
	}

	return got
}

// Handlers report failure by returning an error, and the writer renders it.
func ExampleJSONWriter_Handle() {
	writer := newWriter()

	handler := writer.Handle(func(w http.ResponseWriter, r *http.Request) error {
		writer.Ok(w, r, map[string]any{"id": 7})

		return nil
	})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodGet, "/orders/7", nil))

	fmt.Println("status:", w.Code)
	fmt.Println("id:    ", decode(w)["id"])

	// Output:
	// status: 200
	// id:     7
}

// outOfStock describes its own response, so returning it is all a handler does.
type outOfStock struct {
	SKU string
}

func (e *outOfStock) Error() string  { return "out of stock: " + e.SKU }
func (e *outOfStock) Type() string   { return "https://example.com/probs/out-of-stock" }
func (e *outOfStock) Title() string  { return "Out of stock" }
func (e *outOfStock) Status() int    { return http.StatusConflict }
func (e *outOfStock) Detail() string { return "the item is out of stock" }

func (e *outOfStock) CustomMembers() map[string]any {
	return map[string]any{"sku": e.SKU}
}

// An error implementing Problem needs no mapper: it already says what response
// it deserves, including any extra members.
func ExampleJSONWriter_Handle_problemError() {
	writer := newWriter()

	handler := writer.Handle(func(http.ResponseWriter, *http.Request) error {
		return &outOfStock{SKU: "ABC-123"}
	})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, "/orders", nil))

	got := decode(w)

	fmt.Println("status: ", w.Code)
	fmt.Println("type:   ", got["type"])
	fmt.Println("detail: ", got["detail"])
	fmt.Println("sku:    ", got["sku"])

	// Output:
	// status:  409
	// type:    https://example.com/probs/out-of-stock
	// detail:  the item is out of stock
	// sku:     ABC-123
}

// Errors you do not own, or do not want to couple to HTTP, go through a mapper.
func ExampleWithErrorProblemMapper() {
	errNoSuchOrder := errors.New("no such order")

	writer := newWriter(response.WithErrorProblemMapper(func(err error) response.Problem {
		if errors.Is(err, errNoSuchOrder) {
			return response.NewProblem().
				WithStatus(http.StatusNotFound).
				WithDetail("no order with that id").
				Build()
		}

		return nil // anything else becomes a generic 500
	}))

	handler := writer.Handle(func(http.ResponseWriter, *http.Request) error {
		return fmt.Errorf("loading order: %w", errNoSuchOrder)
	})

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodGet, "/orders/1", nil))

	got := decode(w)

	fmt.Println("status:", w.Code)
	fmt.Println("title: ", got["title"])
	fmt.Println("detail:", got["detail"])

	// Output:
	// status: 404
	// title:  Not Found
	// detail: no order with that id
}
