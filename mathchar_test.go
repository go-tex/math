// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// fontmath.ltx gives every ASCII character its maths class and its glyph, one
// \DeclareMathSymbol per line. Two of them name a glyph other than the character
// typed, both in family "symbols" (cmsy):
//
//	\DeclareMathSymbol{*}{\mathbin}{symbols}{"03} % \ast   fontmath.ltx:150
//	\DeclareMathSymbol{-}{\mathbin}{symbols}{"00}          fontmath.ltx:153
//
// cmsy slot 0 is MINUS SIGN, slot 3 ASTERISK OPERATOR. Set from ASCII instead, a
// formula got a text hyphen and a raised typographic asterisk — reported from the
// go-tex playground on \[ \int_0^\infty e^{-x^2}\,dx \].
func TestMathCharMapsMinusAndAsterisk(t *testing.T) {
	for in, want := range map[rune]rune{'-': 0x2212, '*': 0x2217, 'a': 'a', '+': '+'} {
		if got := mathChar(in); got != want {
			t.Errorf("mathChar(%q) = %U, want %U", in, got, want)
		}
	}
}

// `*` must set exactly as \ast, because fontmath.ltx points them at the same cmsy
// slot — the comment on line 150 says so in as many words.
func TestAsteriskIsAst(t *testing.T) {
	r := newRenderer(t)
	star, _, err := r.RenderSVGMetrics(`a*b`, 10)
	if err != nil {
		t.Fatalf("a*b: %v", err)
	}
	ast, _, err := r.RenderSVGMetrics(`a\ast b`, 10)
	if err != nil {
		t.Fatalf(`a\ast b: %v`, err)
	}
	if star != ast {
		t.Error("`*` does not set as \\ast")
	}
}

// The minus sign is drawn to match the plus: same advance, same bar on the maths
// axis. A text hyphen is neither — in the default font it measures 3.0pt against
// the plus's 7.0pt at 10pt — so `a-b` came out 1.5pt narrower than `a+b` where
// the reference sets the two to the very same width (21.71pt each, Latin Modern
// at 10pt, measured with pdftotext -bbox).
func TestMinusMatchesPlus(t *testing.T) {
	r := newRenderer(t)
	_, minus, err := r.RenderSVGMetrics(`a-b`, 10)
	if err != nil {
		t.Fatalf("a-b: %v", err)
	}
	_, plus, err := r.RenderSVGMetrics(`a+b`, 10)
	if err != nil {
		t.Fatalf("a+b: %v", err)
	}
	if minus.Width != plus.Width {
		t.Errorf("a-b is %.3f wide, a+b %.3f — the minus sign matches the plus", minus.Width, plus.Width)
	}
}

// Spacing follows from the class, so the class is measured through the space it
// buys: a braced atom is Ord, so `a{x}b` carries none, and the difference against
// `axb` is exactly what the class asks for (tex.web:15062 — Ord-Rel is thick,
// Bin either side medium, Ord-Ord nothing).
func TestClassSpacingAgainstABracedAtom(t *testing.T) {
	r := newRenderer(t)
	const px = 10
	mu := float64(px) / 18
	cases := []struct {
		tex, braced string
		want        float64 // total space the two junctions must add
		why         string
	}{
		{`a/b`, `a{/}b`, 0, "`/` is Ord (fontmath.ltx:171): a/b sets as one word"},
		{`a:b`, `a{:}b`, 2 * 3 * mu, "`:` is Rel (fontmath.ltx:155): a thick space each side"},
		{`a+b`, `a{+}b`, 2 * 2 * mu, "`+` is Bin (fontmath.ltx:151): a medium space each side"},
		{`a-b`, `a{-}b`, 2 * 2 * mu, "`-` is Bin (fontmath.ltx:153): a medium space each side"},
		{`n!`, `n{!}`, 0, "`!` is Close (fontmath.ltx:149): nothing after an Ord"},
		{`n?`, `n{?}`, 0, "`?` is Close (fontmath.ltx:169): nothing after an Ord"},
	}
	for _, c := range cases {
		_, spaced, err := r.RenderSVGMetrics(c.tex, px)
		if err != nil {
			t.Fatalf("%s: %v", c.tex, err)
		}
		_, tight, err := r.RenderSVGMetrics(c.braced, px)
		if err != nil {
			t.Fatalf("%s: %v", c.braced, err)
		}
		if got := spaced.Width - tight.Width; got < c.want-0.001 || got > c.want+0.001 {
			t.Errorf("%s adds %.3fpt of space over %s, want %.3f — %s",
				c.tex, got, c.braced, c.want, c.why)
		}
	}
}

// `:=` sets tight: Rel-Rel is 0 in the spacing table, so making `:` a relation
// costs nothing between the two characters while adding the thick space before.
func TestColonEqualsSetsTight(t *testing.T) {
	r := newRenderer(t)
	const px = 10
	_, coloneq, err := r.RenderSVGMetrics(`a:=b`, px)
	if err != nil {
		t.Fatalf("a:=b: %v", err)
	}
	_, eq, err := r.RenderSVGMetrics(`a=b`, px)
	if err != nil {
		t.Fatalf("a=b: %v", err)
	}
	_, colon, err := r.RenderSVGMetrics(`{:}`, px)
	if err != nil {
		t.Fatalf("{:}: %v", err)
	}
	if got := coloneq.Width - eq.Width - colon.Width; got < -0.001 || got > 0.001 {
		t.Errorf("a:=b is %.3fpt wider than a=b plus the colon; Rel-Rel is 0", got)
	}
}
