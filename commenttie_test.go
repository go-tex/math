// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"
)

// ⛔ Two more structure characters were being TYPESET as glyphs, both invisible to every
// channel this campaign measures: nothing is dropped, so the equation census cannot see
// them, and a page count rarely notices a few points. They were found by asking the
// question directly — which characters carry ink they should not — after a bare $ turned
// out to be one (go-tex/math#47).
//
//	%   latex.ltx:153, \catcode`\%=14 — a COMMENT to end of line.
//	    Measured before: "a%commentaire\nb" was 237.00 wide against "ab"'s 35.00.
//	~   latex.ltx:306, \catcode`\~=\active, and the active ~ is \nobreakspace
//	    (latex.ltx:6621) — a non-breaking INTERWORD SPACE, not a tilde.
//	    Measured before: "a~b" was 53.00 against 35.00.
//
// The tests assert WIDTHS, because the defect was width: "it renders" passed throughout.
func w(t *testing.T, r *Renderer, tex string) float64 {
	t.Helper()
	_, m, err := r.RenderSVGMetrics(tex, 32)
	if err != nil {
		t.Fatalf("render(%q): %v", tex, err)
	}
	return m.Width
}

// A comment carries no ink at all, and it swallows the rest of its line.
func TestACommentIsNotTypeset(t *testing.T) {
	r := newRenderer(t)
	plain := w(t, r, `ab`)
	for _, tex := range []string{
		"a%commentaire\nb",
		"a%b\nb",         // the b on the comment's line is gone, the next one stays
		"a%\nb",          // an empty comment
		"ab%commentaire", // a comment at the end
		"ab%",            // a bare % at the end, with no newline at all
	} {
		if got := w(t, r, tex); gomath.Abs(got-plain) > 1e-9 {
			t.Errorf("%q is %.2f wide, ab is %.2f: the comment is carrying ink", tex, got, plain)
		}
	}
	// ⛔ And a LITERAL percent must keep its ink — it is \%, which the symbol table
	// serves. Without this the fix could not be told from deleting both.
	if got := w(t, r, `a\%b`); got <= plain {
		t.Errorf(`a\%%b is %.2f wide, ab is %.2f: the literal percent was lost`, got, plain)
	}
}

// A tie is an interword space, so it is WIDER than nothing and NARROWER than a tilde
// glyph — and exactly the width the control space gives, which is the 6mu the space table
// already holds.
func TestATieIsAnInterwordSpaceAndNotATilde(t *testing.T) {
	r := newRenderer(t)
	plain, tie, ctrl := w(t, r, `ab`), w(t, r, `a~b`), w(t, r, `a\ b`)
	if gomath.Abs(tie-ctrl) > 1e-9 {
		t.Errorf(`a~b is %.2f wide, a\ b is %.2f: a tie is the control space`, tie, ctrl)
	}
	if tie <= plain {
		t.Errorf(`a~b (%.2f) is not wider than ab (%.2f): the space was dropped entirely`, tie, plain)
	}
	// It must not be the tilde glyph, which is much wider than a 6mu space.
	if sim := w(t, r, `a\sim b`); gomath.Abs(tie-sim) < 1e-9 {
		t.Errorf(`a~b is as wide as a\sim b (%.2f): the tie is still a glyph`, sim)
	}
}

// Both compose inside the constructs that lift text-mode content into a formula, which is
// where they arrive from.
func TestTheCommentAndTieComposeInsideWrappers(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		"\\scalebox{0.5}{$a%c\nb$}",
		`\raisebox{1pt}{a~b}`,
		`\text{a~b}`,
		"\\frac{a%c\nb}{c}",
		`x^{a~b}`,
	} {
		renderOK(t, r, tex)
	}
	// The scaled box no longer carries the comment's ink: half of ab.
	plain := w(t, r, `ab`)
	if got, want := w(t, r, "\\scalebox{0.5}{a%c\nb}"), 0.5*plain; gomath.Abs(got-want) > 1e-6 {
		t.Errorf("the scaled comment is %.2f wide, want %.2f (half of ab)", got, want)
	}
}
