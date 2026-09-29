// Package term has stdlib-only terminal helpers: TTY detection and ANSI color.
package term

import (
	"io"
	"os"
	"strings"
)

// Level is the color capability.
type Level int

const (
	None  Level = iota // no ANSI at all
	Basic              // 16 colors
	Color256
)

// IsTerminal reports whether w is a character device (a TTY).
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// ColorLevel decides the color level for stderr-style output. There is no
// color when noColorFlag is set, NO_COLOR is set (any value, even empty),
// TERM is dumb or unset, or the stream is not a TTY. lookup is os.LookupEnv
// in production.
func ColorLevel(lookup func(string) (string, bool), isTTY, noColorFlag bool) Level {
	if noColorFlag || !isTTY {
		return None
	}
	if _, set := lookup("NO_COLOR"); set {
		return None
	}
	t, _ := lookup("TERM")
	if t == "" || t == "dumb" {
		return None
	}
	ct, _ := lookup("COLORTERM")
	if strings.Contains(t, "256color") || strings.Contains(t, "truecolor") || ct == "truecolor" || ct == "24bit" {
		return Color256
	}
	return Basic
}

// Paint wraps s in an SGR color for lvl. name is one of orange, green, red,
// yellow, dim, bold, cyan. With None it returns s unchanged.
func Paint(lvl Level, name, s string) string {
	if lvl == None {
		return s
	}
	var code string
	switch name {
	case "orange":
		code = "38;5;208"
		if lvl == Basic {
			code = "33"
		}
	case "green":
		code = "32"
	case "red":
		code = "31"
	case "yellow":
		code = "33"
	case "cyan":
		code = "36"
	case "dim":
		code = "2"
	case "bold":
		code = "1"
	default:
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
