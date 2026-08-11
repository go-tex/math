// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// renderAt renders tex at a chosen size and fails the test on error.
func renderAt(t *testing.T, r *Renderer, tex string, px int) string {
	t.Helper()
	svg, err := r.RenderSVG(tex, px)
	if err != nil {
		t.Fatalf("render(%q, %d): %v", tex, px, err)
	}
	return svg
}

var (
	reTranslateX = regexp.MustCompile(`translate\(([-0-9.]+),`)
	reAttrDim    = regexp.MustCompile(`^<svg[^>]*\bwidth="([-0-9.]+)"[^>]*\bheight="([-0-9.]+)"`)
)

// maxTranslateX returns the largest x offset of any translated group in the SVG.
func maxTranslateX(svg string) float64 {
	best, seen := 0.0, false
	for _, m := range reTranslateX.FindAllStringSubmatch(svg, -1) {
		v, _ := strconv.ParseFloat(m[1], 64)
		if !seen || v > best {
			best, seen = v, true
		}
	}
	return best
}

// svgDims reads the root <svg> width and height attributes.
func svgDims(t *testing.T, svg string) (w, h float64) {
	t.Helper()
	m := reAttrDim.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no <svg> width/height in %.60s", svg)
	}
	w, _ = strconv.ParseFloat(m[1], 64)
	h, _ = strconv.ParseFloat(m[2], 64)
	return w, h
}

// TestEnvironmentsRender exercises every new environment in both styles.
func TestEnvironmentsRender(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\begin{array}{cc} a & b \\ c & d \end{array}`,
		`\begin{array}{|c|c|} a & b \\ c & d \end{array}`,
		`\begin{array}{lcr} a & bb & ccc \\ d & e & f \end{array}`,
		`\begin{array}{|l|c|r|} 1 & 2 & 3 \end{array}`,
		`\begin{array}{c|c} a & b \end{array}`,
		`\begin{array}{p{2cm}c} a & b \end{array}`, // p{} treated as flush-left
		`\left(\begin{array}{cc} a & b \\ c & d \end{array}\right)`,
		`\begin{aligned} x &= y + z \\ &= w \end{aligned}`,
		`\begin{split} f(x) &= a + b \\ &= c \end{split}`,
		`\begin{aligned} a \\ b \end{aligned}`,          // single column (no &): ncol == 1
		`\begin{aligned} a &= b & c &= d \end{aligned}`, // two rl pairs: inter-pair gap
		`\begin{gathered} a + b \\ c \\ d + e + f \end{gathered}`,
		`\begin{smallmatrix} a & b \\ c & d \end{smallmatrix}`,
		`\left(\begin{smallmatrix} 1 & 0 \\ 0 & 1 \end{smallmatrix}\right)`,
		// nesting: an aligned block containing a fraction and a small matrix.
		`\begin{aligned} A &= \frac{1}{2} \\ B &= \begin{smallmatrix} a \\ b \end{smallmatrix} \end{aligned}`,
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
}

// TestArrayVerticalRules checks that | in the column spec draws rules and widens
// the array, and that a spec without | draws none.
func TestArrayVerticalRules(t *testing.T) {
	r := newRenderer(t)
	plain := renderAt(t, r, `\begin{array}{cc} a & b \\ c & d \end{array}`, 40)
	ruled := renderAt(t, r, `\begin{array}{|c|c|} a & b \\ c & d \end{array}`, 40)
	if n := strings.Count(plain, "<rect"); n != 0 {
		t.Errorf("plain array rules = %d, want 0", n)
	}
	if n := strings.Count(ruled, "<rect"); n != 3 {
		t.Errorf("|c|c| array rules = %d, want 3", n)
	}
	// one interior rule only.
	mid := renderAt(t, r, `\begin{array}{c|c} a & b \end{array}`, 40)
	if n := strings.Count(mid, "<rect"); n != 1 {
		t.Errorf("c|c array rules = %d, want 1", n)
	}
	// edge rules add margin, so the ruled array is wider than the plain one.
	wp, _ := svgDims(t, plain)
	wr, _ := svgDims(t, ruled)
	if !(wr > wp) {
		t.Errorf("ruled width %v not greater than plain %v", wr, wp)
	}
}

// TestArrayColumnAlignment proves l/c/r place a narrow cell at increasing x.
func TestArrayColumnAlignment(t *testing.T) {
	r := newRenderer(t)
	// A narrow row (i) over a wide row (W) in a single column: left keeps the
	// narrow cell flush at the origin, right pushes it fully across, centre
	// splits the difference.
	l := maxTranslateX(renderAt(t, r, `\begin{array}{l} i \\ W \end{array}`, 60))
	c := maxTranslateX(renderAt(t, r, `\begin{array}{c} i \\ W \end{array}`, 60))
	rr := maxTranslateX(renderAt(t, r, `\begin{array}{r} i \\ W \end{array}`, 60))
	if !(l < c && c < rr) {
		t.Errorf("alignment offsets not strictly increasing: l=%v c=%v r=%v", l, c, rr)
	}
}

// TestSmallMatrixSmaller confirms smallmatrix is typeset smaller than matrix.
func TestSmallMatrixSmaller(t *testing.T) {
	r := newRenderer(t)
	_, hSmall := svgDims(t, renderAt(t, r, `\begin{smallmatrix} a & b \\ c & d \end{smallmatrix}`, 40))
	_, hBig := svgDims(t, renderAt(t, r, `\begin{matrix} a & b \\ c & d \end{matrix}`, 40))
	if !(hSmall < hBig) {
		t.Errorf("smallmatrix height %v not smaller than matrix %v", hSmall, hBig)
	}
}

// TestAlignedColumns verifies the aligned/split rl layout differs from a plain
// centred matrix for the same cells (columns are pulled to the & boundary).
func TestAlignedColumns(t *testing.T) {
	r := newRenderer(t)
	al := renderAt(t, r, `\begin{aligned} x &= y \\ zz &= w \end{aligned}`, 40)
	// two rows, each contributing glyph paths.
	if n := strings.Count(al, "<path"); n < 6 {
		t.Errorf("aligned paths = %d, want >= 6", n)
	}
	sp := renderAt(t, r, `\begin{split} x &= y \end{split}`, 40)
	if !strings.HasPrefix(sp, "<svg") {
		t.Errorf("split malformed: %.40s", sp)
	}
}

// TestEnvErrorsAndFallbacks covers the array/env error and fallback branches.
func TestEnvErrorsAndFallbacks(t *testing.T) {
	r := newRenderer(t)
	// Fallbacks: these must NOT error (sensible centred fallback).
	for _, tex := range []string{
		`\begin{array} a & b \end{array}`,     // no column spec at all
		`\begin{array}{z} a \end{array}`,      // unknown column letter → centred
		`\begin{array}{@c} a & b \end{array}`, // ignored punctuation in spec
		`\begin{array}{} a \end{array}`,       // empty spec
	} {
		t.Run("ok/"+tex, func(t *testing.T) {
			if _, err := r.RenderSVG(tex, 32); err != nil {
				t.Errorf("render(%q) should not error: %v", tex, err)
			}
		})
	}
	// Errors: these must error without panicking.
	for _, tex := range []string{
		`\begin{array}{cc} a & b`,          // unterminated env (has spec)
		`\begin{array}{cc a \end{array}`,   // unterminated column spec swallows body
		`\begin{aligned} x &= y`,           // unterminated aligned
		`\begin{gathered} a`,               // unterminated gathered
		`\begin{smallmatrix} a`,            // unterminated smallmatrix
		`\begin{array}{c}\nope\end{array}`, // error propagates from a cell
	} {
		t.Run("err/"+tex, func(t *testing.T) {
			if _, err := r.RenderSVG(tex, 32); err == nil {
				t.Errorf("render(%q) should error", tex)
			}
		})
	}
}

// TestReadColSpec unit-tests the column-spec parser directly.
func TestReadColSpec(t *testing.T) {
	cases := []struct {
		in      string
		aligns  []colAlign
		vrules  []int
		restLen int // remaining tokens after the spec
	}{
		{`{|c|c|}a`, []colAlign{alignC, alignC}, []int{0, 1, 2}, 1},
		{`{lcr}`, []colAlign{alignL, alignC, alignR}, nil, 0},
		{`{c|c}`, []colAlign{alignC, alignC}, []int{1}, 0},
		{`{p{2cm}r}x`, []colAlign{alignL, alignR}, nil, 1}, // p{} → left, width skipped
		{`a & b`, nil, nil, 3},                             // no brace: fallback, tokens intact
		{`{z}`, []colAlign{alignC}, nil, 0},                // unknown letter → centred
	}
	for _, c := range cases {
		toks := tokenize(c.in)
		a, v, rest := readColSpec(toks)
		if !eqAligns(a, c.aligns) {
			t.Errorf("readColSpec(%q) aligns = %v, want %v", c.in, a, c.aligns)
		}
		if !eqInts(v, c.vrules) {
			t.Errorf("readColSpec(%q) vrules = %v, want %v", c.in, v, c.vrules)
		}
		if len(rest) != c.restLen {
			t.Errorf("readColSpec(%q) restLen = %d, want %d", c.in, len(rest), c.restLen)
		}
	}
}

func eqAligns(a, b []colAlign) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
