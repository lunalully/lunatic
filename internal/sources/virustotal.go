// Package sources: virustotal adapter.
//
// Provider:     VirusTotal (https://www.virustotal.com)
// Docs:         https://docs.virustotal.com/reference/domains-relationships
// Endpoint:     GET {base}/api/v3/domains/{domain}/subdomains?limit=40[&cursor=C]
// Auth:         API key, header "x-apikey". Free public keys: 4 req/min,
//
//	500 req/day, non-commercial use only (docs), hence RPS 0.066.
//
// Pagination:   meta.cursor while non-empty; bounded by Session.MaxPages.
// Checked:      2026-09-29
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/lunalully/lunatic/internal/httpx"
)

var virustotalBaseURL = "https://www.virustotal.com"

type virustotal struct{}

func init() { Register(virustotal{}) }

func (virustotal) Info() Info {
	return Info{
		Name: "virustotal", URL: "https://www.virustotal.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 0.066, Burst: 1,
	}
}

func (virustotal) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"x-apikey": key}
	cursor := ""
	for page := 0; page < s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := virustotalBaseURL + "/api/v3/domains/" + url.PathEscape(domain) + "/subdomains?limit=40"
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}
		var r struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			Meta struct {
				Cursor string `json:"cursor"`
			} `json:"meta"`
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := s.HTTP.GetJSON(ctx, u, hdr, &r); err != nil {
			var he *httpx.Error
			if errors.As(err, &he) && he.StatusCode == 404 { // NotFoundError: unknown domain
				return nil
			}
			return err
		}
		if r.Error != nil {
			switch r.Error.Code {
			case "WrongCredentialsError", "AuthenticationRequiredError", "UserNotActiveError", "ForbiddenError":
				return fmt.Errorf("%w: virustotal: %s", ErrAuth, r.Error.Code)
			case "QuotaExceededError", "TooManyRequestsError", "TransientError":
				return fmt.Errorf("%w: virustotal: %s", ErrRateLimited, r.Error.Code)
			}
			return fmt.Errorf("%w: virustotal: %s: %s", ErrUnexpected, r.Error.Code, r.Error.Message)
		}
		for _, d := range r.Data {
			emit(d.ID)
		}
		if r.Meta.Cursor == "" || r.Meta.Cursor == cursor || len(r.Data) == 0 {
			return nil
		}
		cursor = r.Meta.Cursor
	}
	return nil
}
