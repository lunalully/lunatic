// Provider: crt.sh (Sectigo certificate transparency search).
// Docs: no official API page exists; behaviour from the crt.sh Google group and
// public tools. Endpoint: GET /?q=%25.<domain>&output=json (HTTP JSON only; the
// direct PostgreSQL interface is deliberately not used). Auth: none.
// Slow and sometimes unavailable; rely on the per-source timeout.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from
// build environment (egress blocked).
package sources

import (
	"context"
	"net/url"
	"strings"
)

var crtshBaseURL = "https://crt.sh"

type crtsh struct{}

func init() { Register(crtsh{}) }

func (crtsh) Info() Info {
	return Info{Name: "crtsh", URL: "https://crt.sh", Auth: AuthNone, Default: true, RPS: 0.2, Burst: 1}
}

func (crtsh) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	var rows []struct {
		NameValue string `json:"name_value"`
	}
	u := crtshBaseURL + "/?q=" + url.QueryEscape("%."+domain) + "&output=json"
	if err := s.HTTP.GetJSON(ctx, u, map[string]string{"Accept": "application/json"}, &rows); err != nil {
		return err
	}
	for _, r := range rows {
		for _, n := range strings.Split(r.NameValue, "\n") {
			if n = strings.TrimSpace(n); n != "" && !strings.Contains(n, "@") { // skip e-mail SANs (personal data)
				emit(n)
			}
		}
	}
	return nil
}
