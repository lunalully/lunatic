// Provider: FOFA (https://fofa.info / https://en.fofa.info).
// Docs: https://en.fofa.info/api (search interface: /api/v1/search/all).
// Endpoint: GET https://fofa.info/api/v1/search/all?key=<KEY>&qbase64=<base64 of domain="x">&fields=host&size=N&page=P&full=true
// Auth: required; API key in the "key" query parameter (the only place the API takes it). The legacy "email"
// parameter is sent only when the optional credential field "email" is set (key-only use is supported by the
// official SDK; older accounts may still require the email).
// Response: {"error":bool,"errmsg":"","size":N,"results":["host[:port]" | "scheme://host[:port]"]}. error:true is mapped to
// typed errors. Only the hostname part is emitted (scheme, port and path stripped; IPs are dropped by the runner).
// Paging stops on a short page or at MaxPages. Free-tier limits and F-coin costs are unverified.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var fofaBaseURL = "https://fofa.info"

const fofaSize = 1000

type fofa struct{}

func init() { Register(fofa{}) }

func (fofa) Info() Info {
	return Info{Name: "fofa", URL: "https://fofa.info", Auth: AuthRequired,
		CredFields: []string{"key"}, OptCredFields: []string{"email"}, Default: true, RPS: 0.5, Burst: 1}
}

func fofaHost(v string) string {
	if i := strings.Index(v, "://"); i >= 0 {
		v = v[i+3:]
	}
	if i := strings.IndexAny(v, "/?#"); i >= 0 {
		v = v[:i]
	}
	if i := strings.LastIndex(v, "@"); i >= 0 {
		v = v[i+1:]
	}
	if i := strings.LastIndex(v, ":"); i >= 0 && !strings.Contains(v[i+1:], ":") && !strings.HasPrefix(v, "[") {
		v = v[:i]
	}
	return v
}

func (fofa) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "key")
	if err != nil {
		return err
	}
	emailParam := ""
	if e := s.Creds["email"]; e != "" {
		emailParam = "&email=" + url.QueryEscape(e)
	}
	q := base64.StdEncoding.EncodeToString([]byte(`domain="` + domain + `"`))
	for page := 1; page <= s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := fofaBaseURL + "/api/v1/search/all?key=" + url.QueryEscape(key) + "&qbase64=" + url.QueryEscape(q) +
			emailParam + "&fields=host&full=true&size=" + strconv.Itoa(fofaSize) + "&page=" + strconv.Itoa(page)
		var out struct {
			Error   bool     `json:"error"`
			Errmsg  string   `json:"errmsg"`
			Results []string `json:"results"`
		}
		if err := s.HTTP.GetJSON(ctx, u, nil, &out); err != nil {
			return err
		}
		if out.Error {
			low := strings.ToLower(out.Errmsg)
			switch {
			case strings.Contains(low, "insufficient"), strings.Contains(low, "limit"), strings.Contains(low, "too many"):
				return fmt.Errorf("%w: fofa: %.80s", ErrRateLimited, out.Errmsg)
			case strings.Contains(low, "invalid"), strings.Contains(low, "email"), strings.Contains(low, "key"),
				strings.Contains(low, "unauthor"), strings.Contains(low, "401"), strings.Contains(low, "permission"):
				return fmt.Errorf("%w: fofa: %.80s", ErrAuth, out.Errmsg)
			}
			return fmt.Errorf("%w: fofa error: %.80s", ErrUnexpected, out.Errmsg)
		}
		for _, r := range out.Results {
			if h := fofaHost(r); h != "" {
				emit(h)
			}
		}
		if len(out.Results) < fofaSize {
			return nil
		}
	}
	return nil
}
