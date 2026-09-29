// Package banner renders the Lunatic ASCII header.
package banner

import (
	"io"
	"strings"

	"github.com/lunalully/lunatic/internal/term"
)

// Art spells LUNATIC in slanted block letters drawn with '/' strokes, '*'
// horizontals and '\\' for the diagonal of N. 6 rows, 55 columns.
var Art = []string{
	"     //     //  // //    //   ****   ****** ****  *****",
	"    //     //  // //\\   //  //  //    //    //  //     ",
	"   //     //  // // \\  // //    //   //    //  //      ",
	"  //     //  // //  \\ // //****//   //    //  //       ",
	" //     //  // //   \\// //    //   //    //  //        ",
	"//****  ****  //    // //    //   //   ****  *****     ",
}

// Render returns the banner text. lvl None yields plain text with no ANSI.
func Render(version string, lvl term.Level) string {
	var b strings.Builder
	b.WriteString("\n")
	for _, l := range Art {
		b.WriteString("  " + term.Paint(lvl, "orange", l) + "\n")
	}
	b.WriteString("\n  " + term.Paint(lvl, "bold", "* Passive Subdomain Recon *") + "  " + term.Paint(lvl, "dim", "v"+version) + "\n\n")
	return b.String()
}

// Print writes the banner to w (callers pass stderr, only when it is a TTY
// and not --silent).
func Print(w io.Writer, version string, lvl term.Level) {
	io.WriteString(w, Render(version, lvl))
}
