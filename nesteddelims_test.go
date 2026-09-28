// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"
)

// A maths-mode delimiter met inside a maths source is redundant: the engine lifted
// TEXT-mode content into a string this layer reads as maths, and in the document the
// wrapper's argument genuinely is text mode. latex.ltx:11298 is
// \relax\ifmmode\@badmath\else$\fi — outside maths, \( IS $.
//
// The tests assert EQUALITY with the undelimited form, because that is the claim. A
// rendered shape would pass for a delimiter that ate its neighbour, and "it renders"
// would pass for one typeset as a glyph — which is exactly what a bare $ did.
func TestANestedMathsDelimiterIsConsumedAndNotTypeset(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	base := svg(`a+b`)
	for _, tex := range []string{
		`$a+b$`, `\(a+b\)`, `\[a+b\]`,
		`$a+b`, `a+b$`, // an unpaired one is still not an atom
		`\(a+b`, `a+b\)`,
	} {
		if got := svg(tex); got != base {
			t.Errorf("%q does not render as %q", tex, `a+b`)
		}
	}
}

// ⛔ The bare $ used to be TYPESET. $a+b$ measured 104.22 against a+b's 72.22 — 32pt of
// spurious ink that no census could see, because nothing was dropped. This pins the
// width rather than the bytes, so the number that was wrong is the number asserted.
func TestABareDollarAddsNoWidth(t *testing.T) {
	r := newRenderer(t)
	w := func(tex string) float64 {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m.Width
	}
	bare, dollared := w(`a+b`), w(`$a+b$`)
	if gomath.Abs(bare-dollared) > 1e-9 {
		t.Errorf(`$a+b$ is %.2f wide, a+b is %.2f: the dollars are being typeset`, dollared, bare)
	}
	// And a LITERAL dollar still works — it is \$, which the symbol table serves, and it
	// must keep its ink. Without this the fix could not be told from deleting both.
	if w(`\$`) <= 0 {
		t.Error(`\$ has no width: the literal dollar was lost with the delimiters`)
	}
	if w(`a\$b`) <= w(`ab`) {
		t.Errorf(`a\$b (%.2f) is not wider than ab (%.2f)`, w(`a\$b`), w(`ab`))
	}
}

// The delimiters must not eat their neighbours — the equality above would hold just as
// well if they did, on both sides of the comparison.
func TestANestedDelimiterDoesNotEatItsNeighbour(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	if svg(`$a+b$`) == svg(`a+`) {
		t.Error(`the closing $ swallowed the b`)
	}
	if svg(`\(a+b\)`) == svg(`+b`) {
		t.Error(`the opening \( swallowed the a`)
	}
}

// ⛔ \text is an ALPHABET switch in this layer, not a mode escape, so a nested $ must
// return to maths for its span and the closing $ must put the alphabet back.
// \raisebox{-2pt}{\text{\tiny$\bm\rightarrow$}} is a real corpus form.
//
// Three renders separate the three possible behaviours: dropping the pair and keeping the
// text face, dropping it and never restoring the face, and the correct swap.
func TestANestedDollarInsideTextReturnsToMaths(t *testing.T) {
	r := newRenderer(t)
	// ⚠ Byte-equality is the WRONG instrument twice over here. \text{a$x$c} against
	// \text{a}x\text{c} compares one list with three boxes and their inter-atom spacing;
	// and \text{$x$} against x differs by the extra <g> wrapper \text adds, whatever
	// the alphabet did. What separates the three possible behaviours is the WIDTH, which
	// is the glyph chosen: maths italic x is 18.000 at 32px and the text-face x is
	// 15.000.
	w := func(tex string) float64 {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m.Width
	}
	maths, text := w(`x`), w(`\text{x}`)
	if gomath.Abs(maths-text) < 1e-9 {
		t.Fatalf("x and \\text{x} are both %.3f wide: this test cannot tell the faces apart", maths)
	}
	if got := w(`\text{$x$}`); gomath.Abs(got-maths) > 1e-9 {
		t.Errorf(`\text{$x$} is %.3f wide, maths x is %.3f: the span did not return to maths`, got, maths)
	}
	if got := w(`\text{$x$}`); gomath.Abs(got-text) < 1e-9 {
		t.Errorf(`\text{$x$} is %.3f wide, the text face is %.3f: the dollars were merely deleted`, got, text)
	}
	// And the face comes BACK after the closing dollar: \text{$x$x} must be wider than
	// two maths x's would be narrower than — i.e. it is one maths x plus one TEXT x.
	if got, want := w(`\text{$x$x}`), maths+text; gomath.Abs(got-want) > 1e-9 {
		t.Errorf(`\text{$x$x} is %.3f wide, want %.3f (one maths x then one text x): `+
			`the alphabet was not restored after the closing $`, got, want)
	}
}

// The corpus forms this was found in, end to end.
func TestTheCorpusWrapperFormsRender(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\scalebox{0.5}{$a+b$}`,
		`\raisebox{1pt}{\(a+b\)}`,
		`\raisebox{-2pt}{\text{$x$}}`,
		`\scalebox{1.3}{\ensuremath{$\alpha$}}`,
	} {
		renderOK(t, r, tex)
	}
	// And the scaled box no longer carries the dollars' ink: half of a+b, not half of
	// $a+b$.
	_, plain, err := r.RenderSVGMetrics(`a+b`, 32)
	if err != nil {
		t.Fatal(err)
	}
	_, scaled, err := r.RenderSVGMetrics(`\scalebox{0.5}{$a+b$}`, 32)
	if err != nil {
		t.Fatal(err)
	}
	if want := 0.5 * plain.Width; gomath.Abs(scaled.Width-want) > 1e-6 {
		t.Errorf(`\scalebox{0.5}{$a+b$} is %.2f wide, want %.2f (half of a+b)`, scaled.Width, want)
	}
}
