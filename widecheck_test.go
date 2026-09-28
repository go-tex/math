// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"
)

// \widecheck is stix.sty's stretchy caron, U+030C in unicode-math-table.tex and the same
// mathaccentwide class as \widehat (U+0302) and \widetilde (U+0303) — both of which this
// table already serves with a FIXED glyph.
//
// 97 equations over 2 papers (go-tex/engine#466). It was deliberately left out of the
// symbol sweep (go-tex/math#41) because an accent is not a symbol-table entry; it belongs
// here, in the accent table, which is where it now is.
func TestWidecheckIsTheCaronAndPairsWithCheck(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	// On a single character — 91% of the corpus uses — it is exactly \check, because
	// there is nothing to stretch over. That equality is the claim.
	if got, want := svg(`\widecheck{S}`), svg(`\check{S}`); got != want {
		t.Error(`\widecheck{S} does not set as \check{S}`)
	}
	// And it is an ACCENT, not a no-op: the box must be taller than the bare letter.
	_, m1, err := r.RenderSVGMetrics(`\widecheck{S}`, 32)
	if err != nil {
		t.Fatal(err)
	}
	_, m2, err := r.RenderSVGMetrics(`S`, 32)
	if err != nil {
		t.Fatal(err)
	}
	if m1.Height <= m2.Height {
		t.Errorf(`\widecheck{S} is %.2f high, S alone %.2f: no accent was placed`, m1.Height, m2.Height)
	}
	// The width is the nucleus's: an accent does not widen its box.
	if gomath.Abs(m1.Width-m2.Width) > 1e-9 {
		t.Errorf(`\widecheck{S} is %.2f wide, S alone %.2f`, m1.Width, m2.Width)
	}
}

// ⚠ The wide forms are not stretched, and this pins that as a KNOWN state rather than
// leaving it to be discovered: over a wide nucleus the caron is the same size as over a
// narrow one. \widehat has always behaved this way; \widecheck joins it on those terms.
//
// It is asserted so that the day the family is stretched, this test fails and says where
// the assumption lived.
func TestTheWideAccentsAreNotStretchedYet(t *testing.T) {
	r := newRenderer(t)
	accHeight := func(tex, bare string) float64 {
		t.Helper()
		_, a, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		_, b, err := r.RenderSVGMetrics(bare, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", bare, err)
		}
		return a.Height - b.Height
	}
	for _, c := range []struct{ wide, narrow, bareW, bareN string }{
		{`\widecheck{CM}`, `\widecheck{C}`, `CM`, `C`},
		{`\widehat{CM}`, `\widehat{C}`, `CM`, `C`},
		{`\widetilde{CM}`, `\widetilde{C}`, `CM`, `C`},
	} {
		w := accHeight(c.wide, c.bareW)
		n := accHeight(c.narrow, c.bareN)
		if gomath.Abs(w-n) > 1e-9 {
			t.Errorf("%s adds %.3f above its nucleus and %s adds %.3f: "+
				"a wide accent is now size-dependent — if the family was stretched, "+
				"update this test and the comment beside the accents table",
				c.wide, w, c.narrow, n)
		}
	}
}

// It composes where the papers write it.
func TestWidecheckComposes(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\widecheck{S}`, `\widecheck S`, `\widecheck{S\cup Z}`, `\widecheck{CM}`,
		`\widecheck{S}' = \widecheck{m}`, `x^{\widecheck{S}}`, `\frac{\widecheck{S}}{2}`,
	} {
		renderOK(t, r, tex)
	}
}
