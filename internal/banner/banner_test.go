package banner

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lunalully/lunatic/internal/term"
)

func TestWordShape(t *testing.T) {
	if len(Word) != 13 {
		t.Fatalf("word rows: %d", len(Word))
	}
	if w := width(Word); w != 65 {
		t.Fatalf("word width %d", w)
	}
	for _, r := range Word {
		for _, c := range r {
			if !strings.ContainsRune(" |.-'`/\\_,:\"", c) {
				t.Fatalf("unexpected char %q in word", c)
			}
		}
	}
}

func TestRenderGolden(t *testing.T) {
	want, err := os.ReadFile("testdata/banner.golden")
	if err != nil {
		t.Fatal(err)
	}
	got := Render("1.2.3", term.None)
	if got != string(want) {
		t.Fatalf("banner differs from testdata/banner.golden:\n%s", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Fatal("ANSI in plain banner")
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 1+len(Word)+2 { // blank + word + blank + tagline
		t.Fatalf("rows: %d", len(lines))
	}
	for _, l := range lines {
		if n := utf8.RuneCountInString(l); n > 100 {
			t.Fatalf("line too wide (%d): %q", n, l)
		}
	}
	if !strings.Contains(got, "Passive Subdomain Recon  v1.2.3") {
		t.Fatal("tagline missing")
	}
	for _, l := range lines {
		if l != strings.TrimRight(l, " ") {
			t.Errorf("trailing spaces: %q", l)
		}
	}
}

func TestRenderColors(t *testing.T) {
	if !strings.Contains(Render("1", term.Color256), "38;5;208") {
		t.Fatal("256 orange missing")
	}
	b := Render("1", term.Basic)
	if !strings.Contains(b, "\x1b[33m") || strings.Contains(b, "38;5;208") {
		t.Fatal("basic fallback")
	}
}

func TestColorLevel(t *testing.T) {
	env := func(m map[string]string) func(string) (string, bool) {
		return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
	}
	cases := []struct {
		name string
		env  map[string]string
		tty  bool
		flag bool
		want term.Level
	}{
		{"256", map[string]string{"TERM": "xterm-256color"}, true, false, term.Color256},
		{"basic", map[string]string{"TERM": "xterm"}, true, false, term.Basic},
		{"colorterm", map[string]string{"TERM": "xterm", "COLORTERM": "truecolor"}, true, false, term.Color256},
		{"flag", map[string]string{"TERM": "xterm-256color"}, true, true, term.None},
		{"no_color", map[string]string{"TERM": "xterm-256color", "NO_COLOR": ""}, true, false, term.None},
		{"dumb", map[string]string{"TERM": "dumb"}, true, false, term.None},
		{"notty", map[string]string{"TERM": "xterm-256color"}, false, false, term.None},
	}
	for _, c := range cases {
		if got := term.ColorLevel(env(c.env), c.tty, c.flag); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
