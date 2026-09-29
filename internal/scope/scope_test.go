package scope

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDomain(t *testing.T) {
	good := map[string]string{
		"example.com":           "example.com",
		"  Example.COM. ":       "example.com",
		"app.example.com":       "app.example.com",
		"bücher.example":        "xn--bcher-kva.example",
		"EXAMPLE.co.uk":         "example.co.uk",
		"xn--bcher-kva.example": "xn--bcher-kva.example",
	}
	for in, want := range good {
		got, err := ParseDomain(in)
		if err != nil || got != want {
			t.Errorf("ParseDomain(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"", "  ", ".", "https://example.com", "example.com/path", "1.2.3.4", "::1", "[::1]",
		"example.com:8080", "*.example.com", "localhost", "-a.example.com", "a-.example.com",
		"a..example.com", "exa mple.com", "a_b.example.com", "user@example.com",
		strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 130) + "com",
	}
	for _, in := range bad {
		if got, err := ParseDomain(in); err == nil {
			t.Errorf("ParseDomain(%q) = %q; want error", in, got)
		}
	}
}

func TestNormalize(t *testing.T) {
	const d = "example.com"
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"api.example.com", "api.example.com", true},
		{"  API.Example.COM  ", "api.example.com", true},
		{`"api.example.com"`, "api.example.com", true},
		{"api.example.com.", "api.example.com", true},
		{"*.api.example.com", "api.example.com", true},
		{"*.*.api.example.com", "api.example.com", true},
		{"a.*.example.com", "", false},
		{"https://user:pw@api.example.com:8443/path?q=1", "api.example.com", true},
		{"api.example.com:443", "api.example.com", true},
		{"api.example.com/foo", "api.example.com", true},
		{"_dmarc.example.com", "_dmarc.example.com", true},
		{"tést.example.com", "xn--tst-bma.example.com", true},
		{"example.com", "", false},
		{"*.example.com", "", false},
		{"notexample.com", "", false},
		{"www.notexample.com", "", false},
		{"example.com.evil.test", "", false},
		{"api.example.com.evil.test", "", false},
		{"api.example.org", "", false},
		{"", "", false},
		{"-bad.example.com", "", false},
		{"a b.example.com", "", false},
		{"1.2.3.4", "", false},
		{"[::1]", "", false},
		{"a..example.com", "", false},
		{".example.com", "", false},
		{strings.Repeat("a", 64) + ".example.com", "", false},
	}
	for _, tc := range tests {
		got, ok := Normalize(tc.in, d)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Normalize(%q) = %q,%v; want %q,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestExtract(t *testing.T) {
	blob := "a.example.com\n*.b.example.com\nEXAMPLE.com\nnotexample.com\na.example.com, c.example.com evil.com"
	want := []string{"a.example.com", "b.example.com", "c.example.com"}
	if got := Extract(blob, "example.com"); !reflect.DeepEqual(got, want) {
		t.Errorf("Extract = %v; want %v", got, want)
	}
	js := `{"name_value":"x.example.com\ny.example.com","issuer":"foo"}`
	if got := Extract(js, "example.com"); !reflect.DeepEqual(got, []string{"x.example.com", "y.example.com"}) {
		t.Errorf("Extract json = %v", got)
	}
	lit := `x.example.com\ny.example.com`
	if got := Extract(lit, "example.com"); len(got) != 2 {
		t.Errorf("Extract literal-escapes = %v", got)
	}
	if got := Extract("", "example.com"); got != nil {
		t.Errorf("empty = %v", got)
	}
}

func TestFindInText(t *testing.T) {
	html := `<a href="https://Dev.Example.com/x">dev</a> mail.example.com. see
	notexample.com and foo.example.com.evil.test and a.example.community and
	deep.sub.example.com, -lead.example.com, example.com itself, *.wild.example.com`
	want := []string{"dev.example.com", "mail.example.com", "deep.sub.example.com", "lead.example.com", "wild.example.com"}
	if got := FindInText(html, "example.com"); !reflect.DeepEqual(got, want) {
		t.Errorf("FindInText = %v; want %v", got, want)
	}
}
