package banner

import (
	"strings"
	"testing"

	"github.com/lunalully/lunatic/internal/term"
)

func TestRenderPlain(t *testing.T) {
	s := Render("1.2.3", term.None)
	if strings.Contains(s, "\x1b") {
		t.Fatal("ANSI in plain banner")
	}
	if !strings.Contains(s, "Passive Subdomain Recon") || !strings.Contains(s, "v1.2.3") {
		t.Fatal(s)
	}
	for _, l := range strings.Split(s, "\n") {
		if len(l) > 80 {
			t.Fatalf("line too wide (%d): %q", len(l), l)
		}
	}
	if len(Art) != 6 {
		t.Fatal("art rows")
	}
	if !strings.Contains(s, "* Passive Subdomain Recon *") {
		t.Fatal("tagline")
	}
	var n int
	for _, l := range Art {
		for _, c := range l {
			switch c {
			case '/', '\\', '*':
				n++
			case ' ':
			default:
				t.Fatalf("unexpected char %q in art", c)
			}
		}
	}
	if n == 0 {
		t.Fatal("empty art")
	}
	w := len(Art[0])
	for _, l := range Art {
		if len(l) != w {
			t.Errorf("ragged art row: %q", l)
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
