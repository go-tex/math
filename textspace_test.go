// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"
)

// amstext.sty typesets \text's argument in an \hbox in TEXT mode, so a space inside it
// is interword glue and an hbox keeps it. This layer read \text as \mathrm, which
// discards every space: \text{gene }i \text{ is independent from gene } j printed as
// "geneiisindependentfromgenej" (go-tex/engine, paper 2407.19892 page 2).
//
// The test pins WIDTH, the quantity that was wrong: a space adds 6mu = 1/3 em, and a
// run of blanks adds ONE.
func TestTextKeepsItsInterwordSpaces(t *testing.T) {
	r := newRenderer(t)
	w := func(tex string) float64 {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 36)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m.Width
	}
	glued, one := w(`\text{ab}`), w(`\text{a b}`)
	if one <= glued+1 {
		t.Fatalf(`\text{a b} is %.2f wide, \text{ab} is %.2f: the space was dropped`, one, glued)
	}
	space := one - glued
	for _, tc := range []struct {
		tex  string
		want float64
	}{
		{`\text{a  b}`, glued + space},   // a run of blanks is ONE glue
		{"\\text{a\n b}", glued + space}, // so is a line end
		{`\text{a b c}`, w(`\text{abc}`) + 2*space},
		{`\text{ab }`, glued + space}, // an hbox keeps trailing glue
		{`\text{ ab}`, glued + space}, // and leading
		{`\mbox{a b}`, glued + space}, // the other text wrappers
		{`\textrm{a b}`, glued + space},
		{`\text{a{ }b}`, glued + space},                // a nested group is still text
		{`\text{$x$ $y$}`, w(`\text{$x$$y$}`) + space}, // but $…$ is maths: no space inside
		{`\text{$x + y$}`, w(`\text{$x+y$}`)},
	} {
		if got := w(tc.tex); gomath.Abs(got-tc.want) > 1e-6 {
			t.Errorf("%q is %.3f wide, want %.3f", tc.tex, got, tc.want)
		}
	}
	// Maths mode still discards them: only the text span changed.
	if a, b := w(`a b`), w(`ab`); gomath.Abs(a-b) > 1e-9 {
		t.Errorf("a b is %.3f, ab is %.3f: spaces outside \\text must stay discarded", a, b)
	}
	if a, b := w(`\mathrm{a b}`), w(`\mathrm{ab}`); gomath.Abs(a-b) > 1e-9 {
		t.Errorf(`\mathrm{a b} is %.3f, \mathrm{ab} is %.3f: \mathrm is maths and keeps no spaces`, a, b)
	}
	// A blank after a control WORD ends the word; it is not a space.
	if a, b := w(`\text{\alpha b}`), w(`\text{\alpha{}b}`); gomath.Abs(a-b) > 1e-9 {
		t.Errorf(`\text{\alpha b} is %.3f, \text{\alpha{}b} is %.3f`, a, b)
	}
}
