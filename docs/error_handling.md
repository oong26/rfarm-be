# Error Handling

**English** · [Bahasa Indonesia](error_handling.id.md)

[← Back to Documentation](README.md)

---

## Principles

1. **An error is part of the API contract.** It carries a status, a stable code and a
   client-safe message.
2. **Create errors where the knowledge is.** The store knows the row is missing, the
   service knows the state is wrong, the handler knows the field is invalid.
3. **Log or return — never both.** A returned error is logged once, at the boundary.
4. **Never leak internals.** No SQL, no driver errors, no stack traces, no other
   farm's IDs in a response body.
5. **Wrap with context, not with repetition.** `%w` keeps the chain; the message adds
   what is missing.
6. **Codes are English and stable; messages are Indonesian.** The frontend branches
   on the code, the user reads the message.

---

## Taxonomy

| Kind | Created by | Status | `code` |
|---|---|---|---|
| Unparseable request | Handler (parse) | 400 | `bad_request` |
| Malformed query | Handler / service | 400 | `invalid_query` |
| Missing or invalid token | Middleware | 401 | `unauthorized` |
| Refresh token reused, expired, or revoked | Service / `SessionStore` | 401 | `unauthorized` |
| Role not allowed | Middleware | 403 | `forbidden` |
| Row missing, soft-deleted, other farm | Store | 404 | `not_found` |
| Duplicate unique value | Store | 409 | `conflict` |
| Delete blocked by FK | Store | 409 | `conflict` |
| Invalid state transition | Service | 409 | `conflict` |
| `If-Match` version mismatch | Service or store | 412 | `version_conflict` |
| `If-Match` missing | Handler | 428 | `precondition_required` |
| Field / cross-field validation | Handler or service | 422 | `validation_failed` |
| Endpoint removed / one-time link used | Handler | 410 | `gone` |
| Rate limit | Middleware | 429 | `too_many_requests` |
| Unexpected failure | Any | 500 | `internal` |
| Dependency unavailable | Client, `SessionStore` | 503 | `service_unavailable` |
| Dependency timeout | Client, `SessionStore` | 504 | `gateway_timeout` |

Mapping detail: [HTTP Status](standards/http_status.md).

---

## The Error Type

`pkg/response/app_error.go`.

```go
type AppError struct {
    Status  int     // HTTP status
    Code    string  // stable snake_case code, safe for client logic
    Title   string  // short English summary, shown in problem.title
    Message string  // Indonesian, user-facing, no internals
    Fields  []Field // optional field errors
    cause   error   // internal only — never serialized
}

type Field struct {
    Field   string `json:"field"`
    Code    string `json:"code"`
    Message string `json:"message"`
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }
func (e *AppError) Unwrap() error { return e.cause }
```

`pkg/response/problem.go` holds the wire form. `Message` becomes `detail`, so an
error without field details still carries an Indonesian sentence:

```go
type Problem struct {
    Type      string  `json:"type"`
    Title     string  `json:"title"`
    Status    int     `json:"status"`
    Code      string  `json:"code"`
    Detail    string  `json:"detail,omitempty"`
    RequestID string  `json:"requestId"`
    Errors    []Field `json:"errors,omitempty"`
}
```

```go
func BadRequest(msg string) *AppError
func InvalidQuery(msg string) *AppError
func Unauthorized(msg string) *AppError
func Forbidden(msg string) *AppError
func NotFound(resource string) *AppError            // → "Data <resource> tidak ditemukan."
func Conflict(msg string) *AppError
func Gone(msg string) *AppError
func PreconditionFailed(msg string) *AppError        // 412, code version_conflict
func PreconditionRequired() *AppError                // 428, missing If-Match
func UnsupportedMedia(msg string) *AppError
func Unprocessable(fields ...Field) *AppError        // 422, code validation_failed
func TooManyRequests(msg string) *AppError
func Internal(cause error) *AppError                 // message is always generic
func ServiceUnavailable(msg string) *AppError
func GatewayTimeout(msg string) *AppError
```

Rules:

- Constructors return `*AppError`; services and stores return it as `error`.
- `Internal` keeps the cause for logging and never exposes it.
- `Code` never changes once clients depend on it. Adding a code is fine; renaming one
  is a breaking change.
- `Fields` is structured and bounded, never an `error`, never a map of raw values.
- `Message` is written in Indonesian; `Title` and `code` stay in English.

---

## Where Errors Are Created

| Layer | Creates |
|---|---|
| **Handler** | `BadRequest`, `InvalidQuery`, `PreconditionRequired`, `UnsupportedMedia`, field validation through `utils.ParseAndValidate` |
| **Service** | `Forbidden`, `Conflict`, `Unprocessable` (cross-field rules), `PreconditionFailed`, `Internal` for an invariant that should never happen |
| **Store** | `NotFound` from `GetByID`, `Conflict` from unique/FK violations, `PreconditionFailed` from an atomic CTE status |
| **Session store** | `Unauthorized` from an expired/reused refresh token; `ServiceUnavailable` when Redis cannot be reached |
| **Client** | `ServiceUnavailable`, `GatewayTimeout`, wrapped transport errors |
| **Middleware** | `Unauthorized`, `Forbidden`, `TooManyRequests` |

A handler never inspects the error to choose a code. It calls `response.Fail` and
moves on.

---

## Wrapping

```go
// store — add operation context, keep the chain
if err := r.pool.QueryRow(ctx, q, id, farmID).Scan(&s); err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        return nil, response.NotFound("penjualan")
    }
    return nil, fmt.Errorf("stores.GetSaleByID: %w", err)
}
```

- Use `%w`, never `%v`, when the caller may need `errors.Is`/`errors.As`.
- Prefix with `package.Method` — it is the first thing in a log line.
- Messages are lowercase, no trailing period, no `error:` prefix.
- Do not wrap an `*AppError` in another `*AppError`; return it as is.

```go
// ✗ double mapping — hides a 404 or 412 as 500
if err != nil {
    return response.Internal(err)
}

// ✓ pass through, wrap only unknown errors
if err != nil {
    return err
}
```

---

## Handler Boundary

```go
func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request) {
    var req dtos.UpdateSaleRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)
        return
    }

    version, err := utils.IfMatchVersion(r)
    if err != nil {
        response.Fail(w, r, err)   // 428 when the header is missing
        return
    }

    err = h.svc.UpdateSale(r.Context(), dtos.UpdateSaleInput{ /* … Version: version */ })
    if err != nil {
        response.Fail(w, r, err)
        return
    }

    response.NoContent(w)
}
```

```go
// pkg/response/fail.go
func Fail(w http.ResponseWriter, r *http.Request, err error) {
    var appErr *AppError
    if !errors.As(err, &appErr) {
        appErr = Internal(err)          // unknown → 500, cause kept for logs
    }

    if appErr.Status >= http.StatusInternalServerError {
        LoggerFrom(r.Context()).Error("request failed",
            "request_id", RequestIDFrom(r.Context()),
            "method", r.Method,
            "path", r.URL.Path,
            "err", appErr.Unwrap(),
        )
    }

    writeProblem(w, r, appErr)
}
```

`writeProblem` is the only function that serialises an error. It sets
`Content-Type: application/problem+json` and writes the RFC 9457 body.

---

## The Problem Body — RFC 9457

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/problem+json
X-Request-ID: 01J8Z3Q7…
```

```json
{
  "type": "https://rfarm.example/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "code": "validation_failed",
  "requestId": "01J8Z3Q7…",
  "errors": [
    { "field": "rejects", "code": "exceeds_eggs",
      "message": "Telur afkir tidak boleh melebihi telur terkumpul." },
    { "field": "date", "code": "required",
      "message": "Tanggal wajib diisi." }
  ]
}
```

| Field | Rule |
|---|---|
| `type` | `https://rfarm.example/problems/<code>` — stable per code |
| `title` | one short English sentence per code |
| `status` | always equals the HTTP status |
| `code` | the stable `snake_case` code |
| `detail` | the Indonesian `Message`; omitted when `errors[]` carries the text |
| `requestId` | echoed from the request ID; quoted in bug reports |
| `errors[]` | present only for validation-style errors |

---

## Field and Cross-Field Validation (422)

Field errors use the stable code list from the specification:

`required`, `min`, `max`, `invalid_enum`, `duplicate_date`, `exceeds_eggs`,
`exceeds_feed_given`, `journal_unbalanced`, `same_account`, `invalid_account`,
`next_before_date`, `version_conflict`.

```go
// service — a rule that needs other rows
if in.Rejects.GreaterThan(in.Eggs) {
    return response.Unprocessable(response.Field{
        Field:   "rejects",
        Code:    "exceeds_eggs",
        Message: "Telur afkir tidak boleh melebihi telur terkumpul.",
    })
}
```

```go
// two fields that cannot be validated alone
if in.DebitAccountID == in.CreditAccountID {
    return response.Unprocessable(response.Field{
        Field:   "creditAccountId",
        Code:    "same_account",
        Message: "Akun debit dan kredit tidak boleh sama.",
    })
}
```

- `utils.ParseAndValidate` converts `validator.ValidationErrors` into `[]Field`,
  mapping `required`/`min`/`max`/`oneof` to the stable codes.
- `field` is the JSON key the client sent (`pageSize`, not `page_size`).
- Never return `validator.ValidationErrors` or a Go error string.
- A body that cannot be parsed, or a wrong content type, is `400` — not `422`.

---

## Database Errors

Map driver errors once, in a shared store helper.

```go
func mapDBError(err error, resource string) error {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": // unique_violation
            return response.Conflict("Data " + resource + " dengan nilai tersebut sudah ada.")
        case "23503": // foreign_key_violation
            return response.Conflict("Data " + resource + " masih dipakai oleh data lain.")
        case "23502": // not_null_violation
            return response.Internal(err) // a bug in our code, not the client's
        case "22P02": // invalid_text_representation
            return response.BadRequest("Format ID tidak valid.")
        case "40001": // serialization_failure
            return response.Conflict("Data sedang diubah. Silakan coba lagi.")
        case "57014": // query_canceled (statement_timeout)
            return response.GatewayTimeout("Permintaan melebihi batas waktu.")
        }
    }
    return fmt.Errorf("stores: %w", err)
}
```

| PostgreSQL | Meaning | Response |
|---|---|---|
| `23505` | unique violation | `409 Conflict` |
| `23503` | FK violation | `409 Conflict` |
| `23502` | NOT NULL violation | `500 Internal` (our bug) |
| `22P02` | invalid input syntax (e.g. uuid) | `400 Bad Request` |
| `40001` | serialization failure | retry internally, else `409` |
| `57014` | statement timeout | `504 Gateway Timeout` |
| `pgx.ErrNoRows` | no row | `404 Not Found` (from `GetByID` only) |

A plain `UPDATE`/`DELETE` returning zero rows is **not** an error at the store level —
return the count and let the service decide whether it is a `404` or a `412`
([Service & Store](standards/service_repo.md)).

---

## Redis and Session Errors

Refresh sessions live in Redis, so the auth endpoints have a second dependency.

| Redis result | Meaning | Response |
|---|---|---|
| key found, hash matches | live session | continue |
| `redis.Nil` on lookup | no session: expired, logged out, or never existed | `401 Unauthorized` |
| token hash unknown but family alive | the token was already rotated — assume theft | revoke family, `401 Unauthorized` |
| `context.DeadlineExceeded` | Redis slower than the configured timeout | `504 Gateway Timeout` |
| dial/read/write error, `WRONGPASS`, `NOAUTH` | Redis unreachable or misconfigured | `503 Service Unavailable` |

```go
// stores — transport failure stays a transport failure
if err := r.rdb.Get(ctx, sessionKey(hash)).Scan(&s); err != nil {
    if errors.Is(err, redis.Nil) {
        return nil, response.Unauthorized("Sesi sudah berakhir. Silakan masuk kembali.")
    }
    return nil, fmt.Errorf("stores.GetSession: %w", err) // mapped to 503 at the boundary
}
```

- `redis.Nil` is a **cache miss**, not an error — never wrap it as `Internal`.
- Redis being down is not a cache degradation: login, refresh, logout, and `/readyz`
  fail with `503`. There is no in-memory fallback.
- Never put the raw token in an error, a log line, or a problem body.
- A reused refresh token is a security event: log it at `warn` with `user_id` and the
  family ID, and revoke the family before returning `401`.

---

## 409 vs 412

Both mean "the request is valid, but the data says no". The distinction is what the
client must do next.

| | 409 Conflict | 412 Precondition Failed |
|---|---|---|
| Meaning | a uniqueness or state rule is violated | the row changed since the client read it |
| Cause | duplicate date, FK still referenced, invalid transition | `If-Match` version ≠ current `version` |
| Client action | change the payload | re-read, merge, retry with the new version |
| Code | `conflict` | `version_conflict` |

```go
// 409 — the rule is about the data, not about concurrency
return response.Conflict("Catatan harian untuk tanggal tersebut sudah ada.")

// 412 — the client's snapshot is stale
return response.PreconditionFailed("Data sudah diubah pengguna lain. Muat ulang lalu coba lagi.")
```

Never turn a version mismatch into a `409`: the frontend shows a different message and
takes a different action.

---

## Logging

| Level | Use for |
|---|---|
| `debug` | values useful while developing; never personal data |
| `info` | startup, shutdown, background job finished |
| `warn` | recovered oddity, retry, deprecated endpoint used |
| `error` | 5xx, failed background job, data inconsistency |

Rules:

- **Log once.** The handler returns; `response.Fail` (or the middleware) logs. A
  service must not log an error it also returns.
- Every log line includes `request_id`, `user_id` and `farm_id` when known.
- Log the cause (`appErr.Unwrap()`), never `appErr.Message` alone.
- 4xx is not an error-level log — it is a client mistake, visible in the access log.
- Never log tokens, passwords, full bodies, or personal data.

---

## Panics

`middleware.Recover` sits above every route.

```go
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    log.Error("panic recovered",
                        "request_id", RequestIDFrom(r.Context()),
                        "panic", rec,
                        "stack", string(debug.Stack()),
                    )
                    response.Fail(w, r, response.Internal(fmt.Errorf("panic: %v", rec)))
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}
```

- Panic only for programmer errors (nil invariant, impossible state), never for an
  expected business outcome.
- The client always gets the standard 500 problem body.
- The stack goes to the log, never to the response.
- The recover middleware must be registered before anything that can panic, including
  the access log.

---

## Async and Background Errors

- A goroutine must recover its own panics; a crash there kills the process.
- Attach the originating `request_id` (or a new `job_id`) to every log line.
- Never swallow an error with `_ =`; handle it or log it with context.
- A background job that fails must leave a state the operator can see (status column,
  retry counter), not only a log line.

```go
go func() {
    defer func() {
        if rec := recover(); rec != nil {
            log.Error("export job panicked", "panic", rec, "job_id", jobID)
        }
    }()
    if err := exporter.Run(ctx, jobID); err != nil {
        log.Error("export job failed", "job_id", jobID, "err", err)
    }
}()
```

---

## Anti-Patterns

| Anti-pattern | Why it hurts | Do instead |
|---|---|---|
| `if err != nil { return err }` for an unknown error, straight to the client | Leaks SQL/driver detail | Wrap, let the boundary map it |
| `response.Internal(err)` around a typed error | Turns 404/409/412 into 500 | Return the typed error |
| Comparing error strings | Breaks on wording changes | `errors.Is` / typed error |
| Logging in both service and handler | Duplicate noise, wrong level | Log once at the boundary |
| Building a problem body by hand | Drifts from the contract | `response.Fail` |
| Returning a version mismatch as `409` | Wrong client behaviour | `412` + `version_conflict` |
| English user-facing messages | The app is for Indonesian users | Indonesian `message`, English `code` |
| `panic` for validation or state | No contract, kills the request | Typed error |
| A new code per message | Unstable contract | One code per class |
| Wrapping `redis.Nil` as `Internal` | Turns a logged-out user into a `500` | `Unauthorized` |
| Falling back to memory when Redis is down | Two sources of truth, silent session loss | Fail with `503` |

---

## Quick Reference

| Question | Answer |
|---|---|
| Where do I create a `NotFound`? | Store, when a row is missing |
| Where do I create a `Conflict`? | Store for uniqueness/FK, service for state |
| Where do I create a `412`? | Wherever `version` is compared |
| Where do I create a `422`? | Handler (tags) or service (cross-field) |
| Where do I create a `401` for an expired session? | `SessionStore` / auth service, on `redis.Nil` or reuse |
| Where do I create a `503`? | Wherever a Redis transport error surfaces, via `ServiceUnavailable` |
| Where is the error mapped to JSON? | `response.Fail` |
| Where is the error logged? | Once, at the boundary, for 5xx |
| How do I wrap? | `fmt.Errorf("stores.X: %w", err)` |
| What does the client see? | `type`, `title`, `status`, `code`, `detail`, `requestId`, `errors[]` |
| What does the client never see? | SQL, driver text, stack traces, causes |
