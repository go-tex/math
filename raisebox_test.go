// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"regexp"
	"strconv"
	"testing"
)

// The tests assert RATIOS and SIGNS against the unshifted box, and read the emitted
// transform where only the transform can answer. A measured number would pin today's
// font; "shifted up by exactly the lift" is what \raisebox means.

var transformY = regexp.MustCompile(`translate\(([-0-9.]+),([-0-9.]+)\)`)

// firstTranslateY returns the dy of the OUTERMOST translate in the SVG.
func firstTranslateY(t *testing.T, svg string) float64 {
	t.Helper()
	m := transformY.FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("no translate in %.120s", svg)
	}
	v, err := strconv.ParseFloat(m[2], 64)
	if err != nil {
		t.Fatalf("dy %q: %v", m[2], err)
	}
	return v
}

// ⛔ The sign. Y runs DOWNWARD in this coordinate system, so a raise is a NEGATIVE dy.
// Every extent assertion below passes with the sign reversed — only the transform says
// which way the ink actually went, which is the same gap \reflectbox left open.
func TestARaiseboxShiftsUpwardsNotDownwards(t *testing.T) {
	r := newRenderer(t)
	svg, _, err := r.RenderSVGMetrics(`\raisebox{10pt}{x}`, 32)
	if err != nil {
		t.Fatal(err)
	}
	// The outermost translate belongs to the document wrapper; find the raisebox's by
	// asking for the box's own extents instead, which is what the next tests do. Here
	// we only need that SOME translate carries −10.
	var found bool
	for _, m := range transformY.FindAllStringSubmatch(svg, -1) {
		if v, err := strconv.ParseFloat(m[2], 64); err == nil && gomath.Abs(v+10) < 1e-9 {
			found = true
		}
	}
	if !found {
		t.Errorf(`\raisebox{10pt} emitted no translate with dy = -10: the shift is absent or DOWNWARD`)
	}
	svgDown, _, err := r.RenderSVGMetrics(`\raisebox{-10pt}{x}`, 32)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range transformY.FindAllStringSubmatch(svgDown, -1) {
		if v, err := strconv.ParseFloat(m[2], 64); err == nil && gomath.Abs(v-10) < 1e-9 {
			return
		}
	}
	t.Error(`\raisebox{-10pt} emitted no translate with dy = +10: a negative lift must LOWER`)
}

// latex.ltx raises inside an \hbox and never touches the width: h+lift, d−lift, w
// unchanged. The depth GOING NEGATIVE for a large lift is the source's behaviour, not
// an oversight, so it is asserted rather than clamped.
func TestARaiseboxMovesTheExtentsAndNotTheWidth(t *testing.T) {
	r := newRenderer(t)
	m := func(tex string) Metrics {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m
	}
	base := m(`x`)
	up := m(`\raisebox{10pt}{x}`)
	if gomath.Abs(up.Width-base.Width) > 1e-9 {
		t.Errorf("width %v, want %v unchanged", up.Width, base.Width)
	}
	if gomath.Abs(up.Height-(base.Height+10)) > 1e-9 {
		t.Errorf("height %v, want %v", up.Height, base.Height+10)
	}
	// ⛔ The depth goes NEGATIVE inside raiseBox and hlist then CLAMPS it at 0, so the
	// public API cannot show it. Asserting base.Depth-10 here fails for the right
	// reason and the wrong subject; the negative is asserted on raiseBox directly, in
	// TestRaiseBoxItselfCarriesANegativeDepth below. What the API must show is that a
	// raise does not INVENT depth.
	if up.Depth > base.Depth+1e-9 {
		t.Errorf("depth %v grew from %v: a raise must not add depth", up.Depth, base.Depth)
	}
	down := m(`\raisebox{-10pt}{x}`)
	if gomath.Abs(down.Depth-(base.Depth+10)) > 1e-9 {
		t.Errorf("lowered depth %v, want %v", down.Depth, base.Depth+10)
	}
}

// latex.ltx:11923 REPLACES the height with the declared one; it does not add to it.
// And with ONE optional argument only the height is replaced — the depth keeps what the
// raise left it. That asymmetry between \@irsbox and \@iirsbox is the easiest clause to
// flatten by accident.
func TestADeclaredHeightReplacesAndOneBracketLeavesTheDepthAlone(t *testing.T) {
	r := newRenderer(t)
	m := func(tex string) Metrics {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m
	}
	base := m(`x`)
	oneOpt := m(`\raisebox{10pt}[3pt]{x}`)
	if gomath.Abs(oneOpt.Height-3) > 1e-9 {
		t.Errorf("height %v, want exactly 3 (REPLACED, not 3 added to the raise)", oneOpt.Height)
	}
	// One bracket must not touch the depth — asserted on raiseBox, because hlist
	// clamps the list's depth at zero and the API cannot distinguish "left alone at
	// −9.68" from "set to 0".
	r3 := newRenderer(t)
	e := &engine{font: r3.font, upem: float64(r3.font.UnitsPerEm()), gc: r3.gc}
	x := e.mustGlyph('x', 32, clsOrd)
	one := e.raiseBox(x, 10, 3, 0, true, false)
	if want := x.d - 10; gomath.Abs(one.d-want) > 1e-9 {
		t.Errorf("one bracket: depth %v, want %v (untouched by the raise)", one.d, want)
	}
	if gomath.Abs(one.h-3) > 1e-9 {
		t.Errorf("one bracket: height %v, want exactly 3", one.h)
	}
	two := e.raiseBox(x, 10, 3, 4, true, true)
	if gomath.Abs(two.d-4) > 1e-9 {
		t.Errorf("two brackets: depth %v, want exactly 4", two.d)
	}
	twoOpt := m(`\raisebox{10pt}[3pt][4pt]{x}`)
	if gomath.Abs(twoOpt.Height-3) > 1e-9 || gomath.Abs(twoOpt.Depth-4) > 1e-9 {
		t.Errorf("two-option form = h %v d %v, want 3 and 4", twoOpt.Height, twoOpt.Depth)
	}
	// An EMPTY bracket is not a zero: \@irsbox is called with [] when none was
	// written, and \ifx\\#2\\ then leaves the extent alone.
	empty := m(`\raisebox{10pt}[]{x}`)
	if want := base.Height + 10; gomath.Abs(empty.Height-want) > 1e-9 {
		t.Errorf("[] gave height %v, want %v: an empty bracket must not set zero", empty.Height, want)
	}
}

// The papers write ex and pt, positive and negative — \raisebox{.7ex}, {8pt},
// {-0.65ex}, {-2pt}. 1ex must be MEASURED (the height of x), not a guessed fraction of
// the em, which is what e.dimen already does; this pins that \raisebox goes through it.
func TestARaiseboxTakesTheUnitsThePapersWrite(t *testing.T) {
	r := newRenderer(t)
	m := func(tex string) Metrics {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m
	}
	base := m(`x`)
	// ⛔ 1ex is NOT the height of the rendered x. e.dimen measures the UPRIGHT glyph
	// (15.136 at 32px) while maths sets x ITALIC (15.328) — a 1.3% difference, and
	// asserting "twice as high" conflates the two. The right comparison is against
	// e.dimen itself, which is the quantity \raisebox is specified in terms of.
	r2 := newRenderer(t)
	e := &engine{font: r2.font, upem: float64(r2.font.UnitsPerEm()), gc: r2.gc}
	sty := style{px: 32, display: true, spacious: true}
	ex := e.dimen("1ex", sty)
	if ex <= 0 {
		t.Fatalf("1ex measured as %v", ex)
	}
	oneEx := m(`\raisebox{1ex}{x}`)
	if want := base.Height + ex; gomath.Abs(oneEx.Height-want) > 1e-6 {
		t.Errorf(`\raisebox{1ex}{x} height %v, want %v (x plus one measured ex)`, oneEx.Height, want)
	}
	// And the unit is honoured, not ignored: 2ex must lift twice as far as 1ex.
	twoEx := m(`\raisebox{2ex}{x}`)
	if want := base.Height + 2*ex; gomath.Abs(twoEx.Height-want) > 1e-6 {
		t.Errorf(`\raisebox{2ex}{x} height %v, want %v`, twoEx.Height, want)
	}
	for _, tex := range []string{
		`\raisebox{.7ex}{x}`, `\raisebox{8pt}{x}`, `\raisebox{-0.65ex}{x}`,
		`\raisebox{-2pt}{x}`, `\raisebox{0.2ex}{x}`, `\raisebox{1em}{x}`,
	} {
		renderOK(t, r, tex)
	}
}

// A missing lift is an ERROR, not a zero shift: latex.ltx declares \raisebox[1]. Zero
// would silently set the content on the baseline, which is what the strip-and-keep
// fallback this replaces already did — so the error is the only thing that
// distinguishes the new path from the old one on malformed input.
func TestARaiseboxWithoutALiftIsAnError(t *testing.T) {
	r := newRenderer(t)
	if _, err := r.RenderSVG(`\raisebox x`, 32); err == nil {
		t.Error(`\raisebox with no {lift} did not fail`)
	}
	// A malformed CONTENT argument propagates its error rather than being swallowed:
	// an unclosed brace must not silently produce an empty raised box.
	if _, err := r.RenderSVG(`\raisebox{2pt}{`, 32); err == nil {
		t.Error(`\raisebox{2pt}{ with an unclosed content brace did not fail`)
	}
	// An unclosed [ is NOT an optional argument: it is the character it is, and the
	// content follows it. Reading it as one would scan to the end of the formula
	// looking for a ]. All four of these render rather than erroring.
	for _, tex := range []string{
		`\raisebox{2pt}[3pt{x}`,
		`\raisebox{2pt}[3pt`,
		`\raisebox{2pt}[{x}`,
		`\raisebox{2pt}[3pt][4pt{x}`,
	} {
		if _, err := r.RenderSVG(tex, 32); err != nil {
			t.Errorf("%s: %v — an unclosed [ is an ordinary character, not a failure", tex, err)
		}
	}
}

// It composes: a raisebox inside a script, inside a fraction, and nested.
func TestARaiseboxComposes(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`x^{\raisebox{1pt}{a}}`,
		`\frac{\raisebox{2pt}{a}}{b}`,
		`\raisebox{2pt}{\raisebox{2pt}{a}}`,
		`\raisebox{2pt}{$a+b$}`,
		`a\raisebox{1pt}{b}c`,
	} {
		renderOK(t, r, tex)
	}
}

// The negative depth lives in raiseBox and cannot be seen through the public API,
// because hlist clamps a list's depth at zero. Asserted at the level where it exists:
// \hbox{\raise l \box} has depth d−l, and TeX permits that to be negative.
func TestRaiseBoxItselfCarriesANegativeDepth(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	x := e.mustGlyph('x', 32, clsOrd)
	up := e.raiseBox(x, 10, 0, 0, false, false)
	if want := x.d - 10; gomath.Abs(up.d-want) > 1e-9 {
		t.Errorf("raiseBox depth = %v, want %v", up.d, want)
	}
	if up.d >= 0 {
		t.Errorf("raiseBox depth = %v, want negative: the bottom edge is above the baseline", up.d)
	}
	if want := x.h + 10; gomath.Abs(up.h-want) > 1e-9 {
		t.Errorf("raiseBox height = %v, want %v", up.h, want)
	}
	if up.w != x.w {
		t.Errorf("raiseBox width = %v, want %v unchanged", up.w, x.w)
	}
	// And hlist is what hides it — pinned so the clamp is a known property and not a
	// surprise the next reader re-derives.
	l := e.hlist([]*box{up}, style{px: 32, spacious: true})
	if l.d != 0 {
		t.Errorf("hlist depth = %v, want 0: the clamp is the reason the API cannot show the negative", l.d)
	}
}
