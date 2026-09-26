// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"math"
	"testing"
)

// TestExAtMatchesTheReference pins \ex@ against the reference engine at fifteen
// sizes, spanning both branches of \compute@ex@ and the saturation above 20pt.
//
// Measured with `measure dimens` (go-tex/measure), which reads the register out of
// tectonic's log rather than inferring it from a rendered total:
//
//	\fontsize{S}{12}\selectfont $ $ \dimen101=\ex@
//
// The three sizes a LaTeX class actually selects are 10.00, 10.95 and 12.00; the
// others are there because a transcription that is right at one point and wrong in
// its shape would pass on three.
func TestExAtMatchesTheReference(t *testing.T) {
	for _, c := range []struct{ size, want float64 }{
		{5, 0.7374}, {6, 0.7837}, {7, 0.8329}, {8, 0.8853}, {9, 0.9409},
		{10, 1.0000}, {10.95, 1.0591}, {12, 1.1147}, {14.4, 1.2398},
		{17.28, 1.3668}, {19, 1.4222}, {20, 1.4563},
		{20.74, 1.5000}, {24.88, 1.5000}, {30, 1.5000},
	} {
		// The reference prints four decimals, so the tolerance is the printing and
		// nothing more: a transcription that merely approximates \compute@ex@ would
		// need a looser one, and that is the point of the bound.
		if v := exAt(c.size); math.Abs(v-c.want) > 0.0001 {
			t.Errorf("exAt(%g) = %.4f, tectonic gives %.4f", c.size, v, c.want)
		}
	}
}

// TestExAtIsNotProportional states the property the old model assumed. smallmatrix
// was separated by a gap proportional to the cell size; \ex@ is neither
// proportional to the size nor constant, so no single ratio can stand in for it.
func TestExAtIsNotProportional(t *testing.T) {
	if r10, r20 := exAt(10)/10, exAt(20)/20; math.Abs(r10-r20) < 0.02 {
		t.Errorf("ex@/size: %.4f at 10pt and %.4f at 20pt - too close for this test "+
			"to tell a proportional stand-in from the real function", r10, r20)
	}
	if exAt(10) == exAt(20) {
		t.Error("ex@ is constant, so this test could not tell a constant from the real function")
	}
}
