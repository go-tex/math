// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// The three Unicode maths alphabets ISO 80000-2 asks for, which isomath selects
// through \vectorsym, \matrixsym and \tensorsym. The letters are asserted by CODE
// POINT rather than through a rendered width: two faces can share a width, and a
// width says nothing about which glyph was chosen.
//
// Names as unicode-math spells them, with the aliases isomath keeps:
//
//	\mathbfit     \mathbold        bold italic            isomath.sty:184, :189
//	\mathsfit     \mathsans        sans-serif italic      isomath.sty:200
//	\mathsfbfit   \mathboldsans    sans-serif bold italic isomath.sty:218, :219
//
// Digits are the part worth pinning: Unicode has no italic digits at all, so each
// italic alphabet falls back to its own UPRIGHT digit block — bold digits for bold
// italic, sans-serif digits for sans-serif italic, sans-serif bold for sans-serif
// bold italic. Getting that wrong shows up only on a formula that mixes a digit into
// a symbol, which is exactly what isomath's \vectorsym is written to handle.
func TestTheISOMathAlphabetsMapToTheirOwnBlocks(t *testing.T) {
	for _, c := range []struct {
		name            string
		upA, lowA, dig0 rune
	}{
		{"mathbfit", 0x1D468, 0x1D482, 0x1D7CE},
		{"mathbold", 0x1D468, 0x1D482, 0x1D7CE},
		{"boldsymbol", 0x1D468, 0x1D482, 0x1D7CE},
		{"mathsfit", 0x1D608, 0x1D622, 0x1D7E2},
		{"mathsans", 0x1D608, 0x1D622, 0x1D7E2},
		{"mathsfbfit", 0x1D63C, 0x1D656, 0x1D7EC},
		{"mathboldsans", 0x1D63C, 0x1D656, 0x1D7EC},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := alphabetFor(c.name)
			if m == nil {
				t.Fatalf("%s has no alphabet", c.name)
			}
			for _, k := range []struct {
				in, want rune
				what     string
			}{
				{'A', c.upA, "capital A"},
				{'Z', c.upA + 25, "capital Z"},
				{'a', c.lowA, "small a"},
				{'z', c.lowA + 25, "small z"},
				{'0', c.dig0, "digit 0"},
				{'9', c.dig0 + 9, "digit 9"},
			} {
				if got := m(k.in); got != k.want {
					t.Errorf("%s: %s -> U+%04X, want U+%04X", c.name, k.what, got, k.want)
				}
			}
		})
	}
}

// Every letter and digit of the three blocks is present in the shipped face. Checked
// because mapping to a code point the face does not carry trades a dropped equation
// for a notdef box, which is worse: the formula then renders, wrongly, and no census
// of refused equations sees it.
func TestTheISOMathBlocksAreCoveredByTheFace(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	for _, b := range []struct {
		name string
		base rune
		n    int
	}{
		{"bold italic capitals", 0x1D468, 26},
		{"bold italic smalls", 0x1D482, 26},
		{"sans-serif italic capitals", 0x1D608, 26},
		{"sans-serif italic smalls", 0x1D622, 26},
		{"sans-serif bold italic capitals", 0x1D63C, 26},
		{"sans-serif bold italic smalls", 0x1D656, 26},
		{"bold digits", 0x1D7CE, 10},
		{"sans-serif digits", 0x1D7E2, 10},
		{"sans-serif bold digits", 0x1D7EC, 10},
	} {
		for i := 0; i < b.n; i++ {
			if _, ok := e.font.GlyphIndex(b.base + rune(i)); !ok {
				t.Errorf("%s: U+%04X is absent from the face", b.name, b.base+rune(i))
			}
		}
	}
}

// The three faces must be DISTINGUISHABLE in the output, not merely mapped. A
// mapping table can be right while the renderer collapses two of them onto the same
// glyphs, and the alphabet test above would still pass.
func TestTheISOMathAlphabetsRenderDifferently(t *testing.T) {
	r := newRenderer(t)
	seen := map[string]string{}
	for _, name := range []string{"mathbfit", "mathsfit", "mathsfbfit"} {
		svg, err := r.RenderSVG(`\`+name+`{Axz}`, 10)
		if err != nil {
			t.Fatalf(`\%s: %v`, name, err)
		}
		if other, dup := seen[svg]; dup {
			t.Errorf(`\%s renders identically to \%s`, name, other)
		}
		seen[svg] = name
	}
}
