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
