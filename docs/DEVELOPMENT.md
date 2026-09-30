# Lunatic developer guide

Lunatic is a strictly PASSIVE subdomain recon tool in Go (module `github.com/lunalully/lunatic`, binary `lunatic`). It never scans, resolves or contacts the target; it only queries third-party data providers.

## Package layout
- `cmd/lunatic` - main; calls `cli.Main`.
- `internal/cli` - flags, selection, exit codes (0 ok, 1 usage, 2 none succeeded, 3 partial, 130 interrupted).
- `internal/runner` - runs sources concurrently, normalizes/dedupes names, records provenance and per-source outcomes.
- `internal/sources` - the contract (`Source`, `Info`, `Session`, registry, typed errors, `PickKey`). Adapters live here.
- `internal/httpx` - the ONLY HTTP client. Passive guard, rate limit, retries, size limit, redaction.
- `internal/scope` - `ParseDomain`, `Normalize`, `Extract`, `FindInText`.
- `internal/config`, `internal/output`, `internal/banner`, `internal/term`, `internal/errs`, `internal/version`.
- `vendor/` holds `golang.org/x/net/idna` (+ the `x/text` packages it needs), copied from Go's std vendor tree; the build uses `-mod=vendor` automatically because `vendor/` exists. LICENSE files are kept. When the network is available, switch to normal modules with `go mod tidy && go mod vendor` (refresh) or `rm -rf vendor && go mod tidy` (drop vendoring, then `go.sum` is generated).

## File layout
One adapter consists of exactly these files:
- `internal/sources/<name>.go`
- `internal/sources/<name>_test.go`
- `internal/sources/testdata/<name>/...` (fixtures)

Adapter changes should not touch core packages or other adapters. If the contract seems insufficient, open an issue to discuss it before changing it.

## Writing an adapter
```go
package sources

var crtshBaseURL = "https://crt.sh" // REQUIRED package-level var, tests reassign it

type crtsh struct{}

func init() { Register(crtsh{}) }

func (crtsh) Info() Info {
    return Info{Name: "crtsh", URL: "https://crt.sh", Auth: AuthNone, Default: true, RPS: 1, Burst: 1}
}

func (crtsh) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
    var rows []struct{ NameValue string `json:"name_value"` }
    if err := s.HTTP.GetJSON(ctx, crtshBaseURL+"/?q=%25."+url.QueryEscape(domain)+"&output=json", nil, &rows); err != nil {
        return err
    }
    for _, r := range rows { emit(r.NameValue) } // raw; the runner normalizes and scopes
    return nil
}
```
Rules:
1. Use `s.HTTP` (httpx) for every request. Never `net/http` directly, and never a custom client/transport.
2. Passive only: call the provider's query/search API endpoints. Never trigger scans/jobs, never contact the target or any discovered host, never resolve DNS for results, never follow links found in results.
3. Credentials come from `s.Creds` (`PickKey(s.Creds, "api_key")` -> `ErrNoKey`). Declare required ones in `Info.CredFields` and set `Auth`; declare extras that are not needed to run in `Info.OptCredFields` (e.g. intelx `host`, fofa `email`). Both are loaded into `s.Creds` (env `LUNATIC_<SOURCE>_<FIELD>` or config file); readiness (`--list-sources`, runner skip) depends on `CredFields` only. Send keys in headers when the provider allows; never log them. Policy: `Default: true` + `AuthRequired` means "runs by default only when its credentials are configured"; without them it is skipped/`needs key`. `Default: false` sources run only with `-s` or `--all`. All credential values are redacted by the runner.
4. Errors: return typed errors (`ErrNoKey, ErrAuth, ErrRateLimited, ErrTimeout, ErrUnexpected, ErrUnavailable, ErrBlockedHost, ErrDisabled`, wrapped with `%w` is fine). httpx already maps HTTP failures: 401/403 auth, 429 rate limit, 5xx unavailable, other 4xx unexpected. If a provider uses 404 for "no results", check `var he *httpx.Error; errors.As(err,&he) && he.StatusCode==404` and return nil. Zero results is success (nil). Parse failures -> `fmt.Errorf("%w: ...", ErrUnexpected)`.
5. Pagination: stop at `s.MaxPages` (default 10) and when `ctx` is done. Respect provider limits via `Info.RPS/Burst`; do not sleep manually.
6. Emit RAW names with `emit(raw)` (blobs with newlines are fine; use `scope.Extract`/`scope.FindInText` for text/HTML). Do not normalize, filter or dedupe (the runner does).
7. Keep only names. Do not retain or emit personal data (emails, WHOIS contacts, IPs). The one exception: an IP the provider already returned may be handed to `s.IP(ip)` (see Phase 2 below); never store it elsewhere.
8. Disabled providers: register with `Disabled: true` and `DisabledReason`; `Enumerate` returns `ErrDisabled`.
9. Register in `init()` via `Register`. Names are lowercase and unique (duplicate panics).

## Phase 2 sources (IP based)
Some providers only accept IPs (internetdb). Lunatic never resolves DNS, so such a source sets `Info.Phase2 = true`: the runner runs it after every phase-1 source, per domain, with `Session.IPs` = unique public IPs that phase-1 adapters reported via `s.IP(ip)` (nil-safe; only for addresses already present in the provider response, never an extra request). Private/loopback/link-local/reserved addresses are dropped by the runner. A phase-2 adapter sends each IP only to its provider as a path/query parameter, caps how many it sends, treats 404 as "no data", and succeeds with zero results when `Session.IPs` is empty. It never contacts the IPs. Phase-2 sessions have no `ReportIP`.

## Tests (offline only)
Use `httptest` and fixtures in `testdata/<name>/`. Point the adapter at the test server by reassigning its base URL var, restoring it afterwards:
```go
srv := httptest.NewServer(handler)
defer srv.Close()
old := crtshBaseURL; crtshBaseURL = srv.URL; defer func() { crtshBaseURL = old }()
s := &Session{HTTP: httpx.New(httpx.Options{SourceName: "crtsh", TargetDomain: "example.com", BackoffBase: time.Millisecond}), Creds: map[string]string{}, MaxPages: 10}
var got []string
err := crtsh{}.Enumerate(ctx, "example.com", s, func(r string){ got = append(got, r) })
```
The guard only blocks the target domain, so 127.0.0.1 test servers work. Cover: success, empty result, pagination stop at MaxPages, auth error (401 -> `ErrAuth`), rate limit, malformed JSON (-> `ErrUnexpected`), missing key. Never hit the network in tests.

## Checks before sending a change
`gofmt -l .` (empty), `go vet ./...`, `go test ./...`, `go test -race ./...`, `go build ./cmd/lunatic`. `internal/cli` has an end-to-end passive-rules test (all sources through a fake transport) and `internal/sources/contract_test.go` enforces adapter rules; keep them green. Offline only: never add tests that need the network.
