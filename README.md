# build-with-go

A collection of small libraries I reach for when writing Go services: an HTTP
server with a lifecycle you can cancel, request decoding and validation,
RFC 9457 error responses, structured logging that carries a request ID, and
layered configuration.

Everything is `net/http` and `log/slog`. There is no framework here, and nothing
asks you to structure your service a particular way.

```
go get github.com/pushkar-anand/build-with-go
```

Requires Go 1.27.

## A whole service

```go
log := logger.New()

v, err := validator.New()
if err != nil {
    return err
}

reader := request.NewReader(log, v)
writer := response.NewJSONWriter(log)

mux := http.NewServeMux()

mux.Handle("POST /posts", writer.Handle(func(w http.ResponseWriter, r *http.Request) error {
    post, err := reader.ReadAndValidateJSON[CreatePost](r)
    if err != nil {
        return err // a 400 or a 422, rendered as a problem document
    }

    created, err := store.Create(r.Context(), post)
    if err != nil {
        return err // your mapper decides; unmapped errors become a 500
    }

    writer.Write(w, r, http.StatusCreated, created)

    return nil
}))

handler := middleware.RequestID(logger.NewHTTPLogger(log)(mux))

srv := server.New(handler, server.WithHostPort("0.0.0.0", 8080), server.WithLogger(log))

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

return srv.Serve(ctx)
```

That example is real, and runs as part of the test suite — see `Example` in
[`http/example_test.go`](http/example_test.go). Every package has runnable
examples with verified output; `go doc` is the reference.

## The ideas

**Handlers return errors.** `writer.Handle` adapts a
`func(http.ResponseWriter, *http.Request) error` into an `http.Handler`, so a
handler says what went wrong and nothing more. There is one place where errors
become responses.

**Errors describe their own response.** Anything implementing
`response.Problem` — `Type`, `Title`, `Status`, `Detail`, `CustomMembers` —
renders itself as an [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem
document. Read and validation failures already do. For errors you do not own,
`WithErrorProblemMapper` maps them, and anything unmapped becomes a generic 500
rather than leaking an internal message.

**Request IDs travel by themselves.** `middleware.RequestID` honours an inbound
`X-Request-Id` or mints a UUIDv7, puts it in the context, and echoes it back.
Loggers from `logger.New` read it from there, so every line logged during a
request is attributable without handlers passing anything around.

**Shutdown actually drains.** Cancelling the context passed to `Serve` stops the
server accepting connections and waits for in-flight requests. `Serve` returns
only once that has finished, so exiting when it returns is safe.

## Packages

| Package | What it does |
| --- | --- |
| [`config`](config) | YAML config with environment overrides for secrets |
| [`logger`](logger) | slog loggers, and HTTP request logging middleware |
| [`validator`](validator) | struct-tag validation with per-field reasons |
| [`ctxval`](ctxval) | request-scoped context values |
| [`http/server`](http/server) | `http.Server` with context-driven graceful shutdown |
| [`http/middleware`](http/middleware) | request ID middleware |
| [`http/request`](http/request) | decode and validate JSON, form, and query data |
| [`http/response`](http/response) | JSON responses and RFC 9457 problem documents |

### config

The file holds what is safe to commit; the environment holds what is not.
Sources layer, each overriding the last: defaults, then YAML, then the
environment.

```go
cfg, err := config.Load[Config](
    config.WithDefaults(map[string]any{"server.port": 8080}),
    config.WithYAML("config.yaml"),
    config.WithEnvPrefix("APP_"),
)
```

A **double underscore** separates nesting, so a single underscore stays part of
a key name:

```
APP_DATABASE__PASSWORD      ->  database.password
APP_SERVER__READ_TIMEOUT    ->  server.read_timeout
```

### http/request

One `Reader` serves every request type; the type parameter goes at the call
site.

```go
post, err := reader.ReadAndValidateJSON[CreatePost](r)
params, err := reader.ReadAndValidateQueryParams[ListPosts](r)
```

Struct tags cover most rules. For anything spanning several fields, implement
`Valid(ctx) map[string]string` and the type validates itself — no validator
needed:

```go
func (d *DateRange) Valid(context.Context) map[string]string {
    problems := make(map[string]string)

    if d.End.Before(d.Start) {
        problems["end"] = "end must not be before start"
    }

    return problems
}
```

### http/response

```go
writer.Ok(w, r, v)                      // 200
writer.Write(w, r, http.StatusCreated, v)
writer.WriteError(w, r, err)            // resolves err to a Problem
writer.WriteProblem(w, r, problem)
```

Build a one-off problem with `response.NewProblem()`, or give a domain error the
`Problem` methods and return it directly.

### http/server

Timeouts default to read 15s, read-header 5s, write 30s, idle 120s. Streaming
endpoints need `WithWriteTimeout(0)`, since a write timeout caps the lifetime of
a response.

`Listen()` binds without serving, so `Addr()` reports the assigned port — bind
to port 0 in tests and skip the race of reserving one.

## Status

Personal libraries, used in my own projects. The API is not stable; breaking
changes land when something is worth fixing properly.
