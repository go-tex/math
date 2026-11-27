// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"math"
	"testing"

	"github.com/go-opentype/opentype"
)

// make_fraction (tex.web §743-§748) does not place the numerator and the
// denominator at the minimum gap: it STARTS from shift_up / shift_down (num2/num3
// and denom1/denom2 of the math-extension font, the OpenType MATH constants
// FractionNumerator[DisplayStyle]ShiftUp, FractionDenominator[DisplayStyle]
// ShiftDown, and for a stack StackTop[DisplayStyle]ShiftUp, StackBottom
// [DisplayStyle]ShiftDown) and moves them apart only when the clearance to the
// rule (§746) or between the two rows (§745) falls short. Measured against
// tectonic, \frac{x}{y} was 6.171/1.529 here and 6.9512/4.80951 there.
//
// The expected boxes below re-derive §745/§746 from the MATH constants and the
// metrics of the numerator/denominator rendered on their own, at the style TeX
// gives them (script for a text fraction, text for a display one).

const shiftPx = 100 // large, so the 26.6 rounding of the face is negligible

func shiftConst(t *testing.T, r *Renderer, c opentype.MathConstant) float64 {
	t.Helper()
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	return e.mc(c, shiftPx)
}

func boxOf(t *testing.T, r *Renderer, tex string) Metrics {
	t.Helper()
	_, m, err := r.RenderSVGMetrics(tex, shiftPx)
	if err != nil {
		t.Fatalf("%s: %v", tex, err)
	}
	return m
}

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.05 {
		t.Errorf("%s = %.3f, want %.3f", what, got, want)
	}
}

func TestFractionStartsFromTheShifts(t *testing.T) {
	r := newRenderer(t)
	axis := shiftConst(t, r, opentype.AxisHeight)
	rt := shiftConst(t, r, opentype.FractionRuleThickness)
	for _, tc := range []struct {
		name, pre, inner string
		display          bool
	}{
		{"text", "", `\scriptstyle `, false},
		{"display", `\displaystyle `, `\textstyle `, true},
	} {
		up, down := opentype.FractionNumeratorShiftUp, opentype.FractionDenominatorShiftDown
		gn, gd := opentype.FractionNumeratorGapMin, opentype.FractionDenominatorGapMin
		if tc.display {
			up, down = opentype.FractionNumeratorDisplayStyleShiftUp, opentype.FractionDenominatorDisplayStyleShiftDown
			gn, gd = opentype.FractionNumDisplayStyleGapMin, opentype.FractionDenomDisplayStyleGapMin
		}
		for _, c := range [][2]string{{"x", "y"}, {"x^2", "y^2"}, {"xx", "y_j"}, {"A", "g"}} {
			num, den := boxOf(t, r, tc.pre+tc.inner+c[0]), boxOf(t, r, tc.pre+tc.inner+c[1])
			su, sd := shiftConst(t, r, up), shiftConst(t, r, down)
			if d := shiftConst(t, r, gn) - ((su - num.Depth) - (axis + rt/2)); d > 0 {
				su += d
			}
			if d := shiftConst(t, r, gd) - ((axis - rt/2) - (den.Height - sd)); d > 0 {
				sd += d
			}
			m := boxOf(t, r, tc.pre+`\frac{`+c[0]+`}{`+c[1]+`}`)
			near(t, tc.name+` \frac{`+c[0]+`}{`+c[1]+`} height`, m.Height, su+num.Height)
			near(t, tc.name+` \frac{`+c[0]+`}{`+c[1]+`} depth`, m.Depth, sd+den.Depth)
			// the shift is a FLOOR: the numerator never sits below shift_up.
			if m.Height < shiftConst(t, r, up)+num.Height-0.05 {
				t.Errorf("%s \\frac{%s}{%s}: height %.3f is below shift_up + height(num)", tc.name, c[0], c[1], m.Height)
			}
		}
	}
}

func TestStackStartsFromTheShifts(t *testing.T) {
	r := newRenderer(t)
	for _, tc := range []struct {
		name, pre, inner string
		display          bool
	}{
		{"text", "", `\scriptstyle `, false},
		{"display", `\displaystyle `, `\textstyle `, true},
	} {
		up, down, gap := opentype.StackTopShiftUp, opentype.StackBottomShiftDown, opentype.StackGapMin
		if tc.display {
			up, down, gap = opentype.StackTopDisplayStyleShiftUp, opentype.StackBottomDisplayStyleShiftDown, opentype.StackDisplayStyleGapMin
		}
		for _, c := range [][2]string{{"x", "y"}, {"x^2", "y_2"}, {"A", "g"}} {
			num, den := boxOf(t, r, tc.pre+tc.inner+c[0]), boxOf(t, r, tc.pre+tc.inner+c[1])
			su, sd := shiftConst(t, r, up), shiftConst(t, r, down)
			if d := (shiftConst(t, r, gap) - ((su - num.Depth) - (den.Height - sd))) / 2; d > 0 {
				su += d
				sd += d
			}
			m := boxOf(t, r, tc.pre+`\genfrac{}{}{0pt}{}{`+c[0]+`}{`+c[1]+`}`)
			near(t, tc.name+` stack `+c[0]+`/`+c[1]+` height`, m.Height, su+num.Height)
			near(t, tc.name+` stack `+c[0]+`/`+c[1]+` depth`, m.Depth, sd+den.Depth)
		}
	}
}
