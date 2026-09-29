//go:build ignore

// star.go generates the star art embedded in banner.go.
//
//	go run internal/banner/gen/star.go [-r 6.3] [-w 0.55] [-t 0.5] [-h 12] [-c 25] [-sx 2.05]
//
// It builds a regular five-pointed star (pointing up, inner radius R*0.382),
// supersamples every terminal cell (4x4 subpoints, x scaled by -sx because
// cells are ~2:1 tall) and marks the cell when at least -t of its subpoints lie
// within -w row units of the outline. Only the left half (and the center
// column) is computed and mirrored, so the result is exactly symmetric. The
// marked cells are filled with the letters of "lunatic", cycled row-major.
package main

import (
	"flag"
	"fmt"
	"math"
	"strings"
)

type pt struct{ x, y float64 }

func segDist(p, a, b pt) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	t := ((p.x-a.x)*dx + (p.y-a.y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(p.x-(a.x+t*dx), p.y-(a.y+t*dy))
}

func main() {
	R := flag.Float64("r", 6.3, "outer radius in row units")
	hw := flag.Float64("w", 0.55, "stroke half-width in row units")
	th := flag.Float64("t", 0.5, "min fraction of subpoints inside the stroke")
	H := flag.Int("h", 12, "grid rows")
	C := flag.Int("c", 25, "grid columns (odd)")
	sx := flag.Float64("sx", 2.05, "cell height:width ratio")
	oy := flag.Float64("oy", 0, "vertical offset in rows")
	flag.Parse()

	inner := *R * 0.381966
	cx := float64(*C) / 2 / *sx
	// vertical centering: star spans -R .. inner_bottom (R*cos36)
	bottom := *R * math.Cos(math.Pi/5)
	cy := (float64(*H)-(*R+bottom))/2 + *R + *oy
	var v []pt
	for k := 0; k < 10; k++ {
		r := *R
		if k%2 == 1 {
			r = inner
		}
		a := -math.Pi/2 + float64(k)*math.Pi/5
		v = append(v, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	const sub = 4
	mark := make([][]bool, *H)
	mid := *C / 2
	for y := 0; y < *H; y++ {
		mark[y] = make([]bool, *C)
		for x := 0; x <= mid; x++ {
			in := 0
			for i := 0; i < sub; i++ {
				for j := 0; j < sub; j++ {
					p := pt{(float64(x) + (float64(i)+0.5)/sub) / *sx, float64(y) + (float64(j)+0.5)/sub}
					for k := range v {
						if segDist(p, v[k], v[(k+1)%10]) <= *hw {
							in++
							break
						}
					}
				}
			}
			if float64(in)/(sub*sub) >= *th {
				mark[y][x] = true
				mark[y][*C-1-x] = true
			}
		}
	}
	const word = "lunatic"
	n := 0
	var rows []string
	for y := 0; y < *H; y++ {
		var sb strings.Builder
		for x := 0; x < *C; x++ {
			if mark[y][x] {
				sb.WriteByte(word[n%len(word)])
				n++
			} else {
				sb.WriteByte(' ')
			}
		}
		rows = append(rows, strings.TrimRight(sb.String(), " "))
	}
	for len(rows) > 0 && rows[0] == "" {
		rows = rows[1:]
	}
	for len(rows) > 0 && rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}
	for _, r := range rows {
		fmt.Printf("\t%q,\n", r)
	}
	fmt.Println("// rows:", len(rows))
}
