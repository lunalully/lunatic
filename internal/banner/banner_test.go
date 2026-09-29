package banner

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lunalully/lunatic/internal/term"
)

func TestRenderPlain(t *testing.T) {
	s := Render("1.2.3", term.None)
	if strings.Contains(s, "\x1b") {
		t.Fatal("ANSI in plain banner")
	}
	if !strings.Contains(s, "Passive Subdomain Recon  v1.2.3") {
		t.Fatal(s)
	}
	lines := strings.Split(strings.Trim(s, "\n"), "\n")
	if len(lines) != len(Star) {
		t.Fatalf("rows: got %d want %d", len(lines), len(Star))
	}
	for _, l := range strings.Split(s, "\n") {
		if n := utf8.RuneCountInString(l); n > 80 {
			t.Fatalf("line too wide (%d): %q", n, l)
		}
	}
}

func TestWordPixels(t *testing.T) {
	if len(Word) != 7 {
		t.Fatalf("word rows: %d", len(Word))
	}
	for _, r := range Word {
		if n := utf8.RuneCountInString(r); n != 7*5+6 {
			t.Fatalf("word width %d: %q", n, r)
		}
		for _, c := range r {
			if c != ' ' && c != '\u2593' {
				t.Fatalf("unexpected char %q in word", c)
			}
		}
	}
	if !strings.Contains(strings.Join(Word, ""), "\u2593") {
		t.Fatal("empty word")
	}
}

func TestStarOnlyLunaticLetters(t *testing.T) {
	if len(Star) < 11 || len(Star) > 12 {
		t.Fatalf("star rows: %d", len(Star))
	}
	var seq []rune
	for _, r := range Star {
		if utf8.RuneCountInString(r) > 27 {
			t.Fatalf("star too wide: %q", r)
		}
		for _, c := range r {
			if c != ' ' && !strings.ContainsRune("lunatic", c) {
				t.Fatalf("unexpected char %q in star", c)
			}
			if c != ' ' {
				seq = append(seq, c)
			}
		}
	}
	for i, c := range seq {
		if c != rune("lunatic"[i%7]) {
			t.Fatalf("letters not cycled in order at %d: %q", i, c)
		}
	}
}

func TestStarSymmetric(t *testing.T) {
	const w = 27
	for i, r := range Star {
		rs := []rune(r)
		for len(rs) < w {
			rs = append(rs, ' ')
		}
		for x := 0; x < w/2; x++ {
			if (rs[x] == ' ') != (rs[w-1-x] == ' ') {
				t.Fatalf("row %d not symmetric: %q", i, r)
			}
		}
	}
}

func TestRenderAllowedChars(t *testing.T) {
	s := Render("1.2.3", term.None)
	s = strings.Replace(s, "Passive Subdomain Recon  v1.2.3", "", 1)
	for _, c := range s {
		if c != '\n' && c != ' ' && c != '\u2593' && !strings.ContainsRune("lunatic", c) {
			t.Fatalf("unexpected char %q in banner", c)
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
