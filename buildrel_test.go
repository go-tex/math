// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"math"
	"strings"
	"testing"
)

// \buildrel <top> \over <base> is plain TeX's spelling of \stackrel, and latex.ltx
// gives them the SAME expansion:
//
//	:11228  \def\buildrel#1\over#2{\mathrel{\mathop{\kern\z@#2}\limits^{#1}}}
//
// So the test is an equivalence rather than a table of numbers: whatever \stackrel
// does, \buildrel must do, and a number of my own choosing would only pin this
// engine's current behaviour. Found by the dropped-equation census — 9 equations on
// one arXiv paper, which the page count does not notice.
func TestBuildrelIsStackrel(t *testing.T) {
	r := newRenderer(t)
	for _, c := range [][2]string{
		// The classic idiom, and the one a first version refused: \rm is a DECLARATIVE
		// font switch, honoured by parseList and not by a bare parseAtom loop.
		{`\buildrel \rm def \over =`, `\stackrel{\rm def}{=}`},
		{`a \buildrel x \over \to b`, `a \stackrel{x}{\to} b`},
		{`a \buildrel \sim \over = b`, `a \stackrel{\sim}{=} b`},
		// A fraction before it, so the atom spacing around a Rel is exercised too:
		// \buildrel must be a relation, as \mathrel in the definition says.
		{`\frac{a}{b} \buildrel p \over \to c`, `\frac{a}{b} \stackrel{p}{\to} c`},
		// A braced top, where the delimiter scan must not stop inside the group.
		{`\buildrel {a \over b} \over =`, `\stackrel{a \over b}{=}`},
	} {
		_, got, err := r.RenderSVGMetrics(c[0], 10)
		if err != nil {
			t.Errorf("%s: %v", c[0], err)
			continue
		}
		_, want, err := r.RenderSVGMetrics(c[1], 10)
		if err != nil {
			t.Fatalf("%s: %v", c[1], err)
		}
		if math.Abs(got.Width-want.Width) > 0.001 ||
			math.Abs(got.Height-want.Height) > 0.001 ||
			math.Abs(got.Depth-want.Depth) > 0.001 {
			t.Errorf("%s = %.3f/%.3f/%.3f, %s gives %.3f/%.3f/%.3f",
				c[0], got.Width, got.Height, got.Depth,
				c[1], want.Width, want.Height, want.Depth)
		}
	}
}

// \buildrel with no \over is an error and not a silent guess. The whole point of the
// delimited parameter is that the argument's end is stated; inventing one would put
// the top of the stack somewhere the author never asked for.
func TestBuildrelWithoutOverIsRefused(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{`\buildrel x`, `\buildrel`, `a \buildrel b c`} {
		if _, _, err := r.RenderSVGMetrics(tex, 10); err == nil {
			t.Errorf("%s rendered without error", tex)
		} else if !strings.Contains(err.Error(), `\over`) {
			t.Errorf("%s: error %q does not name \\over", tex, err)
		}
	}
}

// Every way the two commands can fail on their ARGUMENTS, each reported rather than
// swallowed. Exercised because a parse error that reaches the engine drops one
// equation, while a parse error that is silently absorbed can drop a document: the
// remaining tokens are then read in the wrong state.
func TestBuildrelAndLefteqnReportArgumentErrors(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct{ tex, want string }{
		// the base is missing altogether
		{`\buildrel x \over`, "expected an atom"},
		{`\lefteqn`, "expected an atom"},
		// the TOP does not parse: parseList's error has to come back out
		{`\buildrel \unknowncmd \over =`, `unknown command \unknowncmd`},
		{`\buildrel \frac{a \over =`, "missing }"},
		// and the argument of \lefteqn
		{`\lefteqn{\sqrt[}`, `unexpected "}"`},
	} {
		_, _, err := r.RenderSVGMetrics(c.tex, 10)
		if err == nil {
			t.Errorf("%s rendered without error", c.tex)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not contain %q", c.tex, err, c.want)
		}
	}
}

// \lefteqn{X} sets X in DISPLAY style at ZERO width, so it overhangs to the right and
// what follows sets over it — eqnarray's way of starting a long left-hand side
// (latex.ltx:11392, \def\lefteqn#1{\rlap{$\displaystyle #1$}}).
//
// Two properties, because either alone can hold while the other is wrong: the width
// is zero, and the vertical extent is the content's rather than nothing.
func TestLefteqnIsZeroWidthAndKeepsItsExtent(t *testing.T) {
	r := newRenderer(t)
	_, plain, err := r.RenderSVGMetrics(`abc`, 10)
	if err != nil {
		t.Fatal(err)
	}
	_, lap, err := r.RenderSVGMetrics(`\lefteqn{abc}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if lap.Width != 0 {
		t.Errorf(`\lefteqn width = %.3f, want 0`, lap.Width)
	}
	if math.Abs(lap.Height-plain.Height) > 0.001 || math.Abs(lap.Depth-plain.Depth) > 0.001 {
		t.Errorf(`\lefteqn extent %.3f/%.3f, the content alone is %.3f/%.3f`,
			lap.Height, lap.Depth, plain.Height, plain.Depth)
	}
}

// The display style is not decoration. \lefteqn exists to be used inside eqnarray,
// whose cells are display style, and a fraction inside it must be set at display
// size — dropping to text style would shrink the very thing \lefteqn usually wraps.
func TestLefteqnSetsItsContentInDisplayStyle(t *testing.T) {
	r := newRenderer(t)
	_, disp, err := r.RenderSVGMetrics(`\displaystyle\frac{a}{b}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	_, lap, err := r.RenderSVGMetrics(`\lefteqn{\frac{a}{b}}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(lap.Height-disp.Height) > 0.001 || math.Abs(lap.Depth-disp.Depth) > 0.001 {
		t.Errorf(`\lefteqn{\frac{a}{b}} extent %.3f/%.3f, display style gives %.3f/%.3f`,
			lap.Height, lap.Depth, disp.Height, disp.Depth)
	}
	// And the control that makes the assertion mean something: text style differs.
	_, text, err := r.RenderSVGMetrics(`\frac{a}{b}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(text.Height-disp.Height) < 0.001 && math.Abs(text.Depth-disp.Depth) < 0.001 {
		t.Skip("display and text style agree on this fraction, so this test proves nothing")
	}
}
