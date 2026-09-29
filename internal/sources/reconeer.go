// Package sources: reconeer adapter.
//
// Provider: Reconeer. Docs: https://www.reconeer.com/docs.html
// Endpoint: GET https://www.reconeer.com/api/domain/<d> (cached/indexed data; 404 = no data).
// Auth: "Authorization: Bearer <api_key>". Docs claim anonymous use, but the research observed
// HTTP 401 without a key, so a key is required. Free: 10 queries/day.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"errors"
	"fmt"

	"github.com/lunalully/lunatic/internal/httpx"
)

var reconeerBaseURL = "https://www.reconeer.com"

type reconeer struct{}

func init() { Register(reconeer{}) }

func (reconeer) Info() Info {
	return Info{
		Name: "reconeer", URL: "https://www.reconeer.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 0.2, Burst: 1,
	}
}

func (reconeer) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var resp struct {
		Subdomains *[]any `json:"subdomains"`
		Error      string `json:"error"`
		Message    string `json:"message"`
	}
	err = s.HTTP.GetJSON(ctx, reconeerBaseURL+"/api/domain/"+domain,
		map[string]string{"Authorization": "Bearer " + key, "Accept": "application/json"}, &resp)
	if err != nil {
		var he *httpx.Error
		if errors.As(err, &he) {
			switch he.StatusCode {
			case 404:
				return nil
			case 402:
				return fmt.Errorf("%w: reconeer requires a paid plan for this request (HTTP 402)", ErrAuth)
			}
		}
		return err
	}
	if resp.Subdomains == nil {
		if m := resp.Error + resp.Message; m != "" {
			return merklemapClassify(m)
		}
		return fmt.Errorf("%w: reconeer response has no subdomains field", ErrUnexpected)
	}
	for _, e := range *resp.Subdomains {
		switch t := e.(type) {
		case string:
			emit(t)
		case map[string]any:
			if v, ok := t["subdomain"].(string); ok {
				emit(v)
			}
		}
	}
	return nil
}
