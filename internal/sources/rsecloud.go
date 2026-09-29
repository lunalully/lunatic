// Package sources: rsecloud adapter.
//
// Provider: RSECloud. Docs: https://documenter.getpostman.com/view/33929903/2sA35G4N2c (not readable
// by the research tooling; endpoint from the Subfinder reference, not confirmed in official docs).
// Endpoint: GET https://api.rsecloud.com/api/v2/subdomains/passive/<d>?page=<n>.
// The /active/ endpoint is NEVER called (semantics unconfirmed): rsecloudAllowedPath is the only prefix used.
// Auth: "X-API-Key: <api_key>". Plan/limits unknown.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"fmt"
	"strconv"
)

var rsecloudBaseURL = "https://api.rsecloud.com"

const rsecloudAllowedPath = "/api/v2/subdomains/passive/"

type rsecloud struct{}

func init() { Register(rsecloud{}) }

func (rsecloud) Info() Info {
	return Info{
		Name: "rsecloud", URL: "https://rsecloud.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

func (rsecloud) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"X-API-Key": key, "Accept": "application/json"}
	for page := 1; page <= merklemapPages(s); page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var resp struct {
			Data       *[]string `json:"data"`
			TotalPages int       `json:"total_pages"`
			Error      string    `json:"error"`
			Message    string    `json:"message"`
			Detail     string    `json:"detail"`
		}
		u := rsecloudBaseURL + rsecloudAllowedPath + domain + "?page=" + strconv.Itoa(page)
		if err := s.HTTP.GetJSON(ctx, u, hdr, &resp); err != nil {
			return err
		}
		if resp.Data == nil {
			if m := resp.Error + resp.Message + resp.Detail; m != "" {
				return merklemapClassify(m)
			}
			return fmt.Errorf("%w: rsecloud response has no data field", ErrUnexpected)
		}
		if len(*resp.Data) == 0 {
			return nil
		}
		for _, h := range *resp.Data {
			emit(h)
		}
		if resp.TotalPages > 0 && page >= resp.TotalPages {
			return nil
		}
	}
	return nil
}
