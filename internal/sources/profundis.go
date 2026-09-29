// Package sources: profundis adapter.
//
// Provider: Profundis. Docs: https://docs.profundis.io/api/api-usage
// Endpoint: POST https://api.profundis.io/api/v2/common/data/subdomains body {"domain":"<d>"},
// streamed text, one subdomain per line. NOTE: endpoint from Subfinder reference, not in official docs
// (the docs list only /common/data/hosts and /common/data/dns).
// Auth: "X-API-KEY: <api_key>"; paid subscription/credits required (each call costs credits).
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lunalully/lunatic/internal/scope"
)

var profundisBaseURL = "https://api.profundis.io"

type profundis struct{}

func init() { Register(profundis{}) }

func (profundis) Info() Info {
	return Info{
		Name: "profundis", URL: "https://profundis.io",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

func (profundis) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"domain": domain})
	resp, err := s.HTTP.Post(ctx, profundisBaseURL+"/api/v2/common/data/subdomains",
		map[string]string{"X-API-KEY": key, "Accept": "text/event-stream", "Content-Type": "application/json"}, body)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(resp.Body), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") {
			continue
		}
		if line[0] == '{' || line[0] == '[' {
			var e struct {
				Error   string `json:"error"`
				Detail  string `json:"detail"`
				Message string `json:"message"`
			}
			if json.Unmarshal([]byte(line), &e) == nil {
				for _, m := range []string{e.Error, e.Detail, e.Message} {
					if m != "" {
						return merklemapClassify(m)
					}
				}
			}
			for _, n := range scope.Extract(line, domain) {
				emit(n)
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "error") {
			return merklemapClassify(line)
		}
		emit(line)
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") {
		return fmt.Errorf("%w: profundis returned HTML", ErrUnexpected)
	}
	return nil
}
