// Provider: GitHub code search (https://github.com).
// Docs: https://docs.github.com/en/rest/search/search#search-code
// Endpoint: GET https://api.github.com/search/code?q="<domain>"&per_page=100&page=N with
// Accept: application/vnd.github.text-match+json. Only the search API response is used; repositories, file contents
// and raw.githubusercontent.com are never fetched.
// Auth: required (personal access token, Authorization: Bearer). Code search limits: 10 requests/minute, at most
// 1,000 results (10 pages). HTTP 403/429 carrying Retry-After or X-RateLimit-Remaining: 0 -> ErrRateLimited.
// Extraction: hostnames only, via scope.FindInText over text_matches[].fragment; fragment text is discarded.
// HTTP 422 on a later page (result window exceeded) ends pagination.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/lunalully/lunatic/internal/httpx"
	"github.com/lunalully/lunatic/internal/scope"
)

var githubBaseURL = "https://api.github.com"

const githubPerPage = 100

type github struct{}

func init() { Register(github{}) }

func (github) Info() Info {
	return Info{Name: "github", URL: "https://github.com", Auth: AuthRequired,
		CredFields: []string{"token"}, Default: true, RPS: 0.15, Burst: 1}
}

func (github) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	token, err := PickKey(s.Creds, "token")
	if err != nil {
		return err
	}
	headers := map[string]string{
		"Authorization":        "Bearer " + token,
		"Accept":               "application/vnd.github.text-match+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	seen := map[string]bool{}
	for page := 1; page <= s.MaxPages && page <= 10; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := githubBaseURL + "/search/code?q=" + url.QueryEscape(`"`+domain+`"`) + "&per_page=" + strconv.Itoa(githubPerPage) + "&page=" + strconv.Itoa(page)
		resp, err := s.HTTP.Get(ctx, u, headers)
		if err != nil {
			var he *httpx.Error
			if errors.As(err, &he) {
				if (he.StatusCode == 403 || he.StatusCode == 429) && resp != nil &&
					(resp.Header.Get("Retry-After") != "" || resp.Header.Get("X-RateLimit-Remaining") == "0") {
					return fmt.Errorf("%w: github search rate limit", ErrRateLimited)
				}
				if he.StatusCode == 422 && page > 1 {
					return nil
				}
			}
			return err
		}
		var out struct {
			Items []struct {
				TextMatches []struct {
					Fragment string `json:"fragment"`
				} `json:"text_matches"`
			} `json:"items"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(resp.Body, &out); err != nil {
			return fmt.Errorf("%w: github invalid JSON: %v", ErrUnexpected, err)
		}
		for _, it := range out.Items {
			for _, tm := range it.TextMatches {
				for _, n := range scope.FindInText(tm.Fragment, domain) {
					if !seen[n] {
						seen[n] = true
						emit(n)
					}
				}
			}
		}
		if len(out.Items) < githubPerPage {
			return nil
		}
	}
	return nil
}
