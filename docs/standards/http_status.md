# HTTP Status Code Standards

**English** · [Bahasa Indonesia](http_status.id.md)

[← Back to Standards](standard.md)

---

## Principle

Return the most specific code that describes the outcome. `400` is not a catch-all:
the status code is part of the API contract and tells the client *why* the request
failed without parsing the message.

Codes and messages have different audiences: **codes are stable English**, messages
are Indonesian and may be reworded for the user.

---

## Code Reference

### 2xx — Success

| Code | Name | When to use |
|---|---|---|
| `200` | OK | Read or update succeeded and a body is returned. |
| `201` | Created | A resource was created; return it (or at least its ID) in the body. |
| `204` | No Content | Success with no body: update, delete, state change. |

### 4xx — Client Error

| Code | Name | When to use |
|---|---|---|
| `400` | Bad Request | Structurally invalid: unparseable JSON, wrong type, malformed query, an impossible value that validation cannot express. |
| `401` | Unauthorized | No valid access token: missing, expired, malformed. |
| `403` | Forbidden | Valid token, but the role (`viewer` / `editor` / `owner`) does not allow the action. |
| `404` | Not Found | The resource does not exist, is soft-deleted, or belongs to another farm. |
| `409` | Conflict | The request is valid but violates a **uniqueness** rule that exists in the data: `daily_records` date already taken, journal reference already used. |
| `410` | Gone | An endpoint was permanently removed, or a one-time download was already consumed. |
| `412` | Precondition Failed | `If-Match` version does not match the row — someone else saved first. |
| `415` | Unsupported Media Type | Body is not `application/json`. |
| `422` | Unprocessable Entity | Parsed, but field validation failed: required, min/max, enum, cross-field rule (`rejects > eggs`). |
| `429` | Too Many Requests | Rate limit exceeded. |

### 5xx — Server Error

| Code | Name | When to use |
|---|---|---|
| `500` | Internal Server Error | An unexpected failure: a bug, a nil dereference, an unhandled case. Never for a predictable business rule. |
| `503` | Service Unavailable | A dependency (database, external API) is unreachable or saturated — retry later. |
| `504` | Gateway Timeout | A downstream call was accepted but did not finish within our timeout. |

---

## 409 vs 412 vs 422 — Decision Guide

```
Is the body unparseable or structurally wrong?
  └─ yes → 400 Bad Request

Did it parse but fail field or cross-field validation?
  └─ yes → 422 Unprocessable Entity

Is the request valid and the data would accept it, except for the row's version?
  └─ yes → 412 Precondition Failed

Is the request valid but a uniqueness rule already holds?
  ├─ daily record for that date exists        → 409 Conflict
  ├─ journal reference already used           → 409 Conflict
  └─ deleting a row still referenced          → 409 Conflict
```

**Rule of thumb:** if the same request would succeed after someone else's data
changes (they save, the dependent row is removed), it is `409` or `412`. If no state
change could ever make it valid, it is `400` or `422`.

---

## Success Response Shape

```json
// 200 — single resource
{ "data": { "id": "0192…", "date": "2026-10-31", "qty": 120, "amount": 264000 } }

// 200 — list
{
  "data": [ { "id": "0192…" } ],
  "meta": { "page": 1, "pageSize": 25, "total": 137 }
}

// 201 — created
{ "data": { "id": "0192…", "version": 1 } }
```

- Never return a bare array at the top level.
- Never return `null` for a list; return an empty array.
- `meta` is mandatory on list endpoints — see [List API](list_api.md).
- Updates that return the record also set `ETag: "<version>"`.

---

## Error Response Shape — RFC 9457

Content type: `application/problem+json`.

```json
{
  "type": "https://rfarm.example/problems/validation",
  "title": "Validation failed",
  "status": 422,
  "code": "validation_failed",
  "requestId": "01J8Z3…",
  "errors": [
    { "field": "rejects", "code": "exceeds_eggs", "message": "Telur afkir tidak boleh melebihi telur terkumpul." }
  ]
}
```

| Field | Rule |
|---|---|
| `type` | `https://rfarm.example/problems/<code>` — stable, documented. |
| `title` | Short English summary, one per code. |
| `status` | Must equal the HTTP status. |
| `code` | Stable `snake_case` English code the frontend may branch on. |
| `detail` | Indonesian message for errors without `errors[]`; omitted when field errors carry the messages. |
| `requestId` | From `X-Request-ID`; quote it in bug reports. |
| `errors[]` | Optional field errors: `field`, `code`, `message`. `field` matches the JSON key. |

`errors[].code` uses the spec's stable list: `required`, `min`, `max`,
`invalid_enum`, `duplicate_date`, `exceeds_eggs`, `exceeds_feed_given`,
`journal_unbalanced`, `same_account`, `invalid_account`, `next_before_date`,
`version_conflict`.

Codes are English; messages are Indonesian for the user. Never put SQL, a driver
error, a stack trace, or another farm's IDs in a body.

---

## `pkg/response` Mapping

| Constructor | Status | `code` |
|---|---|---|
| `response.OK(w, data)` | 200 | — |
| `response.Created(w, data)` | 201 | — |
| `response.NoContent(w)` | 204 | — |
| `response.List(w, data, meta)` | 200 | — |
| `response.BadRequest(msg)` | 400 | `bad_request` |
| `response.Unauthorized(msg)` | 401 | `unauthorized` |
| `response.Forbidden(msg)` | 403 | `forbidden` |
| `response.NotFound(resource)` | 404 | `not_found` |
| `response.Conflict(msg)` | 409 | `conflict` |
| `response.Gone(msg)` | 410 | `gone` |
| `response.PreconditionFailed(msg)` | 412 | `version_conflict` |
| `response.UnsupportedMedia(msg)` | 415 | `unsupported_media_type` |
| `response.Unprocessable(msg, fields)` | 422 | `validation_failed` |
| `response.TooManyRequests(msg)` | 429 | `too_many_requests` |
| `response.Internal(err)` | 500 | `internal` |
| `response.ServiceUnavailable(msg)` | 503 | `service_unavailable` |
| `response.GatewayTimeout(msg)` | 504 | `gateway_timeout` |
| `response.Fail(w, r, err)` | from the error | from the error |

`response.Fail` is the single conversion point. Handlers never build a problem body
by hand. Details: [Error Handling](../error_handling.md).

---

## Handler Pattern

```go
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req dtos.CreateSaleRequest
    if err := utils.ParseAndValidate(r, &req); err != nil {
        response.Fail(w, r, err)   // 400 (unparseable) or 422 (invalid fields)
        return
    }

    sale, err := h.svc.CreateSale(r.Context(), in)
    if err != nil {
        response.Fail(w, r, err)   // 404 / 409 / 412 / 500 / …
        return
    }

    response.Created(w, sale)
}
```

One exit per outcome, no `switch status` in the handler, no error strings built in
the handler.

---

## Optimistic Locking Headers

| Direction | Header | Rule |
|---|---|---|
| Response (read/update) | `ETag: "3"` | the row's `version` |
| Request (update/delete) | `If-Match: "3"` | required; missing → `428 Precondition Required` |
| Success | `200` / `204` with new `ETag` | version incremented by the update |
| Mismatch | `412` + `code: version_conflict` | the client must re-read and retry |

```go
w.Header().Set("ETag", fmt.Sprintf(`"%d"`, sale.Version))
```

---

## Idempotency Header

`POST` accepts an optional `Idempotency-Key` so a client retry cannot create a second
row (`specification.md` §9.1).

| Direction | Header | Rule |
|---|---|---|
| Request (`POST`) | `Idempotency-Key: <uuid>` | optional; generated once per logical create |
| Replay within 24 h | same key | returns the original status and body; no second row |
| Same key, different payload | — | `409`, `code: conflict` |

- The key is scoped to the farm and the endpoint, and stored for 24 hours.
- Only `POST` takes it: `PUT` is idempotent by definition and `DELETE` is guarded by
  `If-Match`.
- A missing header is never an error — idempotency is opt-in, not a precondition.

---

## OpenAPI `responses` Baseline

Every endpoint declares at least:

```yaml
responses:
  "401": { $ref: '#/components/responses/Unauthorized' }
  "403": { $ref: '#/components/responses/Forbidden' }
  "500": { $ref: '#/components/responses/Internal' }
```

Add only the codes that path can actually return:

| Condition in the handler | Add |
|---|---|
| Parses and validates a body | `400`, `422` |
| Reads a resource by ID | `404` |
| Writes a unique-constrained field | `409` |
| Updates or deletes with `If-Match` | `412` |
| Deletes a row with dependents | `409` |
| Role check beyond authentication | `403` |
| Calls an external service | `503`, `504` |

The error schema is one shared component (`Problem`), never a per-endpoint copy.

---

## Deprecating an Endpoint

Two phases. Never skip Phase 1 — removing a route without notice breaks the
frontend silently.

### Phase 1 — still works, marked deprecated

Keep the normal 2xx response and add headers:

```go
w.Header().Set("Deprecation", "true")
w.Header().Set("Sunset", "Sat, 31 Dec 2026 23:59:59 GMT")
w.Header().Set("Link", `</api/v1/sales>; rel="successor-version"`)
```

In `api/openapi.yaml`: `deprecated: true` and a description pointing to the
successor.

### Phase 2 — fully removed

Return `410 Gone`, not `404` — it signals intentional, permanent removal. Keep the
route registered so clients get a clear answer.

```go
r.Get("/old-sales", func(w http.ResponseWriter, r *http.Request) {
    response.Fail(w, r, response.Gone("GET /old-sales was removed; use GET /api/v1/sales"))
})
```

---

## Quick Reference

| Situation | Code |
|---|---|
| Read / update succeeded with a body | 200 |
| Created, returning the resource | 201 |
| Update / delete / state change succeeded | 204 |
| No valid token | 401 |
| Valid token, insufficient role | 403 |
| Row missing, soft-deleted, or another farm's | 404 |
| Duplicate unique value (daily date, journal ref) | 409 |
| `If-Match` version mismatch | 412 |
| `If-Match` missing on a required endpoint | 428 |
| Body parsed, fields invalid | 422 |
| Body unparseable / structurally impossible | 400 |
| Endpoint permanently removed | 410 |
| Unexpected failure | 500 |
| Dependency unavailable | 503 |
| Dependency timeout | 504 |
