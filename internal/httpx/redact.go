package httpx

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const redacted = "[REDACTED]"

// Sensitive query parameter names (case-insensitive); values are masked even
// when they are not among the configured secrets.
var secretParam = regexp.MustCompile(`(?i)(\b(?:key|k|apikey|api_key|api-key|token|access_token|auth|password|secret|api_secret|username|email|key_id)=)[^&\s"'<>]*`)

// Redact replaces every configured secret (raw and URL-encoded) and the values
// of well-known credential query parameters with "[REDACTED]".
func (c *Client) Redact(s string) string { return redact(s, c.secrets) }

// Redact is the package-level variant for callers that only hold secrets.
func Redact(s string, secrets []string) string { return redact(s, prepSecrets(secrets)) }

func prepSecrets(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		for _, v := range []string{s, url.QueryEscape(s), url.PathEscape(s)} {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func redact(s string, secrets []string) string {
	for _, sec := range secrets {
		s = strings.ReplaceAll(s, sec, redacted)
	}
	return secretParam.ReplaceAllString(s, "${1}"+redacted)
}
