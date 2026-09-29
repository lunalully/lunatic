package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lunalully/lunatic/internal/sources"
)

type fake struct {
	info sources.Info
	fn   func(ctx context.Context, d string, s *sources.Session, emit func(string)) error
	runs *int32
}

func (f *fake) Info() sources.Info { return f.info }
func (f *fake) Enumerate(ctx context.Context, d string, s *sources.Session, emit func(string)) error {
	if f.runs != nil {
		atomic.AddInt32(f.runs, 1)
	}
	return f.fn(ctx, d, s, emit)
}

func emitting(d string, names ...string) func(context.Context, string, *sources.Session, func(string)) error {
	return func(_ context.Context, dom string, _ *sources.Session, emit func(string)) error {
		for _, n := range names {
			emit(n + "." + dom)
		}
		return nil
	}
}

const secret = "TOPSECRETVALUE123"

func registry(runs *int32) []sources.Source {
	return []sources.Source{
		&fake{info: sources.Info{Name: "alpha", Default: true}, fn: emitting("", "a", "b"), runs: runs},
		&fake{info: sources.Info{Name: "beta", Default: true}, fn: emitting("", "b"), runs: runs},
		&fake{info: sources.Info{Name: "boom"}, runs: runs, fn: func(context.Context, string, *sources.Session, func(string)) error {
			return sources.ErrRateLimited
		}},
		&fake{info: sources.Info{Name: "keyed", Auth: sources.AuthRequired, CredFields: []string{"api_key"}, Default: true}, runs: runs,
			fn: func(_ context.Context, d string, s *sources.Session, emit func(string)) error {
				k, err := sources.PickKey(s.Creds, "api_key")
				if err != nil {
					return err
				}
				if k == secret {
					emit("k." + d)
				}
				return fmt.Errorf("upstream rejected key=%s", k)
			}},
		&fake{info: sources.Info{Name: "off", Disabled: true, DisabledReason: "service shut down", Default: true}, runs: runs, fn: emitting("", "never")},
	}
}

type result struct {
	code           int
	stdout, stderr string
}

func do(t *testing.T, e env, envs map[string]string, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	if e.sources == nil {
		e.sources = registry(nil)
	}
	e.lookupEnv = func(k string) (string, bool) { v, ok := envs[k]; return v, ok }
	// Isolate from any real user config.
	if _, ok := envs["XDG_CONFIG_HOME"]; !ok {
		envs["XDG_CONFIG_HOME"] = t.TempDir()
	}
	code := run(context.Background(), args, &out, &errb, e)
	return result{code, out.String(), errb.String()}
}

func env0() map[string]string { return map[string]string{} }

func TestDefaultRun(t *testing.T) {
	var runs int32
	r := do(t, env{sources: registry(&runs)}, env0(), "-d", "example.com")
	if r.code != 0 || r.stdout != "a.example.com\nb.example.com\n" {
		t.Fatalf("%+v", r)
	}
	if runs != 2 {
		t.Fatalf("default run should only use alpha+beta, ran %d", runs)
	}
	for _, want := range []string{"alpha", "ok 2", "beta", "ok 1", "2 unique subdomain"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("summary missing %q:\n%s", want, r.stderr)
		}
	}
	if strings.Contains(r.stderr, "\x1b") || strings.Contains(r.stderr, "Passive Subdomain Recon") {
		t.Fatal("non-TTY stderr must be plain without banner")
	}
}

func TestSilent(t *testing.T) {
	r := do(t, env{}, env0(), "-d", "example.com", "--silent")
	if r.code != 0 || r.stderr != "" || r.stdout != "a.example.com\nb.example.com\n" {
		t.Fatalf("%+v", r)
	}
	// Fatal errors still reach stderr.
	r = do(t, env{}, env0(), "--silent", "-d", "http://x.com")
	if r.code != 1 || r.stderr == "" {
		t.Fatalf("%+v", r)
	}
}

func TestExitCodes(t *testing.T) {
	cases := []struct {
		args []string
		code int
	}{
		{[]string{"-d", "example.com", "-s", "alpha"}, 0},
		{[]string{"-d", "example.com", "-s", "alpha,boom"}, 3},
		{[]string{"-d", "example.com", "-s", "boom"}, 2},
		{[]string{"-d", "example.com", "-s", "keyed"}, 2},
		{[]string{"-d", "example.com", "-s", "off"}, 2},
		{[]string{"-d", "example.com", "-s", "alpha,keyed"}, 0}, // skipped is not a failure
		{[]string{"-d", "example.com", "-es", "alpha,beta"}, 2}, // nothing usable left
		{[]string{"-d", "example.com", "-s", "nope"}, 1},
		{[]string{"-d", "example.com", "--exclude-sources", "nope"}, 1},
		{[]string{"-d", "example.com", "--all", "-s", "alpha"}, 1},
		{[]string{}, 1},
		{[]string{"-d", "example.com", "extra"}, 1},
		{[]string{"-d", "example.com", "--bogus"}, 1},
		{[]string{"-d", "example.com", "--timeout", "0"}, 1},
	}
	for _, c := range cases {
		if r := do(t, env{}, env0(), c.args...); r.code != c.code {
			t.Errorf("%v: code %d want %d\nstderr: %s", c.args, r.code, c.code, r.stderr)
		}
	}
}

func TestUnknownSourceListsValid(t *testing.T) {
	r := do(t, env{}, env0(), "-d", "example.com", "-s", "nope")
	if !strings.Contains(r.stderr, "nope") || !strings.Contains(r.stderr, "alpha, beta, boom, keyed, off") || r.stdout != "" {
		t.Fatalf("%+v", r)
	}
}

func TestSkippedReasons(t *testing.T) {
	r := do(t, env{}, env0(), "-d", "example.com", "-s", "keyed,off,alpha")
	if !strings.Contains(r.stderr, "missing credentials: LUNATIC_KEYED_API_KEY") || !strings.Contains(r.stderr, "service shut down") {
		t.Fatal(r.stderr)
	}
}

func TestAllReportsMissingKeys(t *testing.T) {
	var runs int32
	r := do(t, env{sources: registry(&runs)}, env0(), "-d", "example.com", "--all")
	// alpha, beta ok; boom failed; keyed skipped; off excluded entirely.
	if r.code != 3 || strings.Contains(r.stderr, "off ") || !strings.Contains(r.stderr, "missing credentials") || !strings.Contains(r.stderr, "boom") {
		t.Fatalf("%+v", r)
	}
	if runs != 3 {
		t.Fatalf("runs=%d", runs)
	}
}

func TestExcludeSources(t *testing.T) {
	r := do(t, env{}, env0(), "-d", "example.com", "-es", "alpha")
	if r.stdout != "b.example.com\n" {
		t.Fatal(r.stdout)
	}
}

func TestInvalidDomains(t *testing.T) {
	for _, d := range []string{"", "https://example.com", "1.2.3.4", "example", "*.example.com", "example.com:80", "a b.com"} {
		var runs int32
		r := do(t, env{sources: registry(&runs)}, env0(), "-d", d)
		if r.code != 1 || r.stdout != "" || runs != 0 {
			t.Errorf("-d %q: %+v runs=%d", d, r, runs)
		}
	}
}

func TestDomainList(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	os.WriteFile(good, []byte("# targets\nexample.com\n\n  Example.ORG.  # trailing\nexample.com\n"), 0o644)
	r := do(t, env{}, env0(), "-dL", good, "-s", "alpha")
	if r.code != 0 || r.stdout != "a.example.com\nb.example.com\na.example.org\nb.example.org\n" {
		t.Fatalf("%+v", r)
	}
	bad := filepath.Join(dir, "bad.txt")
	os.WriteFile(bad, []byte("example.com\nhttp://evil.com\n\nnot valid\n"), 0o644)
	var runs int32
	r = do(t, env{sources: registry(&runs)}, env0(), "-dL", bad)
	if r.code != 1 || runs != 0 || !strings.Contains(r.stderr, "bad.txt:2") || !strings.Contains(r.stderr, "bad.txt:4") {
		t.Fatalf("%+v runs=%d", r, runs)
	}
	if r := do(t, env{}, env0(), "-dL", filepath.Join(dir, "missing")); r.code != 1 {
		t.Fatalf("%+v", r)
	}
	// -d and -dL combine.
	r = do(t, env{}, env0(), "-dL", good, "-d", "third.io", "-s", "beta")
	if !strings.Contains(r.stdout, "b.third.io") || r.code != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestJSON(t *testing.T) {
	r := do(t, env{}, env0(), "-d", "example.com", "--json")
	want := `{"domain":"example.com","subdomain":"a.example.com","sources":["alpha"]}` + "\n" +
		`{"domain":"example.com","subdomain":"b.example.com","sources":["alpha","beta"]}` + "\n"
	if r.stdout != want {
		t.Fatalf("%q", r.stdout)
	}
	if r2 := do(t, env{}, env0(), "-d", "example.com", "-oJ"); r2.stdout != want {
		t.Fatal("-oJ alias")
	}
}

func TestOutputFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out.txt")
	os.WriteFile(p, []byte("stale\n"), 0o600)
	r := do(t, env{}, env0(), "-d", "example.com", "-o", p)
	data, _ := os.ReadFile(p)
	if string(data) != "a.example.com\nb.example.com\n" || r.stdout != string(data) {
		t.Fatalf("file=%q stdout=%q", data, r.stdout)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 && fi.Mode().Perm() != 0o644 {
		t.Fatal(fi.Mode())
	}
	if r := do(t, env{}, env0(), "-d", "example.com", "-o", filepath.Join(t.TempDir(), "no", "such", "dir", "f")); r.code != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestSecretsNeverPrinted(t *testing.T) {
	envs := map[string]string{"LUNATIC_KEYED_API_KEY": secret}
	for _, args := range [][]string{
		{"--list-sources"},
		{"-d", "example.com", "-s", "keyed", "-v"},
		{"-d", "example.com", "-s", "keyed", "--json"},
		{"-d", "example.com", "--all"},
	} {
		r := do(t, env{}, envs, args...)
		if strings.Contains(r.stdout+r.stderr, secret) {
			t.Fatalf("%v leaked secret:\nstdout:%s\nstderr:%s", args, r.stdout, r.stderr)
		}
	}
	r := do(t, env{}, envs, "-d", "example.com", "-s", "keyed")
	if r.code != 2 || r.stdout != "k.example.com\n" || !strings.Contains(r.stderr, "[REDACTED]") {
		t.Fatalf("%+v", r)
	}
	// Secret from a config file is also never printed.
	cf := filepath.Join(t.TempDir(), "c.yaml")
	os.WriteFile(cf, []byte("sources:\n  keyed:\n    api_key: "+secret+"\n"), 0o600)
	r = do(t, env{}, env0(), "--config", cf, "--list-sources")
	if strings.Contains(r.stdout+r.stderr, secret) || !strings.Contains(r.stdout, "ready") {
		t.Fatalf("%+v", r)
	}
}

func TestListSources(t *testing.T) {
	r := do(t, env{}, env0(), "--list-sources")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("%+v", r)
	}
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 6 || !strings.HasPrefix(lines[0], "NAME") {
		t.Fatalf("%q", r.stdout)
	}
	find := func(name string) string {
		for _, l := range lines {
			if strings.HasPrefix(l, name+" ") {
				return l
			}
		}
		return ""
	}
	if l := find("keyed"); !strings.Contains(l, "required") || !strings.Contains(l, "needs key") || !strings.Contains(l, "LUNATIC_KEYED_API_KEY") {
		t.Errorf("keyed: %q", l)
	}
	if l := find("off"); !strings.Contains(l, "disabled") || !strings.Contains(l, "service shut down") {
		t.Errorf("off: %q", l)
	}
	if l := find("alpha"); !strings.Contains(l, "none") || !strings.Contains(l, "ready") || !strings.Contains(l, "yes") {
		t.Errorf("alpha: %q", l)
	}
	if r := do(t, env{}, map[string]string{"LUNATIC_KEYED_API_KEY": "x"}, "--list-sources"); !strings.Contains(r.stdout, "ready") {
		t.Fatal("key present should be ready")
	}
}

func TestColorAndBanner(t *testing.T) {
	tty := map[string]string{"TERM": "xterm-256color"}
	r := do(t, env{stderrTTY: true}, tty, "-d", "example.com")
	if !strings.Contains(r.stderr, "Passive Subdomain Recon") || !strings.Contains(r.stderr, "38;5;208") {
		t.Fatalf("banner/color missing:\n%s", r.stderr)
	}
	if strings.Contains(r.stdout, "\x1b") {
		t.Fatal("ANSI on stdout")
	}
	basic := do(t, env{stderrTTY: true}, map[string]string{"TERM": "xterm"}, "-d", "example.com")
	if !strings.Contains(basic.stderr, "\x1b[33m") || strings.Contains(basic.stderr, "38;5;208") {
		t.Fatal("basic fallback")
	}
	for name, c := range map[string]struct {
		envs map[string]string
		args []string
	}{
		"flag":     {map[string]string{"TERM": "xterm-256color"}, []string{"--no-color"}},
		"NO_COLOR": {map[string]string{"TERM": "xterm-256color", "NO_COLOR": ""}, nil},
		"dumb":     {map[string]string{"TERM": "dumb"}, nil},
	} {
		r := do(t, env{stderrTTY: true}, c.envs, append([]string{"-d", "example.com"}, c.args...)...)
		if strings.Contains(r.stderr, "\x1b") || !strings.Contains(r.stderr, "Passive Subdomain Recon") {
			t.Errorf("%s: %q", name, r.stderr)
		}
	}
	r = do(t, env{stderrTTY: true}, tty, "-d", "example.com", "--silent")
	if r.stderr != "" {
		t.Fatalf("silent TTY: %q", r.stderr)
	}
	r = do(t, env{stderrTTY: true}, tty, "--list-sources")
	if r.stderr != "" || strings.Contains(r.stdout, "\x1b") {
		t.Fatal("list-sources must not print banner or ANSI")
	}
}

func TestVersionAndHelp(t *testing.T) {
	r := do(t, env{}, env0(), "--version")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "lunatic ") {
		t.Fatalf("%+v", r)
	}
	for _, a := range []string{"-h", "--help"} {
		r := do(t, env{}, env0(), a)
		if r.code != 0 || !strings.Contains(r.stdout, "-dL") || !strings.Contains(r.stdout, "--exclude-sources") || !strings.Contains(r.stdout, "default 90") {
			t.Fatalf("%s: %+v", a, r)
		}
	}
}

func TestConfig(t *testing.T) {
	if r := do(t, env{}, env0(), "-d", "example.com", "--config", "/nonexistent/x.yaml"); r.code != 1 {
		t.Fatalf("%+v", r)
	}
	cf := filepath.Join(t.TempDir(), "c.yaml")
	os.WriteFile(cf, []byte("sources:\n  keyed:\n    api_key: "+secret+"\n"), 0o644)
	r := do(t, env{}, env0(), "-d", "example.com", "--config", cf, "-s", "keyed")
	if r.stdout != "k.example.com\n" || !strings.Contains(r.stderr, "world-readable") || strings.Contains(r.stderr, secret) {
		t.Fatalf("%+v", r)
	}
	r = do(t, env{}, env0(), "-d", "example.com", "--config", cf, "-s", "keyed", "--silent")
	if strings.Contains(r.stderr, "world-readable") {
		t.Fatal("silent must suppress warnings")
	}
}

func TestInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	blocker := &fake{info: sources.Info{Name: "blocker", Default: true}, fn: func(ctx context.Context, d string, _ *sources.Session, emit func(string)) error {
		emit("x." + d)
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}}
	var out, errb bytes.Buffer
	code := run(ctx, []string{"-d", "example.com"}, &out, &errb, env{sources: []sources.Source{blocker}, lookupEnv: func(string) (string, bool) { return "", false }})
	if code != 130 || out.String() != "x.example.com\n" {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

func TestNoUsableSources(t *testing.T) {
	r := do(t, env{sources: []sources.Source{}}, env0(), "-d", "example.com")
	if r.code != 2 || !strings.Contains(r.stderr, "no usable sources") {
		t.Fatalf("%+v", r)
	}
}

func TestListSourcesOptionalEnv(t *testing.T) {
	srcs := []sources.Source{
		&fake{info: sources.Info{Name: "fofa", Auth: sources.AuthRequired, CredFields: []string{"key"}, OptCredFields: []string{"email"}, Default: true}},
	}
	r := do(t, env{sources: srcs}, env0(), "--list-sources")
	if !strings.Contains(r.stdout, "LUNATIC_FOFA_KEY") || !strings.Contains(r.stdout, "optional env: LUNATIC_FOFA_EMAIL") {
		t.Fatalf("%q", r.stdout)
	}
	// Only the optional field configured: source is still not ready.
	e := env0()
	e["LUNATIC_FOFA_EMAIL"] = "me@example.org"
	r = do(t, env{sources: srcs}, e, "--list-sources")
	if !strings.Contains(r.stdout, "needs key") || strings.Contains(r.stdout, "me@example.org") {
		t.Fatalf("%q", r.stdout)
	}
	// With the key set (optional unset): ready.
	e = env0()
	e["LUNATIC_FOFA_KEY"] = "k"
	r = do(t, env{sources: srcs}, e, "--list-sources")
	if !strings.Contains(r.stdout, "ready") {
		t.Fatalf("%q", r.stdout)
	}
}
