package main

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// write writes the report as text: what is in one store alone, then a row
// for every array that differs, most tiles changed first.
func (r *Report) write(w io.Writer) error {
	b := bufio.NewWriter(w)
	fmt.Fprintf(b, "a: %s\nb: %s\n", r.A, r.B)
	differ := 0
	for _, d := range r.Arrays {
		if d.differs() {
			differ++
		}
	}
	fmt.Fprintf(b, "%d arrays in both, %d differ; %d only in a, %d only in b\n", len(r.Arrays), differ, len(r.OnlyA), len(r.OnlyB))
	list := func(what string, paths []string) {
		for _, p := range paths {
			fmt.Fprintf(b, "  %s  %s\n", what, name(p))
		}
	}
	list("only in a:", r.OnlyA)
	list("only in b:", r.OnlyB)
	list("group only in a:", r.GroupsOnlyA)
	list("group only in b:", r.GroupsOnlyB)
	for _, g := range r.Groups {
		fmt.Fprintf(b, "  group %s: attributes differ: %s\n", name(g.Path), strings.Join(g.Attributes, ", "))
	}
	if r.Same {
		fmt.Fprintln(b, "the same")
		return b.Flush()
	}

	// A row an array: the share of its tiles changed, the largest and mean
	// change, and the rows and columns the changes lie within; what more
	// there is to say goes on the lines under it.
	width := len("array")
	for _, d := range r.Arrays {
		if d.differs() {
			width = max(width, len(d.Path))
		}
	}
	fmt.Fprintf(b, "\n%-*s  %8s  %10s  %10s  %-11s  %s\n", width, "array", "tiles", "max|a-b|", "mean|a-b|", "y", "x")
	for _, d := range r.Arrays {
		if !d.differs() {
			continue
		}
		fmt.Fprintf(b, "%-*s  ", width, d.Path)
		switch {
		case !d.Compared:
			fmt.Fprintf(b, "not compared: %s %v against %s %v\n", d.TypeA, d.ShapeA, d.TypeB, d.ShapeB)
		case d.Changed == 0:
			fmt.Fprintf(b, "%8s\n", "0%")
		default:
			most, mean, y, x := "", "", "", ""
			if d.measured > 0 {
				most, mean = number(d.MaxAbs), number(d.MeanAbs)
			}
			if d.Box != nil {
				y = fmt.Sprintf("%d-%d", d.Box.Y0, d.Box.Y1)
				if len(d.ShapeA) >= 2 {
					x = fmt.Sprintf("%d-%d", d.Box.X0, d.Box.X1)
				}
			}
			fmt.Fprintf(b, "%8s  %10s  %10s  %-11s  %s\n", percent(d.Share), most, mean, y, x)
		}
		under := func(format string, args ...any) {
			fmt.Fprintf(b, "%*s"+format+"\n", append([]any{width + 2, ""}, args...)...)
		}
		if len(d.Attributes) > 0 {
			under("attributes differ: %s", strings.Join(d.Attributes, ", "))
		}
		if d.Changed > 0 {
			under("%d of %d tiles, %d of %d elements", d.TilesChanged, d.Tiles, d.Changed, d.Elements)
		}
		if d.Unmeasured > 0 {
			under("%d to or from NaN, or by an infinite amount", d.Unmeasured)
		}
		for _, c := range d.Codes {
			under("%8d  %s -> %s", c.Count, code(c.From, c.FromName), code(c.To, c.ToName))
		}
		if d.Other > 0 {
			under("%8d  other changes of code", d.Other)
		}
		if d.PNG != "" {
			under("where: %s", d.PNG)
		}
	}
	return b.Flush()
}

func name(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func percent(share float64) string {
	p := 100 * share
	if p > 0 && p < 0.01 {
		return "<0.01%"
	}
	return strconv.FormatFloat(p, 'f', 2, 64) + "%"
}

func number(v float64) string { return strconv.FormatFloat(v, 'g', 4, 64) }

func code(c int64, name string) string {
	if name == "" {
		return strconv.FormatInt(c, 10)
	}
	return fmt.Sprintf("%d %s", c, name)
}

// picture is where a map array changed, shrunk to a side at most: each
// pixel keeps the largest change of the tiles under it, +Inf for a change
// with no amount (a code, a bool, a NaN).
type picture struct {
	w, h, scale int
	mag         []float32
	changed     []bool
}

func newPicture(rows, cols, side int) *picture {
	scale := max(1, (max(rows, cols)+side-1)/side)
	w, h := (cols+scale-1)/scale, (rows+scale-1)/scale
	return &picture{w: w, h: h, scale: scale, mag: make([]float32, w*h), changed: make([]bool, w*h)}
}

func (p *picture) mark(y, x int, mag float32) {
	i := (y/p.scale)*p.w + x/p.scale
	p.changed[i] = true
	p.mag[i] = max(p.mag[i], mag)
}

// write writes the picture into dir, named for the array's path, and says
// where. Unchanged is black; a change is grey from 64, rising with its amount
// to 255 at the largest in the array, and 255 where it has no amount.
func (p *picture) write(dir, path string) (string, error) {
	top := float32(0)
	for i, m := range p.mag {
		if p.changed[i] && !math.IsInf(float64(m), 0) {
			top = max(top, m)
		}
	}
	img := image.NewGray(image.Rect(0, 0, p.w, p.h))
	for i, m := range p.mag {
		switch {
		case !p.changed[i]:
		case math.IsInf(float64(m), 0) || top == 0:
			img.Pix[i] = 255
		default:
			img.Pix[i] = uint8(64 + math.Round(float64(191*m/top)))
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	file := filepath.Join(dir, strings.ReplaceAll(path, "/", "_")+".png")
	f, err := os.Create(file)
	if err != nil {
		return "", err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return "", err
	}
	return file, f.Close()
}
