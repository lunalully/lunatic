// Provider: Intelligence X phonebook (https://intelx.io, https://phonebook.cz).
// Docs: https://help.intelx.io/docs/api/ ; SDK https://github.com/IntelligenceX/SDK
// Endpoints: POST https://<host>/phonebook/search?k=<KEY> {"term":"<domain>","maxresults":100000,"media":0,"target":1,"timeout":20}
// -> {"id":"...","status":0}; then GET https://<host>/phonebook/search/result?k=<KEY>&id=<ID>&limit=10000 polled
// (status 0 = results, 3 = not ready yet, 1 = finished, 2 = unknown id) up to MaxPages polls.
// Auth: required (api_key); optional field: host. The key is valid only on the API host of its account
// (free.intelx.io for free accounts, 2.intelx.io for paid, public.intelx.io for unregistered). "host" defaults to
// free.intelx.io when empty.
// The key is sent as the k query parameter (used by the phonebook API in third-party code) and as the x-key header.
// Only selectors that are plain hostnames are emitted; e-mails, URLs and other selector types are discarded.
// The file/bucket endpoints (which return leak content) are never called. Free-tier quota unverified;
// third-party integrations formally need the API license.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var intelxBaseURL = "https://free.intelx.io"

type intelx struct{}

func init() { Register(intelx{}) }

func (intelx) Info() Info {
	return Info{Name: "intelx", URL: "https://intelx.io", Auth: AuthRequired,
		CredFields: []string{"api_key"}, OptCredFields: []string{"host"}, Default: true, RPS: 1, Burst: 1}
}

func intelxBase(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return intelxBaseURL, nil
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	u, err := url.Parse(host)
	if err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") || u.User != nil || u.RawQuery != "" {
		return "", fmt.Errorf("%w: intelx host must be a bare host name", ErrUnexpected)
	}
	return u.Scheme + "://" + u.Host, nil
}

func (intelx) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	base, err := intelxBase(s.Creds["host"])
	if err != nil {
		return err
	}
	headers := map[string]string{"x-key": key, "Accept": "application/json"}
	body, _ := json.Marshal(map[string]any{"term": domain, "maxresults": 100000, "media": 0, "target": 1, "timeout": 20})
	var start struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	}
	if err := s.HTTP.PostJSON(ctx, base+"/phonebook/search?k="+url.QueryEscape(key), headers, body, &start); err != nil {
		if g2status(err) == 402 {
			return fmt.Errorf("%w: intelx credits exhausted", ErrRateLimited)
		}
		return err
	}
	switch start.Status {
	case 0:
	case 2:
		return fmt.Errorf("%w: intelx concurrent search limit", ErrRateLimited)
	default:
		return fmt.Errorf("%w: intelx search rejected (status %d)", ErrUnexpected, start.Status)
	}
	if start.ID == "" {
		return fmt.Errorf("%w: intelx returned no search id", ErrUnexpected)
	}
	got := 0
	for poll := 0; poll < s.MaxPages; poll++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := base + "/phonebook/search/result?k=" + url.QueryEscape(key) + "&id=" + url.QueryEscape(start.ID) + "&limit=10000"
		var out struct {
			Selectors []struct {
				Value string `json:"selectorvalue"`
			} `json:"selectors"`
			Status int `json:"status"`
		}
		if err := s.HTTP.GetJSON(ctx, u, headers, &out); err != nil {
			if g2status(err) == 402 {
				return fmt.Errorf("%w: intelx credits exhausted", ErrRateLimited)
			}
			return err
		}
		for _, sel := range out.Selectors {
			v := strings.TrimSpace(sel.Value)
			if v == "" || strings.ContainsAny(v, "@/: \t") {
				continue
			}
			got++
			emit(v)
		}
		switch out.Status {
		case 1:
			return nil
		case 2:
			return fmt.Errorf("%w: intelx search id not found", ErrUnexpected)
		}
	}
	if got == 0 {
		return fmt.Errorf("%w: intelx search not finished after %d polls", ErrTimeout, s.MaxPages)
	}
	return nil
}
