// Package sources: shodanct adapter.
//
// Provider:     Shodan Certificate Transparency mirror (https://ctl.shodan.io)
// Docs:         none published beyond the site; endpoint per the site's own
//
//	API links (/api/v1/domain/{domain}/hostnames) and subfinder.
//
// Endpoint:     GET {base}/api/v1/domain/{domain}/hostnames
// Auth:         none observed. The provider does not document limits.
// Pagination:   none (single JSON array).
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

var shodanctBaseURL = "https://ctl.shodan.io"

type shodanct struct{}

func init() { Register(shodanct{}) }

func (shodanct) Info() Info {
	return Info{Name: "shodanct", URL: "https://ctl.shodan.io", Auth: AuthNone, Default: true, RPS: 1, Burst: 1}
}

func (shodanct) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	resp, err := s.HTTP.Get(ctx, shodanctBaseURL+"/api/v1/domain/"+url.PathEscape(domain)+"/hostnames", nil)
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return fmt.Errorf("%w: shodanct: invalid JSON: %v", ErrUnexpected, err)
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		// Tolerate an object wrapper; error objects become typed errors.
		var obj struct {
			Hostnames []string `json:"hostnames"`
			Error     string   `json:"error"`
		}
		if err2 := json.Unmarshal(raw, &obj); err2 != nil {
			return fmt.Errorf("%w: shodanct: unexpected response shape", ErrUnexpected)
		}
		if obj.Error != "" {
			return fmt.Errorf("%w: shodanct: %s", ErrUnexpected, obj.Error)
		}
		list = obj.Hostnames
	}
	for _, h := range list {
		emit(h)
	}
	return nil
}
