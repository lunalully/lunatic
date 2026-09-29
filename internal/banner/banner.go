// Package banner renders the Lunatic header: the "luNatiC" wordmark (ASCII
// art supplied by the author) with a small ASCII drawing to its right.
package banner

import (
	"io"
	"strings"

	"github.com/lunalully/lunatic/internal/term"
)

// Word is the wordmark, 13 rows by 65 columns (rows are right-trimmed).
// Transcribed by hand from the author's image; no FIGlet font was used.
var Word = []string{
	"                                                        _..._",
	".---.                                                .-'_..._''.",
	"|   |              _..._                     .--.  .' .'      '.\\",
	"|   |            .'     '.                   |__| / .'",
	"|   |           .   .-.   .              .|  .--.. '",
	"|   |           |  '   '  |    __      .' |_ |  || |",
	"|   |   _    _  |  |   |  | .:--.'.  .'     ||  || |",
	"|   |  | '  / | |  |   |  |/ |   \\ |'--.  .-'|  |. '",
	"|   | .' | .' | |  |   |  |`\" __ | |   |  |  |  | \\ '.          .",
	"|   | /  | /  | |  |   |  | .'.''| |   |  |  |__|  '. `._____.-'/",
	"'---'|   ''.  | |  |   |  |/ /   | |_  |  '.'        `-.______ /",
	"     '   .'|  '/|  |   |  |\\ \\._.\\ '/  |   /                  '",
	"      `-'  `--' '--'   '--' '--'  `\"   `'-'",
}

// Art is the drawing shown to the right of the wordmark, reproduced exactly
// as supplied by the author (7 rows, leading spaces are significant).
const Art = `___    A
| |   {*}
| |  __V__
|_|o_|%%%|0_
   |       |
   |       |
   |_______|`

const (
	gap     = 4
	tagline = "Passive Subdomain Recon  v"
)

// ArtLines returns the rows of Art.
func ArtLines() []string { return strings.Split(Art, "\n") }

// width in columns (runes) of the widest row.
func width(rows []string) int {
	w := 0
	for _, r := range rows {
		if n := len([]rune(r)); n > w {
			w = n
		}
	}
	return w
}

func pad(s string, w int) string {
	if n := len([]rune(s)); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// Render returns the banner text. lvl None yields plain text with no ANSI.
// The drawing is aligned to the bottom row of the wordmark; the tagline
// goes under the wordmark, after a blank line.
func Render(version string, lvl term.Level) string {
	art := ArtLines()
	ww := width(Word)
	var b strings.Builder
	b.WriteString("\n")
	emit := func(line string) {
		if line = strings.TrimRight(line, " "); line != "" {
			b.WriteString("  " + term.Paint(lvl, "orange", line))
		}
		b.WriteString("\n")
	}
	off := len(Word) - len(art)
	for i, w := range Word {
		line := pad(w, ww)
		if j := i - off; j >= 0 {
			line += strings.Repeat(" ", gap) + art[j]
		}
		emit(line)
	}
	emit("")
	emit(tagline + version)
	b.WriteString("\n")
	return b.String()
}

// Print writes the banner to w (callers pass stderr, only when it is a TTY
// and not --silent).
func Print(w io.Writer, version string, lvl term.Level) {
	io.WriteString(w, Render(version, lvl))
}
