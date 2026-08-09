// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package math is a pure-Go TeX math-mode typesetter: it parses a subset of TeX
// math syntax and lays it out to a self-contained SVG using the OpenType MATH
// table (via go-opentype) for metrics and vector glyph outlines — no TeX engine,
// no server, no cgo. It compiles to GOOS=js/wasm for offline, client-side math
// preview.
//
// Supported subset (prototype): letters (rendered as math italic), digits,
// operators and a named-symbol table (\alpha, \sum, \int, \leq, …), superscripts
// (^), subscripts (_), grouping ({…}) and fractions (\frac{num}{den}).
package math

import (
	"fmt"
	gomath "math"
	"strings"

	"github.com/go-opentype/opentype"
)

// Renderer typesets TeX math with a single MATH-table font.
type Renderer struct {
	font *opentype.Font
}

// New builds a Renderer from an OpenType font that must carry a MATH table
// (e.g. STIX Two Math, Latin Modern Math).
func New(fontBytes []byte) (*Renderer, error) {
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	if !f.HasMath() {
		return nil, fmt.Errorf("texmath: font has no MATH table")
	}
	return &Renderer{font: f}, nil
}

// RenderSVG typesets tex at the given base pixel size and returns a complete,
// self-contained <svg> document.
func (r *Renderer) RenderSVG(tex string, sizePx int) (string, error) {
	if sizePx <= 0 {
		sizePx = 40
	}
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm())}
	toks := tokenize(tex)
	b, _, err := e.parseRun(toks, sizePx, false)
	if err != nil {
		return "", err
	}
	return e.document(b), nil
}

// ── boxes ───────────────────────────────────────────────────────────────────

// box is a laid-out fragment in SVG coordinates (Y down), with its reference
// baseline at y=0 and left edge at x=0. h is the extent above the baseline, d
// the depth below it, w the advance width — all in pixels.
type box struct {
	svg  strings.Builder
	w    float64
	h, d float64
}

func (b *box) String() string { return b.svg.String() }

// place appends src into dst translated by (dx, dy).
func place(dst *box, src *box, dx, dy float64) {
	fmt.Fprintf(&dst.svg, `<g transform="translate(%s,%s)">%s</g>`, ftoa(dx), ftoa(dy), src.String())
}

// ── engine ──────────────────────────────────────────────────────────────────

type engine struct {
	font *opentype.Font
	upem float64
}

// mc reads a MATH constant scaled to sizePx (design units × sizePx/upem).
func (e *engine) mc(which opentype.MathConstant, sizePx int) float64 {
	fc := e.font.NewFace(sizePx)
	return float64(fc.MathConstant(which))
}

// scriptSize returns the pixel size for scripts, scaled by the font's
// ScriptPercentScaleDown MATH constant (a percentage).
func (e *engine) scriptSize(sizePx int) int {
	// Real MATH fonts define ScriptPercentScaleDown; the s<1 floor keeps scripts
	// visible even at tiny base sizes.
	pct := e.font.NewFace(sizePx).MathConstant(opentype.ScriptPercentScaleDown)
	s := sizePx * pct / 100
	if s < 1 {
		s = 1
	}
	return s
}

// glyphBox lays out a single rune as a box using its advance and ink extent.
func (e *engine) glyphBox(r rune, sizePx int) (*box, bool) {
	fc := e.font.NewFace(sizePx)
	gid, ok := e.font.GlyphIndex(r)
	if !ok {
		return nil, false
	}
	d, _ := fc.GlyphSVGPath(gid)
	segs, _ := fc.GlyphOutline(gid)
	scale := float64(sizePx) / e.upem
	minY, maxY := 0.0, 0.0
	for _, s := range segs {
		for _, p := range s.P {
			y := -p.Y * scale // font Y-up → SVG Y-down
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	b := &box{w: float64(fc.Advance(r)), h: -minY, d: maxY}
	if d != "" {
		fmt.Fprintf(&b.svg, `<path d="%s"/>`, d)
	}
	return b, true
}

// hbox concatenates boxes left to right on a shared baseline.
func hbox(items ...*box) *box {
	out := &box{}
	x := 0.0
	for _, it := range items {
		place(out, it, x, 0)
		x += it.w
		if it.h > out.h {
			out.h = it.h
		}
		if it.d > out.d {
			out.d = it.d
		}
	}
	out.w = x
	return out
}

// attachScripts places a superscript and/or subscript on a nucleus.
func (e *engine) attachScripts(nuc *box, sup, sub *box, sizePx int) *box {
	out := &box{}
	place(out, nuc, 0, 0)
	out.w, out.h, out.d = nuc.w, nuc.h, nuc.d
	x := nuc.w
	if sup != nil {
		shift := e.mc(opentype.SuperscriptShiftUp, sizePx)
		place(out, sup, x, -shift)
		if t := shift + sup.h; t > out.h {
			out.h = t
		}
		if r := x + sup.w; r > out.w {
			out.w = r
		}
	}
	if sub != nil {
		shift := e.mc(opentype.SubscriptShiftDown, sizePx)
		place(out, sub, x, shift)
		if b := shift + sub.d; b > out.d {
			out.d = b
		}
		if r := x + sub.w; r > out.w {
			out.w = r
		}
	}
	return out
}

// fraction stacks num over den with a rule at the math axis.
func (e *engine) fraction(num, den *box, sizePx int) *box {
	axis := e.mc(opentype.AxisHeight, sizePx)
	rule := e.mc(opentype.FractionRuleThickness, sizePx)
	gapN := e.mc(opentype.FractionNumeratorGapMin, sizePx)
	gapD := e.mc(opentype.FractionDenominatorGapMin, sizePx)
	pad := float64(sizePx) * 0.1
	w := gomath.Max(num.w, den.w) + 2*pad

	out := &box{w: w}
	// Numerator: its baseline sits so its depth clears the rule + gap above axis.
	numBaseline := -(axis + rule/2 + gapN + num.d)
	place(out, num, (w-num.w)/2, numBaseline)
	// Denominator below the axis.
	denBaseline := -axis + rule/2 + gapD + den.h
	place(out, den, (w-den.w)/2, denBaseline)
	// The rule.
	fmt.Fprintf(&out.svg, `<rect x="0" y="%s" width="%s" height="%s"/>`,
		ftoa(-axis-rule/2), ftoa(w), ftoa(rule))

	out.h = -numBaseline + num.h
	out.d = denBaseline + den.d
	return out
}

// document wraps a laid-out box in a padded, baseline-shifted SVG root.
func (e *engine) document(b *box) string {
	pad := 4.0
	w := b.w + 2*pad
	h := b.h + b.d + 2*pad
	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		ftoa(w), ftoa(h), ftoa(w), ftoa(h))
	// Fill rule: paths use nonzero; black by default.
	fmt.Fprintf(&s, `<g fill="currentColor" transform="translate(%s,%s)">%s</g>`,
		ftoa(pad), ftoa(pad+b.h), b.String())
	s.WriteString(`</svg>`)
	return s.String()
}

// ftoa formats a float with up to three decimals and no trailing zeros.
func ftoa(v float64) string {
	if v == 0 {
		return "0"
	}
	s := strings.TrimRight(fmt.Sprintf("%.3f", v), "0")
	return strings.TrimSuffix(s, ".")
}
