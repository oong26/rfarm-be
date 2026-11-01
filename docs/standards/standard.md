# Development Standards

**English** · [Bahasa Indonesia](standard.id.md)

[← Back to Documentation](../README.md)

---

> Ground rules for writing consistent, maintainable Go code in this repository.
> Every rule here exists to keep the codebase readable and the team aligned.

---

## Core principles

1. **Layers are one-way.** `handler → service → store`. A store never imports a
   service, a service never imports a handler, and nothing imports `cmd/`.
2. **Business rules live in the service layer.** Handlers deal with HTTP; stores
   deal with SQL; services decide what is allowed.
3. **Explicit contracts.** Each layer talks through an interface it depends on:
   `SaleService` for handlers, `SaleStore` for services.
4. **The API contract comes first.** `api/openapi.yaml` is reviewed before the
   handler exists; code follows the spec, not the other way round.
5. **Errors are part of the API.** Return the most specific status and a stable
   code; never let a driver error reach the client.
6. **Money is `decimal.Decimal`.** Never `float64` for money or quantities.
7. **Every query is farm-scoped.** `farm_id` comes from the authenticated
   principal, never from the request body.
8. **One resource, one endpoint.** No `/all`, `/list`, `/paginated` suffixes; no
   query flag that switches an endpoint's purpose.
9. **Test behaviour, not lines.** Cover every branch that changes behaviour; do not
   chase a coverage number.
10. **Documentation ships with the code.** A behaviour change updates OpenAPI and
    its doc in the same commit.
11. **No secrets in the repo.** Configuration lives in `config.yaml` (git-ignored);
    only the template `config.example.yaml` is committed.
12. **Small, reviewable changes.** One logical change per commit and per PR.

---

## Topics

### Naming conventions

- [Struct Naming](struct.md) — `Request`, `Input`, `Params`, sharing a struct
  across layers, decimal fields, where each struct lives
- [API Naming & Nesting](api_naming.md) — plural vs singular, abbreviations,
  CRUD vs action verbs, nesting depth, `/admin`, one purpose per endpoint
- [Method Naming](method_naming.md) — scoped list methods (`By{Parent}`), CRUD
  naming, handler/service/store method symmetry, constructors

### Patterns & logic

- [Service & Store Logic](service_repo.md) — layer responsibilities, store
  interfaces over sqlc, transactions, optimistic locking with `version`
- [HTTP Status Codes](http_status.md) — which code to return and why,
  409 vs 412 vs 422, problem details mapping, OpenAPI responses
- [List API](list_api.md) — pagination as default, `page`/`pageSize`, filters,
  sorting, meta, exports
- [Error Handling](../error_handling.md) — error taxonomy, RFC 9457 bodies,
  wrapping, logging once, client-safe messages

### Quality

- [Unit Testing](unit_test.md) — philosophy, layer responsibilities, file layout,
  branch coverage, baseline cases, fakes, testcontainers
- [Documentation](documentation.md) — where each kind of knowledge lives, document
  types, templates, the English/Indonesian twin rule

### Workflow

- [Development](../development.md) — setup, Make targets, sqlc, migrations, tools
- [Git Workflow](../git_workflow.md) — branches, commits, PRs, releases

---

## How to add or change a standard

1. **Check the problem is real.** A rule earns its place by preventing a mistake
   the team already made, not by describing a preference.
2. **Write the rule in the topic file**, with a short example of the correct form
   and, where useful, the wrong one beside it.
3. **Add or update the entry** in the Topics list above and in
   [docs/README.md](../README.md) if the file is new.
4. **Mirror it in the `.id.md` twin** — same headings, same order.
5. **Open the PR with a real example** from the codebase or the change that
   motivated the rule.

A new standard file uses the same skeleton as the existing ones:

```markdown
# <Topic>

**English** · [Bahasa Indonesia](<topic>.id.md)

[← Back to Standards](standard.md)

---

## Core Rule

One paragraph: what the rule is and why it exists.

---

## Examples

### Correct

### Avoid

---

## Quick Reference

| Rule | Correct | Avoid |
|---|---|---|
```

---

## Pull request checklist

- [ ] The change follows the layer direction: no upward or sideways imports.
- [ ] Business rules are in the service, not in the handler or the SQL.
- [ ] Request/response structs follow [struct.md](struct.md); money uses `decimal`.
- [ ] Routes and methods follow [api_naming.md](api_naming.md) and
      [method_naming.md](method_naming.md).
- [ ] Every new error path returns the right status and a problem-details body
      ([http_status.md](http_status.md)).
- [ ] Every new query filters `farm_id` and `deleted_at IS NULL`.
- [ ] Updates use `version` + `If-Match` and return `412` on mismatch.
- [ ] List endpoints paginate by default and use the shared meta
      ([list_api.md](list_api.md)).
- [ ] Tests cover the happy path, the dependency failure, and the business rule
      that matters ([unit_test.md](unit_test.md)).
- [ ] `api/openapi.yaml` is current; behaviour changes updated their doc in the
      same commit ([documentation.md](documentation.md)).
- [ ] `.id.md` twin updated when a doc changed.
- [ ] No secrets, no commented-out code, no `fmt.Println`.
