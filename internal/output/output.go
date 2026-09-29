// Package output writes results. Output is data only: never ANSI, never
// banners or summaries.
//
// Destination policy (Subfinder-like): results always go to stdout; -o FILE
// additionally writes the same bytes to FILE (created/truncated, 0644).
package output

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/lunalully/lunatic/internal/runner"
)

// WriteTXT writes one subdomain per line. Identical names are printed once
// even if they appear under several input domains.
func WriteTXT(w io.Writer, findings []runner.Finding) error {
	bw := bufio.NewWriter(w)
	seen := make(map[string]bool, len(findings))
	for _, f := range findings {
		if seen[f.Subdomain] {
			continue
		}
		seen[f.Subdomain] = true
		bw.WriteString(f.Subdomain)
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

type record struct {
	Domain    string   `json:"domain"`
	Subdomain string   `json:"subdomain"`
	Sources   []string `json:"sources"`
}

// WriteJSONL writes one JSON object per finding:
// {"domain":"example.com","subdomain":"api.example.com","sources":["crtsh"]}
func WriteJSONL(w io.Writer, findings []runner.Finding) error {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	enc.SetEscapeHTML(false)
	for _, f := range findings {
		srcs := f.Sources
		if srcs == nil {
			srcs = []string{}
		}
		if err := enc.Encode(record{f.Domain, f.Subdomain, srcs}); err != nil {
			return err
		}
	}
	return bw.Flush()
}
