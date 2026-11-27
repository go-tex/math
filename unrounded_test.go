// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"

	"github.com/go-opentype/opentype"
)

// The engine lays out at px = the point size, so a MATH value rounded to a whole
// pixel is rounded to a whole point. These pin that nothing is: every read below is
// asserted at 10px, where STIX Two Math's numbers are fractional (AxisHeight 258 of
// 1000 em is 2.58, and rounds to 3). A face at scale 1.0 could not tell.
func TestMATHValuesAreReadUnrounded(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}

	if got := e.mc(opentype.AxisHeight, 10); gomath.Abs(got-2.58) > 1e-9 {
		t.Errorf("mc(AxisHeight, 10) = %v, want 2.58 (the rounded read gave 3)", got)
	}
	// The percentage constants are ratios and must come back verbatim.
	if got := e.mc(opentype.ScriptPercentScaleDown, 10); got != float64(e.face(10).MathConstant(opentype.ScriptPercentScaleDown)) || got < 1 {
		t.Errorf("mc(ScriptPercentScaleDown) = %v, want the verbatim percentage", got)
	}
	// A glyph's advance: x at 10px is not a whole number of pixels.
	b, ok := e.glyphBox('x', 10, clsOrd)
	if !ok {
		t.Fatal("no glyph for x")
	}
	want := float64(e.face(10).AdvanceUnits('x'))
	if gomath.Abs(b.w-want) > 1e-9 || b.w == gomath.Trunc(b.w) {
		t.Errorf("glyph width = %v, want the unrounded %v", b.w, want)
	}
}

// A variant is taken when its UNROUNDED advance reaches the target. STIX's first two
// parenthesis variants advance 9.33 and 11.87 at 10px; the rounded forms (9, 12)
// made a 11.9 target look reached by the second.
func TestVariantIsChosenOnItsUnroundedAdvance(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	gid, _ := e.font.GlyphIndex('(')
	vs, _ := e.face(10).MathVariantsUnits(gid, true)
	if len(vs) < 3 || gomath.Abs(vs[1].Advance-11.87) > 0.001 {
		t.Fatalf("variants changed: %+v — re-derive the targets", vs)
	}
	reached := e.stretchVertical('(', 11.8, 10, clsOpen)
	past := e.stretchVertical('(', 11.9, 10, clsOpen)
	if past.h+past.d < reached.h+reached.d+1 {
		t.Errorf("target 11.9 gave %.2f, target 11.8 gave %.2f: 11.87 does not reach 11.9, so the next variant "+
			"(14.27) must be taken", past.h+past.d, reached.h+reached.d)
	}
}

// tex.web §762 (make_left_right): the target is max(delta1/500*901, 2*delta1-5pt).
func TestDelimiterTargetIsSection762(t *testing.T) {
	for _, c := range []struct{ delta1, want float64 }{
		{6, 10.812}, // the factor wins: 1.802 x 6
		{20, 36.04}, // still the factor: 36.04 against 35
		{30, 55},    // the shortfall wins: 60 - 5 against 54.06
		{100, 195},  // far out the shortfall always wins
		{0, 0},      // nothing to span
	} {
		if got := delimiterTarget(c.delta1); gomath.Abs(got-c.want) > 1e-9 {
			t.Errorf("delimiterTarget(%v) = %v, want %v", c.delta1, got, c.want)
		}
	}
}
