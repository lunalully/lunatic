// Package banner renders the Lunatic header: the "luNatiC" wordmark (ASCII
// art supplied by the author) and the tagline below it.
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

const (
	tagline = "Passive Subdomain Recon  v"
)

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

// Render returns the banner text. lvl None yields plain text with no ANSI.
// The tagline goes under the wordmark, after a blank line.
func Render(version string, lvl term.Level) string {
	var b strings.Builder
	b.WriteString("\n")
	emit := func(line string) {
		if line = strings.TrimRight(line, " "); line != "" {
			b.WriteString("  " + term.Paint(lvl, "orange", line))
		}
		b.WriteString("\n")
	}
	for _, w := range Word {
		emit(w)
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
