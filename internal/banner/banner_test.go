package banner

import (
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lunalully/lunatic/internal/term"
)

// wantArt is the drawing exactly as supplied by the author.
var wantArt = []string{
	"___    A",
	"| |   {*}",
	"| |  __V__",
	"|_|o_|%%%|0_",
	"   |       |",
	"   |       |",
	"   |_______|",
}

func TestArtExact(t *testing.T) {
	got := ArtLines()
	if len(got) != len(wantArt) {
		t.Fatalf("art rows: got %d want %d", len(got), len(wantArt))
	}
	for i := range wantArt {
		if got[i] != wantArt[i] {
			t.Errorf("art row %d: got %q want %q", i, got[i], wantArt[i])
		}
	}
}

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
	// the drawing sits 4 columns right of the wordmark, bottom aligned
	off := len(Word) - len(wantArt)
	for j, a := range wantArt {
		l := lines[1+off+j]
		want := strings.TrimRight("  "+pad(Word[off+j], width(Word))+"    "+a, " ")
		if l != want {
			t.Errorf("row %d: got %q want %q", off+j, l, want)
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
