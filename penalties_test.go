// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"
)

// A penalty carries no ink and no width. This layer does not break formulas, so the
// ONLY correct behaviour is to change nothing at all — and the only test that says so
// is byte-equality, because "it renders" would pass for a penalty that quietly
// inserted a kern, and "it does not error" would pass for one that swallowed the
// following atom.
func TestAPenaltyChangesNothingAtAll(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	for _, c := range []struct{ with, without string }{
		{`a+\allowbreak b`, `a+b`},
		{`a+\nobreak b`, `a+b`},
		{`a+\break b`, `a+b`},
		{`\allowbreak x`, `x`},
		{`x\allowbreak`, `x`},
		{`\frac{a\allowbreak +b}{c}`, `\frac{a+b}{c}`},
		{`x^{a\allowbreak b}`, `x^{ab}`},
	} {
		if got, want := svg(c.with), svg(c.without); got != want {
			t.Errorf("%q and %q do not render identically", c.with, c.without)
		}
	}
}

// ⛔ The equality above would also hold if the penalty ATE its neighbour on both
// sides of the comparison. This pins that the surrounding material is still there,
// by comparing against a formula that is genuinely different.
func TestAPenaltyDoesNotEatItsNeighbour(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	if svg(`a+\allowbreak b`) == svg(`a+`) {
		t.Error(`\allowbreak swallowed the b`)
	}
	if svg(`a\allowbreak +b`) == svg(`a b`) {
		t.Error(`\allowbreak swallowed the +`)
	}
}

// latex.ltx:11200 is \DeclareRobustCommand\mathstrut{\vphantom(}: the height and
// depth of a parenthesis, and ZERO width. All three properties are asserted, because
// each one is a different way to get it wrong — a \phantom( would take the width, an
// \hphantom( would take neither extent, and forgetting the ( would give an empty box.
func TestMathstrutIsAVphantomParenthesis(t *testing.T) {
	r := newRenderer(t)
	m := func(tex string) Metrics {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m
	}
	strut, vph := m(`\mathstrut`), m(`\vphantom{(}`)
	if gomath.Abs(strut.Width-vph.Width) > 1e-9 ||
		gomath.Abs(strut.Height-vph.Height) > 1e-9 ||
		gomath.Abs(strut.Depth-vph.Depth) > 1e-9 {
		t.Errorf(`\mathstrut = %+v, \vphantom{(} = %+v`, strut, vph)
	}
	// Zero width, and a real vertical extent: the parenthesis is taller than an x.
	if strut.Width != 0 {
		t.Errorf(`\mathstrut width = %v, want 0`, strut.Width)
	}
	if x := m(`x`); strut.Height <= x.Height {
		t.Errorf(`\mathstrut height %v is not above x's %v — the ( is missing`,
			strut.Height, x.Height)
	}
}

// And it DOES its job: a \mathstrut raises the height of the list it sits in, which
// is the only reason to write one. A no-op implementation passes every test above
// except this one.
func TestMathstrutRaisesTheListItSitsIn(t *testing.T) {
	r := newRenderer(t)
	h := func(tex string) float64 {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m.Height
	}
	if with, without := h(`x\mathstrut`), h(`x`); with <= without {
		t.Errorf(`x\mathstrut is %v high, x alone is %v: the strut does nothing`,
			with, without)
	}
	// Zero width means it must not make the list WIDER.
	_, a, _ := r.RenderSVGMetrics(`x\mathstrut`, 32)
	_, b, _ := r.RenderSVGMetrics(`x`, 32)
	if gomath.Abs(a.Width-b.Width) > 1e-9 {
		t.Errorf(`x\mathstrut is %v wide, x alone %v: the strut took width`, a.Width, b.Width)
	}
}

// ⛔ The same mistake was already in the tree, for ten OTHER commands. parseControl
// handled \relax, \protect, \leavevmode, \noindent, \ignorespaces, \par, \normalfont,
// \normalcolor, \limits and \nolimits by returning newBox(clsOrd) — an empty ORDINARY
// ATOM, which the spacing machinery counts. All ten made "a+X b" set differently from
// "a+b", and every one of those equations rendered, with the wrong inter-atom space.
// Nothing objected, because no test compared the two and a page count cannot see a
// thin space.
//
// This walks the whole transparent list against a formula that does not contain it.
// It is the generalisation of the penalty test above, and it is the test that should
// have existed when that list was written.
func TestEveryTransparentCommandIsActuallyTransparent(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	// a+X b against a+b: the Bin class of + is what an intruding Ord atom disturbs,
	// so this placement is the one that shows the defect.
	base := svg(`a+b`)
	for _, n := range []string{
		"allowbreak", "nobreak", "break",
		"limits", "nolimits", "displaylimits",
		"normalfont", "rmfamily", "sffamily", "ttfamily", "bfseries", "mdseries",
		"itshape", "slshape", "scshape", "upshape", "em", "normalcolor",
		"normalsize", "small", "footnotesize", "scriptsize", "tiny",
		"large", "Large", "LARGE", "huge", "Huge",
		"par", "relax", "protect", "leavevmode", "noindent", "ignorespaces",
	} {
		if got := svg(`a+\` + n + ` b`); got != base {
			t.Errorf(`a+\%s b does not render as a+b: it is not transparent`, n)
		}
	}
	// And it must still be transparent where the neighbour is an Ord, an Open, or a
	// script — the classes whose spacing rules differ.
	for _, c := range []struct{ with, without string }{
		{`x\relax y`, `xy`},
		{`\relax (x)`, `(x)`},
		{`x^{2\relax }`, `x^{2}`},
		{`\sin\relax x`, `\sin x`},
	} {
		if svg(c.with) != svg(c.without) {
			t.Errorf("%q does not render as %q", c.with, c.without)
		}
	}
}
