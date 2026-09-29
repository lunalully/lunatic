// Package scope validates target domains and normalizes candidate names.
//
// Label rules: letters, digits and hyphens (LDH) plus underscore. Underscore is
// deliberately allowed in candidate names because real DNS names such as
// _dmarc.example.com or _acme-challenge.example.com appear in certificate and
// passive-DNS data. Labels are 1-63 bytes, must not start or end with a hyphen,
// and the full name is at most 253 bytes. Target domains (ParseDomain) are
// stricter: no underscores.
package scope

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"golang.org/x/net/idna"
)

var lenient = idna.New(idna.MapForLookup(), idna.BidiRule(), idna.Transitional(false))

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func labelErr(label string, allowUnderscore bool) error {
	if len(label) == 0 {
		return errors.New("empty label")
	}
	if len(label) > 63 {
		return fmt.Errorf("label %q is longer than 63 characters", label)
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return fmt.Errorf("label %q starts or ends with a hyphen", label)
	}
	for i := 0; i < len(label); i++ {
		c := label[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
		case c == '_' && allowUnderscore:
		default:
			return fmt.Errorf("label %q contains invalid character %q", label, string(c))
		}
	}
	return nil
}

func validName(name string, allowUnderscore bool) error {
	if name == "" {
		return errors.New("empty name")
	}
	if len(name) > 253 {
		return errors.New("name is longer than 253 characters")
	}
	for _, l := range strings.Split(name, ".") {
		if err := labelErr(l, allowUnderscore); err != nil {
			return err
		}
	}
	return nil
}

// ParseDomain validates a user-supplied target domain and returns its
// canonical form (lowercase, punycode, no trailing dot). Subdomain scopes
// such as app.example.com are preserved as given.
func ParseDomain(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", errors.New("empty domain")
	}
	if strings.Contains(s, "://") || strings.Contains(s, "/") {
		return "", fmt.Errorf("%q looks like a URL; give a bare domain such as example.com", input)
	}
	if ip := net.ParseIP(strings.Trim(s, "[]")); ip != nil {
		return "", fmt.Errorf("%q is an IP address, not a domain", input)
	}
	if strings.Contains(s, "@") {
		return "", fmt.Errorf("%q must not contain user info", input)
	}
	if strings.Contains(s, ":") {
		return "", fmt.Errorf("%q must not contain a port", input)
	}
	if strings.Contains(s, "*") {
		return "", fmt.Errorf("%q must not contain wildcards", input)
	}
	s = strings.ToLower(s)
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return "", errors.New("empty domain")
	}
	if !isASCII(s) {
		a, err := idna.Lookup.ToASCII(s)
		if err != nil {
			return "", fmt.Errorf("%q is not a valid internationalized domain: %v", input, err)
		}
		s = a
	}
	if err := validName(s, false); err != nil {
		return "", fmt.Errorf("invalid domain %q: %v", input, err)
	}
	if !strings.Contains(s, ".") {
		return "", fmt.Errorf("%q is a single label; a domain like example.com is required", input)
	}
	return s, nil
}

// Normalize cleans one raw candidate and returns it only if it is a strict
// subdomain of domain (label-boundary match; the domain itself is rejected).
// domain must already be canonical (see ParseDomain).
func Normalize(raw, domain string) (string, bool) {
	s := strings.Trim(raw, " \t\r\n\"'`,;")
	if s == "" {
		return "", false
	}
	s = strings.ToLower(s)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#\\"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if strings.ContainsAny(s, "[]") {
		return "", false // IPv6 literal or garbage
	}
	if i := strings.Index(s, ":"); i >= 0 {
		port := s[i+1:]
		if port == "" || strings.Trim(port, "0123456789") != "" {
			return "", false
		}
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".")
	for strings.HasPrefix(s, "*.") {
		s = s[2:]
	}
	if s == "" {
		return "", false
	}
	if !isASCII(s) {
		a, err := lenient.ToASCII(s)
		if err != nil {
			return "", false
		}
		s = a
	}
	if validName(s, true) != nil {
		return "", false
	}
	if !strings.HasSuffix(s, "."+domain) {
		return "", false
	}
	return s, true
}

func splitBlob(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n', ',', ';', '"', '\'', '<', '>', '(', ')', '[', ']', '{', '}', '|', '`':
		return true
	}
	return false
}

var escapes = strings.NewReplacer(`\n`, " ", `\r`, " ", `\t`, " ")

// Extract splits a blob (newline/comma/space separated lists, certificate
// name_value fields, JSON-ish text) and returns the normalized in-scope names,
// deduplicated, in first-seen order.
func Extract(raw, domain string) []string {
	var out []string
	seen := map[string]bool{}
	for _, tok := range strings.FieldsFunc(escapes.Replace(raw), splitBlob) {
		if n, ok := Normalize(tok, domain); ok && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

func isLabelChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// FindInText finds every occurrence of a name ending in .<domain> inside
// arbitrary text (HTML, plain text) and returns the normalized in-scope names,
// deduplicated, in first-seen order. Matching is case-insensitive and respects
// label boundaries (notexample.com and example.com.evil.test do not match).
func FindInText(text, domain string) []string {
	re := regexp.MustCompile(`[a-z0-9_.-]+\.` + regexp.QuoteMeta(domain))
	lower := strings.ToLower(text)
	var out []string
	seen := map[string]bool{}
	for _, m := range re.FindAllStringIndex(lower, -1) {
		end := m[1]
		if end < len(lower) {
			if isLabelChar(lower[end]) {
				continue
			}
			if lower[end] == '.' && end+1 < len(lower) && isLabelChar(lower[end+1]) {
				continue
			}
		}
		cand := strings.TrimLeft(lower[m[0]:m[1]], ".-")
		if n, ok := Normalize(cand, domain); ok && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
