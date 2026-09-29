// Package sources: onyphe adapter.
//
// Provider: ONYPHE. Docs: https://search.onyphe.io/docs/general-apis/search
// Endpoint: GET https://www.onyphe.io/api/v2/search/?q=category:resolver domain:<d>&page=<n>&size=<n>
// (stored data only). The v3 on-demand APIs trigger live actions and are NEVER used:
// onypheAllowedPath is the only path prefix requested.
// Auth: "Authorization: bearer <api_key>". Paid plans (no free tier confirmed).
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
// The "resolver" category name comes from the Subfinder reference.
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

var onypheBaseURL = "https://www.onyphe.io"

const onypheAllowedPath = "/api/v2/search/"

var onypheSize = 1000

type onyphe struct{}

func init() { Register(onyphe{}) }

func (onyphe) Info() Info {
	return Info{
		Name: "onyphe", URL: "https://www.onyphe.io",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

func (onyphe) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"Authorization": "bearer " + key, "Accept": "application/json"}
	for page := 1; page <= merklemapPages(s); page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var resp struct {
			Error   any               `json:"error"`
			Text    string            `json:"text"`
			Message string            `json:"message"`
			Status  string            `json:"status"`
			MaxPage int               `json:"max_page"`
			Results *[]map[string]any `json:"results"`
		}
		u := onypheBaseURL + onypheAllowedPath + "?q=" + url.QueryEscape("category:resolver domain:"+domain) +
			"&page=" + strconv.Itoa(page) + "&size=" + strconv.Itoa(onypheSize)
		if err := s.HTTP.GetJSON(ctx, u, hdr, &resp); err != nil {
			return err
		}
		if e, ok := resp.Error.(float64); (ok && e != 0) || (resp.Error != nil && !ok && resp.Error != "" && resp.Error != false) {
			msg := resp.Text
			if msg == "" {
				msg = resp.Message
			}
			if msg == "" {
				msg = fmt.Sprint(resp.Error)
			}
			return merklemapClassify(msg)
		}
		if resp.Results == nil {
			return fmt.Errorf("%w: onyphe response has no results field", ErrUnexpected)
		}
		if len(*resp.Results) == 0 {
			return nil
		}
		for _, r := range *resp.Results {
			for _, f := range []string{"hostname", "subdomains"} {
				for _, h := range merklemapFlatten(r[f]) {
					emit(h)
				}
			}
		}
		if resp.MaxPage > 0 && page >= resp.MaxPage {
			return nil
		}
	}
	return nil
}
