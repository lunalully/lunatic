package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func write(t *testing.T, mode os.FileMode, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEnvName(t *testing.T) {
	if g := EnvName("shodan", "api_key"); g != "LUNATIC_SHODAN_API_KEY" {
		t.Fatal(g)
	}
	if g := EnvName("dns-db", "api.id"); g != "LUNATIC_DNS_DB_API_ID" {
		t.Fatal(g)
	}
}

func TestLoadAndPrecedence(t *testing.T) {
	p := write(t, 0o600, `# comment
sources:
  shodan:
    api_key: "file-key"   # trailing
  censys:
    api_id: 'id1'
    api_secret: YOUR_KEY_HERE
  empty:
    api_key: ""
  fofa:
    api_key: <put key>
other: 1
`)
	cfg, warns, err := Load(p, env(map[string]string{"LUNATIC_CENSYS_API_SECRET": "envsecret", "LUNATIC_FOFA_API_KEY": "fofaenv"}))
	if err != nil || len(warns) != 0 {
		t.Fatal(err, warns)
	}
	if got := cfg.Creds("shodan", []string{"api_key"}); !reflect.DeepEqual(got, map[string]string{"api_key": "file-key"}) {
		t.Fatal(got)
	}
	if got := cfg.Creds("censys", []string{"api_id", "api_secret"}); !reflect.DeepEqual(got, map[string]string{"api_id": "id1", "api_secret": "envsecret"}) {
		t.Fatal(got)
	}
	if got := cfg.Creds("empty", []string{"api_key"}); len(got) != 0 {
		t.Fatal(got)
	}
	if got := cfg.Creds("fofa", []string{"api_key"}); got["api_key"] != "fofaenv" {
		t.Fatal(got)
	}
	if got := cfg.Creds("nothing", []string{"api_key"}); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestEnvPlaceholderFallsBackToFile(t *testing.T) {
	p := write(t, 0o600, "sources:\n  a:\n    api_key: real\n")
	cfg, _, _ := Load(p, env(map[string]string{"LUNATIC_A_API_KEY": "YOUR_KEY_HERE"}))
	if cfg.Creds("a", []string{"api_key"})["api_key"] != "real" {
		t.Fatal("placeholder env should be ignored")
	}
}

func TestMissingFiles(t *testing.T) {
	if _, _, err := Load(filepath.Join(t.TempDir(), "nope.yaml"), env(nil)); err == nil {
		t.Fatal("explicit missing file must error")
	}
	x := t.TempDir()
	cfg, w, err := Load("", env(map[string]string{"XDG_CONFIG_HOME": x}))
	if err != nil || cfg == nil || len(w) != 0 {
		t.Fatal(err)
	}
}

func TestDefaultPath(t *testing.T) {
	if g := DefaultPath(env(map[string]string{"XDG_CONFIG_HOME": "/x"})); g != "/x/lunatic/config.yaml" {
		t.Fatal(g)
	}
	x := t.TempDir()
	os.MkdirAll(filepath.Join(x, "lunatic"), 0o755)
	os.WriteFile(filepath.Join(x, "lunatic", "config.yaml"), []byte("sources:\n  a:\n    k: v\n"), 0o600)
	cfg, _, err := Load("", env(map[string]string{"XDG_CONFIG_HOME": x}))
	if err != nil || cfg.Sources["a"]["k"] != "v" {
		t.Fatal(err, cfg)
	}
}

func TestWorldReadableWarning(t *testing.T) {
	p := write(t, 0o644, "sources:\n  a:\n    api_key: SECRETVALUE\n")
	_, warns, err := Load(p, env(nil))
	if err != nil || len(warns) != 1 || strings.Contains(warns[0], "SECRETVALUE") {
		t.Fatal(err, warns)
	}
}

func TestBadYAML(t *testing.T) {
	for _, c := range []string{"sources:\n\ta: b\n", "sources\n", "sources:\n  a:\n    k: \"unterminated\n"} {
		p := write(t, 0o600, c)
		if _, _, err := Load(p, env(nil)); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	for _, v := range []string{"", "  ", "YOUR_KEY_HERE", "your_api_key", "<key>", "changeme"} {
		if !IsPlaceholder(v) {
			t.Errorf("%q should be placeholder", v)
		}
	}
	if IsPlaceholder("abc123") {
		t.Error("real value flagged")
	}
}

func TestCredsIncludeOptionalFields(t *testing.T) {
	env := map[string]string{"LUNATIC_FOFA_EMAIL": "e@x.org"}
	cfg := &Config{Sources: map[string]map[string]string{"fofa": {"key": "k"}}, getenv: func(k string) string { return env[k] }}
	got := cfg.Creds("fofa", []string{"key", "email"})
	if got["key"] != "k" || got["email"] != "e@x.org" {
		t.Fatalf("got %v", got)
	}
}

func TestParseSubset(t *testing.T) {
	text := "# top comment\r\n" +
		"version: 1   # ignored top-level key\r\n" +
		"other:\r\n  nested: value\r\n" +
		"sources:\r\n" +
		"  shodan:\r\n" +
		"    api_key: abc123   # trailing comment\r\n" +
		"  censys:\r\n" +
		"    api_id: \"id # not a comment\"\r\n" +
		"    api_secret: 'it''s'\r\n" +
		"  empty:\r\n" +
		"    api_key:\r\n" +
		"  explicit_empty:\r\n" +
		"    api_key: \"\"\r\n" +
		"    other: ~\r\n" +
		"\r\n" +
		"  # comment between\r\n" +
		"  virustotal:\r\n" +
		"    api_key: 'k:with:colons'\r\n"
	got, err := parse(text)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{
		"shodan":         {"api_key": "abc123"},
		"censys":         {"api_id": "id # not a comment", "api_secret": "it's"},
		"empty":          {},
		"explicit_empty": {"api_key": ""},
		"virustotal":     {"api_key": "k:with:colons"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestParseSourcesNullAndMissing(t *testing.T) {
	for _, c := range []string{"", "# only comments\n", "sources: {}\n", "sources: ~\n", "sources:\n", "---\nsources:\n"} {
		got, err := parse(c)
		if err != nil || len(got) != 0 {
			t.Errorf("%q -> %v %v", c, got, err)
		}
	}
}

func TestParseErrorsHaveLineNumbers(t *testing.T) {
	cases := map[string]string{
		"sources:\n  a:\n    k: v\n  b\n":         "line 4",
		"# c\n\nsources:\n\ta: b\n":               "line 4",
		"sources:\n  a:\n    k: \"unterminated\n": "line 3",
		"sources:\n  a:\n    k: 'unterminated\n":  "line 3",
	}
	for in, want := range cases {
		_, err := parse(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err=%v, want %q", in, err, want)
		}
	}
	// Load wraps with the file path too.
	p := write(t, 0o600, "sources:\n  a\n")
	_, _, err := Load(p, env(nil))
	if err == nil || !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err=%v", err)
	}
}
