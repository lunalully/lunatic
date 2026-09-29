//go:build ignore

// star.go generates the star art embedded in banner.go.
//
//	go run internal/banner/gen/star.go [-r 8.2] [-t 0.75] [-h 16] [-w 32]
//
// It computes the 10 vertices of a regular five-pointed star (pointing up),
// marks the grid cells whose center lies within -t units of any edge of the
// outline polygon (x is scaled by 2 because terminal cells are ~2:1 tall) and
// fills the marked cells with the letters of "lunatic", cycled in order.
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
	R := flag.Float64("r", 8.2, "outer radius in row units")
	th := flag.Float64("t", 0.75, "max distance from an edge, in row units")
	H := flag.Int("h", 16, "grid rows")
	W := flag.Int("w", 32, "grid columns")
	flag.Parse()

	inner := *R * 0.381966 // regular pentagram
	cx, cy := float64(*W)/4, float64(*H)/2+0.05*float64(*H)/2
	var v []pt
	for k := 0; k < 10; k++ {
		r := *R
		if k%2 == 1 {
			r = inner
		}
		a := -math.Pi/2 + float64(k)*math.Pi/5
		v = append(v, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	const word = "lunatic"
	n := 0
	var rows []string
	for y := 0; y < *H; y++ {
		var sb strings.Builder
		for x := 0; x < *W; x++ {
			p := pt{(float64(x) + 0.5) / 2, float64(y) + 0.5}
			hit := false
			for i := range v {
				if segDist(p, v[i], v[(i+1)%10]) <= *th {
					hit = true
					break
				}
			}
			if hit {
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
