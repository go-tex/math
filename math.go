// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package math is a pure-Go TeX math-mode typesetter: it parses a broad subset
// of TeX math syntax and lays it out to a self-contained SVG using the OpenType
// MATH table (via go-opentype) for metrics and vector glyph outlines — no TeX
// engine, no server, no cgo. It compiles to GOOS=js/wasm for offline math
// preview.
//
// Supported: math-italic variables and a large named-symbol table with proper
// atom-class spacing (Appendix G); superscripts/subscripts and big-operator
// limits; fractions; radicals (\sqrt, \sqrt[n]); stretchy delimiters
// (\left…\right); accents (\hat, \vec, \bar, …); \overline/\underline; math
// alphabets (\mathbb, \mathcal, \mathfrak, \mathrm, \mathbf, \mathsf, \mathtt,
// \mathit); \text; matrices (matrix/pmatrix/bmatrix/vmatrix/Vmatrix/cases),
// array (with an l/c/r + | column spec), aligned/split, gathered and
// smallmatrix; primes; spacing commands; \displaystyle/\textstyle; declarative
// in-math font switches (\rm \bf \it \sf \tt \cal \sl); named operators (\log,
// \sin, \lim, …) and \operatorname / \operatorname*; and the modular
// annotations \bmod, \pmod, \mod and \pod.
package math

import (
	"fmt"
	gomath "math"
	"strings"

	"github.com/go-opentype/opentype"
)

// Renderer typesets TeX math with a single MATH-table font.
type Renderer struct{ font *opentype.Font }

// New builds a Renderer from an OpenType font carrying a MATH table.
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

// RenderSVG typesets tex at the given base pixel size (inline/text style) and
// returns a complete, self-contained <svg> document.
func (r *Renderer) RenderSVG(tex string, sizePx int) (string, error) {
	return r.render(tex, sizePx, false)
}

// RenderDisplaySVG is like RenderSVG but in display style (larger operators,
// limits set above/below).
func (r *Renderer) RenderDisplaySVG(tex string, sizePx int) (string, error) {
	return r.render(tex, sizePx, true)
}

// Metrics describes a rendered math box in pixels. Width is the advance width;
// Height the extent above the baseline; Depth the extent below it. In the SVG
// returned alongside these metrics the baseline sits at y=Height, and the box
// spans [0,Width]×[0,Height+Depth] with no padding — so a caller can align the
// math baseline with its surrounding text baseline exactly (place the SVG's top
// edge Height above the baseline), which RenderSVG's padded, baseline-agnostic
// output does not permit.
type Metrics struct {
	Width, Height, Depth float64
}

// RenderSVGMetrics typesets tex at inline/text style like RenderSVG, but returns
// a tightly-cropped SVG together with its exact box Metrics (advance width,
// height above the baseline, depth below). This is the baseline-aware form: the
// caller can place the box so the math baseline meets the text baseline instead
// of guessing from the overall height.
func (r *Renderer) RenderSVGMetrics(tex string, sizePx int) (string, Metrics, error) {
	return r.renderM(tex, sizePx, false)
}

// RenderDisplaySVGMetrics is RenderSVGMetrics in display style.
func (r *Renderer) RenderDisplaySVGMetrics(tex string, sizePx int) (string, Metrics, error) {
	return r.renderM(tex, sizePx, true)
}

func (r *Renderer) renderM(tex string, sizePx int, display bool) (string, Metrics, error) {
	if sizePx <= 0 {
		sizePx = 40
	}
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm())}
	b, _, err := e.parseList(resolveConditionals(tokenize(tex)), style{px: sizePx, display: display, spacious: true}, stopEnd)
	if err != nil {
		return "", Metrics{}, err
	}
	return e.documentTight(b), Metrics{Width: b.w, Height: b.h, Depth: b.d}, nil
}

func (r *Renderer) render(tex string, sizePx int, display bool) (string, error) {
	if sizePx <= 0 {
		sizePx = 40
	}
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm())}
	b, _, err := e.parseList(resolveConditionals(tokenize(tex)), style{px: sizePx, display: display, spacious: true}, stopEnd)
	if err != nil {
		return "", err
	}
	return e.document(b), nil
}

// ── style ───────────────────────────────────────────────────────────────────

// style carries the current typesetting context down the layout.
type style struct {
	px       int             // pixel size at this level
	display  bool            // display style (limits above/below, larger stacks)
	spacious bool            // apply inter-atom spacing (off inside scripts)
	alpha    func(rune) rune // active letter alphabet (nil = math italic)
}

// script returns the style for scripts of the current level.
func (s style) script(e *engine) style {
	return style{px: e.scriptSize(s.px), spacious: false, alpha: s.alpha}
}

// inner returns the text-style context for fraction parts / matrix cells.
func (s style) inner() style {
	return style{px: s.px, spacious: s.spacious, alpha: s.alpha}
}

// ── atom classes & boxes ────────────────────────────────────────────────────

type atomClass uint8

const (
	clsOrd atomClass = iota
	clsOp
	clsBin
	clsRel
	clsOpen
	clsClose
	clsPunct
	clsInner
)

// box is a laid-out fragment in SVG coordinates (Y down), baseline at y=0, left
// edge at x=0. h is the extent above the baseline, d the depth below, w the
// advance width — all in pixels. cls is the atom class for spacing.
type box struct {
	svg  strings.Builder
	w    float64
	h, d float64
	cls  atomClass
}

func (b *box) String() string { return b.svg.String() }

func newBox(cls atomClass) *box { return &box{cls: cls} }

// place appends src into dst translated by (dx, dy).
func place(dst, src *box, dx, dy float64) {
	fmt.Fprintf(&dst.svg, `<g transform="translate(%s,%s)">%s</g>`, ftoa(dx), ftoa(dy), src.String())
}

// rule appends a filled rectangle (x,y,w,h) to dst.
func rule(dst *box, x, y, w, h float64) {
	fmt.Fprintf(&dst.svg, `<rect x="%s" y="%s" width="%s" height="%s"/>`, ftoa(x), ftoa(y), ftoa(w), ftoa(h))
}

// ── engine ──────────────────────────────────────────────────────────────────

type engine struct {
	font *opentype.Font
	upem float64
}

func (e *engine) face(px int) *opentype.Face { return e.font.NewFace(px) }

// mc reads a MATH constant scaled to px.
func (e *engine) mc(which opentype.MathConstant, px int) float64 {
	return float64(e.face(px).MathConstant(which))
}

func (e *engine) axis(px int) float64 { return e.mc(opentype.AxisHeight, px) }

func (e *engine) scriptSize(px int) int {
	pct := e.face(px).MathConstant(opentype.ScriptPercentScaleDown)
	s := px * pct / 100
	if s < 1 {
		s = 1
	}
	return s
}

// scale returns px-per-font-unit at size px.
func (e *engine) scale(px int) float64 { return float64(px) / e.upem }

// glyphBox lays out a single rune as a box of the given class.
func (e *engine) glyphBox(r rune, px int, cls atomClass) (*box, bool) {
	gid, ok := e.font.GlyphIndex(r)
	if !ok {
		return nil, false
	}
	return e.gidBox(gid, px, cls), true
}

// gidBox builds a box for an already-resolved glyph index.
func (e *engine) gidBox(gid opentype.GlyphIndex, px int, cls atomClass) *box {
	fc := e.face(px)
	d, _ := fc.GlyphSVGPath(gid)
	h, dp := e.gidVExtent(gid, px)
	b := &box{w: float64(fc.AdvanceIndex(gid)), h: h, d: dp, cls: cls}
	if d != "" {
		fmt.Fprintf(&b.svg, `<path d="%s"/>`, d)
	}
	return b
}

// gidVExtent returns a glyph's ink extent above (h) and below (d) the baseline,
// in pixels (SVG Y-down).
func (e *engine) gidVExtent(gid opentype.GlyphIndex, px int) (h, d float64) {
	segs, _ := e.face(px).GlyphOutline(gid)
	s := e.scale(px)
	minY, maxY := 0.0, 0.0
	for _, sg := range segs {
		for _, p := range sg.P {
			y := -p.Y * s
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	return -minY, maxY
}

// interAtom returns the space (px) inserted between two adjacent atom classes,
// per TeX's Appendix G table (thin=3mu, med=4mu, thick=5mu; mu = em/18).
func interAtom(l, r atomClass, px int) float64 {
	const (
		_ = iota
		thin
		med
		thick
	)
	table := [8][8]int{
		clsOrd:   {clsOp: thin, clsBin: med, clsRel: thick, clsInner: thin},
		clsOp:    {clsOrd: thin, clsOp: thin, clsRel: thick, clsInner: thin},
		clsBin:   {clsOrd: med, clsOp: med, clsOpen: med, clsInner: med},
		clsRel:   {clsOrd: thick, clsOp: thick, clsOpen: thick, clsInner: thick},
		clsClose: {clsOp: thin, clsBin: med, clsRel: thick, clsInner: thin},
		clsPunct: {clsOrd: thin, clsOp: thin, clsRel: thin, clsOpen: thin, clsClose: thin, clsPunct: thin, clsInner: thin},
		clsInner: {clsOrd: thin, clsOp: thin, clsBin: med, clsRel: thick, clsOpen: thin, clsPunct: thin, clsInner: thin},
	}
	mu := float64(px) / 18
	return float64(table[l][r]) * mu
}

// hlist lays boxes left to right on a shared baseline, inserting inter-atom
// spacing when sty.spacious, and resolving Bin→Ord where a binary operator
// cannot stand.
func (e *engine) hlist(items []*box, sty style) *box {
	out := newBox(clsOrd)
	if len(items) == 0 {
		return out
	}
	for i, b := range items {
		if b.cls != clsBin {
			continue
		}
		prev := clsOpen
		if i > 0 {
			prev = items[i-1].cls
		}
		if i == 0 || prev == clsBin || prev == clsOp || prev == clsRel || prev == clsOpen || prev == clsPunct {
			b.cls = clsOrd
		}
	}
	x := 0.0
	for i, it := range items {
		if i > 0 && sty.spacious {
			x += interAtom(items[i-1].cls, it.cls, sty.px)
		}
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
	if len(items) == 1 {
		out.cls = items[0].cls
	}
	return out
}

// attachScripts places a superscript and/or subscript on a nucleus.
func (e *engine) attachScripts(nuc, sup, sub *box, sty style) *box {
	out := newBox(nuc.cls)
	place(out, nuc, 0, 0)
	out.w, out.h, out.d = nuc.w, nuc.h, nuc.d
	x := nuc.w
	if sup != nil {
		shift := gomath.Max(e.mc(opentype.SuperscriptShiftUp, sty.px), nuc.h-e.mc(opentype.SuperscriptBaselineDropMax, sty.px)*0)
		place(out, sup, x, -shift)
		if t := shift + sup.h; t > out.h {
			out.h = t
		}
		if r := x + sup.w; r > out.w {
			out.w = r
		}
	}
	if sub != nil {
		shift := gomath.Max(e.mc(opentype.SubscriptShiftDown, sty.px), nuc.d)
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

// attachLimits stacks a lower and/or upper limit centred over/under a big
// operator (display style).
func (e *engine) attachLimits(op, over, under *box, sty style) *box {
	w := op.w
	if over != nil && over.w > w {
		w = over.w
	}
	if under != nil && under.w > w {
		w = under.w
	}
	out := newBox(clsOp)
	out.w = w
	place(out, op, (w-op.w)/2, 0)
	out.h, out.d = op.h, op.d
	if over != nil {
		base := op.h + e.mc(opentype.UpperLimitGapMin, sty.px) + over.d
		place(out, over, (w-over.w)/2, -base)
		out.h = base + over.h
	}
	if under != nil {
		base := op.d + e.mc(opentype.LowerLimitGapMin, sty.px) + under.h
		place(out, under, (w-under.w)/2, base)
		out.d = base + under.d
	}
	return out
}

// fraction stacks num over den with a rule on the math axis.
func (e *engine) fraction(num, den *box, sty style) *box {
	px := sty.px
	axis := e.axis(px)
	rt := e.mc(opentype.FractionRuleThickness, px)
	gapN := e.mc(opentype.FractionNumeratorGapMin, px)
	gapD := e.mc(opentype.FractionDenominatorGapMin, px)
	if sty.display {
		gapN = e.mc(opentype.FractionNumDisplayStyleGapMin, px)
		gapD = e.mc(opentype.FractionDenomDisplayStyleGapMin, px)
	}
	pad := float64(px) * 0.12
	w := gomath.Max(num.w, den.w) + 2*pad
	out := newBox(clsInner)
	out.w = w
	numBase := -(axis + rt/2 + gapN + num.d)
	place(out, num, (w-num.w)/2, numBase)
	denBase := -axis + rt/2 + gapD + den.h
	place(out, den, (w-den.w)/2, denBase)
	rule(out, 0, -axis-rt/2, w, rt)
	out.h = -numBase + num.h
	out.d = denBase + den.d
	return out
}

// binom stacks num over den with NO rule (the body of \binom, wrapped in
// parentheses by the caller). It reuses the fraction gaps, widened by the rule
// thickness that a fraction would occupy, so the two rows sit a little apart.
func (e *engine) binom(num, den *box, sty style) *box {
	px := sty.px
	axis := e.axis(px)
	rt := e.mc(opentype.FractionRuleThickness, px)
	gapN := e.mc(opentype.FractionNumeratorGapMin, px) + rt
	gapD := e.mc(opentype.FractionDenominatorGapMin, px) + rt
	if sty.display {
		gapN = e.mc(opentype.FractionNumDisplayStyleGapMin, px) + rt
		gapD = e.mc(opentype.FractionDenomDisplayStyleGapMin, px) + rt
	}
	pad := float64(px) * 0.12
	w := gomath.Max(num.w, den.w) + 2*pad
	out := newBox(clsInner)
	out.w = w
	numBase := -(axis + gapN + num.d)
	place(out, num, (w-num.w)/2, numBase)
	denBase := -axis + gapD + den.h
	place(out, den, (w-den.w)/2, denBase)
	out.h = -numBase + num.h
	out.d = denBase + den.d
	return out
}

// overUnder centres a small box above (over) or below (under) a base box, as
// \overset/\underset/\stackrel do.
func (e *engine) overUnder(base, extra *box, over bool, sty style) *box {
	out := newBox(base.cls)
	out.w = gomath.Max(base.w, extra.w)
	gap := float64(sty.px) * 0.1
	place(out, base, (out.w-base.w)/2, 0)
	if over {
		ty := -(base.h + gap + extra.d)
		place(out, extra, (out.w-extra.w)/2, ty)
		out.h = -ty + extra.h
		out.d = base.d
	} else {
		ty := base.d + gap + extra.h
		place(out, extra, (out.w-extra.w)/2, ty)
		out.h = base.h
		out.d = ty + extra.d
	}
	return out
}

// boxed draws a rectangular frame around the content (\boxed).
func (e *engine) boxed(b *box, sty style) *box {
	pad := float64(sty.px) * 0.15
	rt := e.mc(opentype.FractionRuleThickness, sty.px)
	if rt <= 0 {
		rt = float64(sty.px) * 0.05
	}
	out := newBox(clsOrd)
	out.w = b.w + 2*pad
	place(out, b, pad, 0)
	top, bot := -(b.h + pad), b.d+pad
	out.h, out.d = b.h+pad, b.d+pad
	rule(out, 0, top, out.w, rt)          // top
	rule(out, 0, bot-rt, out.w, rt)       // bottom
	rule(out, 0, top, rt, bot-top)        // left
	rule(out, out.w-rt, top, rt, bot-top) // right
	return out
}

// brace approximates \overbrace/\underbrace with a rule above/below the content
// (the exact curly brace is not modelled); a following sub/superscript labels it.
func (e *engine) brace(b *box, over bool, sty style) *box {
	if over {
		return e.overline(b, sty)
	}
	return e.underline(b, sty)
}

// phantom reserves the argument's dimensions without drawing it (\phantom), or
// only its width (\hphantom) / only its height+depth (\vphantom).
func (e *engine) phantom(b *box, name string) *box {
	out := newBox(clsOrd)
	switch name {
	case "hphantom":
		out.w = b.w
	case "vphantom":
		out.h, out.d = b.h, b.d
	default:
		out.w, out.h, out.d = b.w, b.h, b.d
	}
	return out
}

// braWrap wraps a box in fixed-size open/close delimiter glyphs (braket/physics
// notation: \ket, \bra, \norm, …).
func (e *engine) braWrap(open, close rune, arg *box, sty style) *box {
	return e.hlist([]*box{
		e.mustGlyph(open, sty.px, clsOpen),
		arg,
		e.mustGlyph(close, sty.px, clsClose),
	}, sty)
}

// negate overlays a slash on a box, centred on its ink, for \not.
func (e *engine) negate(b *box, sty style) *box {
	out := newBox(b.cls)
	out.w, out.h, out.d = b.w, b.h, b.d
	place(out, b, 0, 0)
	sl := e.mustGlyph('/', sty.px, clsOrd)
	dx := (b.w - sl.w) / 2
	dy := (b.d-b.h)/2 - (sl.d-sl.h)/2 // align the slash's mid-height with the atom's
	place(out, sl, dx, dy)
	if t := -dy + sl.h; t > out.h {
		out.h = t
	}
	if bt := dy + sl.d; bt > out.d {
		out.d = bt
	}
	return out
}

// overline / underline draw a rule above / below the content.
func (e *engine) overline(b *box, sty style) *box {
	gap := e.mc(opentype.OverbarVerticalGap, sty.px)
	rt := e.mc(opentype.OverbarRuleThickness, sty.px)
	asc := e.mc(opentype.OverbarExtraAscender, sty.px)
	out := newBox(clsOrd)
	out.w, out.d = b.w, b.d
	place(out, b, 0, 0)
	top := b.h + gap
	rule(out, 0, -(top + rt), b.w, rt)
	out.h = top + rt + asc
	return out
}

func (e *engine) underline(b *box, sty style) *box {
	gap := e.mc(opentype.UnderbarVerticalGap, sty.px)
	rt := e.mc(opentype.UnderbarRuleThickness, sty.px)
	desc := e.mc(opentype.UnderbarExtraDescender, sty.px)
	out := newBox(clsOrd)
	out.w, out.h = b.w, b.h
	place(out, b, 0, 0)
	bot := b.d + gap
	rule(out, 0, bot, b.w, rt)
	out.d = bot + rt + desc
	return out
}

// accent places an accent glyph centred over the nucleus.
func (e *engine) accent(nuc *box, acc rune, sty style) *box {
	ab := e.mustGlyph(acc, sty.px, clsOrd)
	out := newBox(nuc.cls)
	out.w, out.d = nuc.w, nuc.d
	place(out, nuc, 0, 0)
	ax := (nuc.w - ab.w) / 2
	gap := float64(sty.px) * 0.04
	ty := -(nuc.h + gap + ab.d) // accent ink bottom sits just above the nucleus
	place(out, ab, ax, ty)
	out.h = -ty + ab.h
	return out
}

// radical typesets \sqrt{body} (index may be nil) with a stretchy radical sign.
func (e *engine) radical(body, index *box, sty style) *box {
	px := sty.px
	rt := e.mc(opentype.RadicalRuleThickness, px)
	gap := e.mc(opentype.RadicalVerticalGap, px)
	if sty.display {
		gap = e.mc(opentype.RadicalDisplayStyleVerticalGap, px)
	}
	asc := e.mc(opentype.RadicalExtraAscender, px)
	target := body.h + body.d + gap + rt
	sign := e.stretchVertical('√', target, px, clsOrd)
	out := newBox(clsOrd)
	ruleY := -(body.h + gap + rt)
	x := 0.0
	if index != nil {
		place(out, index, x, ruleY+index.d*0)
		x += index.w * 0.8
	}
	// Place the sign so its ink top meets the rule; its tick then reaches down to
	// (about) the body's depth.
	ty := ruleY + sign.h
	place(out, sign, x, ty)
	x += sign.w
	place(out, body, x, 0)
	rule(out, x, ruleY, body.w, rt)
	out.w = x + body.w
	out.h = -ruleY + rt + asc
	out.d = gomath.Max(body.d, ty+sign.d)
	return out
}

// stretchVertical returns the natural box (baseline-relative, not re-centred)
// for rune r grown to at least target height using MATH size variants; the
// largest variant is the fallback.
func (e *engine) stretchVertical(r rune, target float64, px int, cls atomClass) *box {
	gid, _ := e.font.GlyphIndex(r) // 0 (notdef) if absent
	best := gid
	variants, _ := e.face(px).MathVariants(gid, true)
	for _, v := range variants {
		best = v.Glyph
		if float64(v.Advance) >= target {
			break
		}
	}
	return e.gidBox(best, px, cls)
}

// axisCentre re-positions a box so its vertical midpoint sits on the math axis
// (used for stretchy delimiters, which straddle the axis).
func (e *engine) axisCentre(b *box, px int) *box {
	dy := -e.axis(px) - (b.h-b.d)/2
	out := newBox(b.cls)
	out.w = b.w
	place(out, b, 0, dy)
	out.h = b.h - dy
	out.d = b.d + dy
	if out.d < 0 {
		out.d = 0
	}
	return out
}

// delimited wraps content in stretchy left/right delimiters (0 rune = none).
func (e *engine) delimited(inner *box, open, close rune, sty style) *box {
	target := 2 * gomath.Max(inner.h-e.axis(sty.px), inner.d+e.axis(sty.px))
	out := newBox(clsInner)
	x := 0.0
	grow := func(b *box) {
		if b.h > out.h {
			out.h = b.h
		}
		if b.d > out.d {
			out.d = b.d
		}
	}
	if open != 0 {
		l := e.axisCentre(e.stretchVertical(open, target, sty.px, clsOpen), sty.px)
		place(out, l, x, 0)
		x += l.w
		grow(l)
	}
	place(out, inner, x, 0)
	grow(inner)
	x += inner.w
	if close != 0 {
		rr := e.axisCentre(e.stretchVertical(close, target, sty.px, clsClose), sty.px)
		place(out, rr, x, 0)
		x += rr.w
		grow(rr)
	}
	out.w = x
	return out
}

// colAlign selects a matrix/array column's horizontal alignment.
type colAlign uint8

const (
	alignC colAlign = iota // centred (matrix default; zero value)
	alignL                 // flush left
	alignR                 // flush right
)

// gridOpts parameterises the generic grid layout.
type gridOpts struct {
	aligns []colAlign // per-column alignment; short/nil → remaining cols centred
	gaps   []float64  // per-column right gap (len ncol-1); nil → uniform colGap
	colGap float64    // uniform inter-column gap (also the edge margin for vrules)
	rowGap float64    // inter-row gap
	vrules []int      // gap indices 0..ncol at which to draw a vertical rule
}

// gridLayout lays rows/cols on a grid centred on the math axis, with per-column
// alignment and optional vertical rules between columns. It is the common core
// for matrix, array, aligned, gathered and smallmatrix.
func (e *engine) gridLayout(rows [][]*box, o gridOpts, sty style) *box {
	ncol := 0
	for _, r := range rows {
		if len(r) > ncol {
			ncol = len(r)
		}
	}
	align := func(j int) colAlign {
		if j < len(o.aligns) {
			return o.aligns[j]
		}
		return alignC
	}
	gap := func(j int) float64 { // gap to the right of column j
		if j < len(o.gaps) {
			return o.gaps[j]
		}
		return o.colGap
	}
	colW := make([]float64, ncol)
	rowH := make([]float64, len(rows))
	rowD := make([]float64, len(rows))
	for i, r := range rows {
		for j, c := range r {
			if c.w > colW[j] {
				colW[j] = c.w
			}
			if c.h > rowH[i] {
				rowH[i] = c.h
			}
			if c.d > rowD[i] {
				rowD[i] = c.d
			}
		}
	}
	totalH := 0.0
	for i := range rows {
		totalH += rowH[i] + rowD[i]
		if i > 0 {
			totalH += o.rowGap
		}
	}
	// vertical-rule bookkeeping and left margin.
	hasRule := func(g int) bool {
		for _, v := range o.vrules {
			if v == g {
				return true
			}
		}
		return false
	}
	x0 := 0.0
	if hasRule(0) {
		x0 = o.colGap // room for a left-edge rule
	}
	// column start positions and the right edge.
	colStart := make([]float64, ncol)
	x := x0
	for j := 0; j < ncol; j++ {
		colStart[j] = x
		x += colW[j]
		if j < ncol-1 {
			x += gap(j)
		}
	}
	rightEnd := x
	grid := newBox(clsInner)
	axis := e.axis(sty.px)
	grid.h = totalH/2 + axis
	grid.d = totalH/2 - axis
	// draw vertical rules first (behind the cells).
	if len(o.vrules) > 0 {
		rt := e.mc(opentype.FractionRuleThickness, sty.px)
		for _, g := range o.vrules {
			var rx float64
			switch {
			case g <= 0:
				rx = x0 / 2
			case g >= ncol:
				rx = rightEnd + o.colGap/2
			default:
				rx = colStart[g] - gap(g-1)/2
			}
			rule(grid, rx-rt/2, -grid.h, rt, grid.h+grid.d)
		}
	}
	y := -(totalH / 2) - axis
	for i, r := range rows {
		y += rowH[i]
		for j := 0; j < ncol; j++ {
			if j < len(r) && r[j] != nil {
				var off float64
				switch align(j) {
				case alignR:
					off = colW[j] - r[j].w
				case alignC:
					off = (colW[j] - r[j].w) / 2
				}
				place(grid, r[j], colStart[j]+off, y)
			}
		}
		y += rowD[i] + o.rowGap
	}
	gw := rightEnd
	if hasRule(ncol) {
		gw += o.colGap
	}
	grid.w = gw
	return grid
}

// alignedAligns returns the alternating right/left column alignments used by the
// aligned/split environments (rl pairs joined on &).
func alignedAligns(ncol int) []colAlign {
	a := make([]colAlign, ncol)
	for j := range a {
		if j%2 == 0 {
			a[j] = alignR
		} else {
			a[j] = alignL
		}
	}
	return a
}

// alignedGaps gives tight gaps inside each rl pair and wider gaps between pairs.
func alignedGaps(ncol, px int) []float64 {
	if ncol <= 1 {
		return nil
	}
	g := make([]float64, ncol-1)
	for j := range g {
		if j%2 == 0 { // between the right and left halves of a pair
			g[j] = float64(px) * 0.17
		} else { // between two pairs
			g[j] = float64(px) * 0.9
		}
	}
	return g
}

// finishEnv lays out a parsed environment's rows according to its kind.
// cellPx is the pixel size cells were typeset at (script size for smallmatrix).
func (e *engine) finishEnv(info envInfo, rows [][]*box, aligns []colAlign, vrules []int, cellPx int, sty style) *box {
	ncol := 0
	for _, r := range rows {
		if len(r) > ncol {
			ncol = len(r)
		}
	}
	p := float64(cellPx)
	switch info.kind {
	case kindArray:
		return e.gridLayout(rows, gridOpts{aligns: aligns, colGap: p * 0.5, rowGap: p * 0.4, vrules: vrules}, sty)
	case kindAligned:
		return e.gridLayout(rows, gridOpts{aligns: alignedAligns(ncol), gaps: alignedGaps(ncol, cellPx), colGap: p * 0.5, rowGap: p * 0.5}, sty)
	case kindGathered:
		return e.gridLayout(rows, gridOpts{colGap: p * 0.6, rowGap: p * 0.5}, sty)
	case kindSmall:
		return e.gridLayout(rows, gridOpts{colGap: p * 0.6, rowGap: p * 0.35}, sty)
	default: // matrix family
		grid := e.gridLayout(rows, gridOpts{colGap: p * 0.6, rowGap: p * 0.4}, sty)
		if info.open == 0 && info.close == 0 {
			return grid
		}
		return e.delimited(grid, info.open, info.close, sty)
	}
}

// document wraps a laid-out box in a padded, baseline-shifted SVG root.
func (e *engine) document(b *box) string {
	pad := 6.0
	w := b.w + 2*pad
	h := b.h + b.d + 2*pad
	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		ftoa(w), ftoa(h), ftoa(w), ftoa(h))
	fmt.Fprintf(&s, `<g fill="currentColor" transform="translate(%s,%s)">%s</g>`,
		ftoa(pad), ftoa(pad+b.h), b.String())
	s.WriteString(`</svg>`)
	return s.String()
}

// documentTight is document without the cosmetic padding: the baseline sits at
// y=b.h, the viewBox is exactly the box's advance width × (height+depth). It
// backs RenderSVGMetrics so callers get a box whose reported Metrics match the
// SVG one-to-one and whose baseline can be aligned with surrounding text.
func (e *engine) documentTight(b *box) string {
	w, h := b.w, b.h+b.d
	var s strings.Builder
	fmt.Fprintf(&s, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		ftoa(w), ftoa(h), ftoa(w), ftoa(h))
	fmt.Fprintf(&s, `<g fill="currentColor" transform="translate(%s,%s)">%s</g>`,
		ftoa(0), ftoa(b.h), b.String())
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
