//go:build ignore

// star.go generates the star art embedded in banner.go.
//
//	go run internal/banner/gen/star.go [-c 31] [-h 13] [-w 2] [-oy 0]
//
// It computes the 10 vertices of an upright golden star (inner radius
// 0.382*R), scaling x by 2.05 because terminal cells are ~2:1 tall, and
// rasterizes the 10 edges as lines: for every row, the exact x where each
// slanted edge crosses the row centre is marked (round(x) plus one neighbour
// towards the star's inside, so the stroke is -w columns wide; single-cell
// tips). Near-horizontal edges (the arm tops) mark their whole run. Only the
// left half is computed and mirrored, so the result is exactly symmetric.
// Marked cells are filled with the letters of "lunatic", cycled row-major.
package main

import (
	"flag"
	"fmt"
	"math"
	"strings"
)

type pt struct{ x, y float64 }

func main() {
	C := flag.Int("c", 31, "grid columns (odd)")
	H := flag.Int("h", 13, "grid rows")
	w := flag.Int("w", 2, "stroke width in columns (1 or 2)")
	oy := flag.Float64("oy", 0, "row sampling offset")
	flag.Parse()
	const sx = 2.05
	// Height of the star = R + R*cos36 rows (in row units, top tip to bottom tips).
	R := (float64(*H) - 0.3) / (1 + math.Cos(math.Pi/5))
	inner := R * 0.381966
	cx := float64(*C-1) / 2
	var v []pt
	for k := 0; k < 10; k++ {
		r := R
		if k%2 == 1 {
			r = inner
		}
		a := -math.Pi/2 + float64(k)*math.Pi/5
		v = append(v, pt{cx + sx*r*math.Cos(a), R + r*math.Sin(a)})
	}
	mid := *C / 2
	mark := make([][]bool, *H)
	for y := range mark {
		mark[y] = make([]bool, *C)
	}
	set := func(y, x int) {
		if y < 0 || y >= *H || x < 0 || x >= *C {
			return
		}
		if x > mid {
			x = *C - 1 - x
		}
		mark[y][x] = true
		mark[y][*C-1-x] = true
	}
	for k := range v {
		a, b := v[k], v[(k+1)%10]
		if a.y > b.y {
			a, b = b, a
		}
		if b.y-a.y < 0.5 { // near-horizontal: whole run
			y := int(math.Round((a.y + b.y) / 2))
			lo, hi := math.Min(a.x, b.x), math.Max(a.x, b.x)
			for x := int(math.Round(lo)); x <= int(math.Round(hi)); x++ {
				set(y, x)
			}
			continue
		}
		at := func(y float64) float64 { return a.x + (y-a.y)/(b.y-a.y)*(b.x-a.x) }
		for y := 0; y < *H; y++ {
			// part of the edge inside this row's band [y-.5, y+.5]
			y0 := math.Max(a.y, float64(y)-0.5+*oy)
			y1 := math.Min(b.y, float64(y)+0.5+*oy)
			if y0 > y1 {
				continue
			}
			x0, x1 := at(y0), at(y1)
			lo, hi := int(math.Round(math.Min(x0, x1))), int(math.Round(math.Max(x0, x1)))
			for x := lo; x <= hi; x++ {
				set(y, x)
			}
			// widen single-cell steep strokes to -w columns, except at the tips
			if *w > 1 && lo == hi && a.y < float64(y)-0.5 && b.y > float64(y)+0.5 {
				if lo < mid {
					set(y, lo+1)
				} else {
					set(y, lo-1)
				}
			}
		}
	}
	const word = "lunatic"
	n := 0
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
		fmt.Printf("\t%q,\n", strings.TrimRight(sb.String(), " "))
	}
}
