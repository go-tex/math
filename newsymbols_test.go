// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// One unknown command drops the WHOLE formula, so a missing symbol costs far more
// than the symbol. Sweeping the 154-paper go-tex fidelity corpus for equations the
// layer refused, these names sat at the head of the list, each one an ordinary
// character of a published package:
//
//	\llceil 26  \checkmark 17  \upmu 14  \dashrightarrow 13  \mathsection 11
//	\hexagon 5  \nsucceq 4  \varTheta 4
//
// Every codepoint here was read from unicode-math-table.tex and confirmed against
// UnicodeData.txt — never from the AMS font slot, which names a position in an
// encoding and not a character. Two of them are traps:
//
//   - \varTheta is amsmath's (amsmath.sty:388, cmmi slot "02), the ITALIC capital
//     theta. U+03F4 "greek capital theta symbol" is the upright one and would be a
//     different letter.
//   - \mathellipsis is not a glyph: fontmath.ltx:512 makes it
//     \mathinner{\ldotp\ldotp\ldotp}, so it must set exactly as \ldots.
//
// \llceil, the largest entry, is deliberately absent: it is stmaryrd's own glyph
// and Unicode has no codepoint for it. Guessing one would put a wrong character on
// 26 displays, which is worse than leaving them to be reported.
func TestSymbolsAddedForTheCorpus(t *testing.T) {
	for name, want := range map[string]rune{
		"checkmark": 0x2713, "mathsection": 0x00A7,
		"dashrightarrow": 0x21E2, "nsucceq": 0x22E1,
		"hexagon": 0x2394, "varTheta": 0x0398,
	} {
		s, ok := symbols[name]
		if !ok {
			t.Errorf(`\%s is not in the symbol table`, name)
			continue
		}
		if s.r != want {
			t.Errorf(`\%s = U+%04X, want U+%04X`, name, s.r, want)
		}
	}
	if symbols["mathellipsis"].r != symbols["ldots"].r {
		t.Errorf(`\mathellipsis = %U, want the same as \ldots (%U)`,
			symbols["mathellipsis"].r, symbols["ldots"].r)
	}
	if _, ok := symbols["llceil"]; ok {
		t.Error(`\llceil has a codepoint here; Unicode has none for it (see the comment)`)
	}
}

// upgreek declares the upright Greek alphabet one \DeclareMathSymbol at a time, and
// a document that writes \upmu writes \upsigma on the next line. Each name takes
// the same character as its italic counterpart, because this table already keys the
// italic names on the plain Greek block (\mu is U+03BC) and the face decides.
func TestUprightGreekMatchesItsItalicCounterpart(t *testing.T) {
	lower := []string{"alpha", "beta", "chi", "delta", "epsilon", "eta", "gamma",
		"iota", "kappa", "lambda", "mu", "nu", "omega", "phi", "pi", "psi", "rho",
		"sigma", "tau", "theta", "upsilon", "varepsilon", "varphi", "varpi",
		"vartheta", "xi", "zeta"}
	for _, n := range lower {
		up, ok := symbols["up"+n]
		if !ok {
			t.Errorf(`\up%s is missing`, n)
			continue
		}
		if up.r != symbols[n].r {
			t.Errorf(`\up%s = %U, want %U (same as \%s)`, n, up.r, symbols[n].r, n)
		}
	}
	// upgreek's capitals, spelled as the package spells them: \Updelta, not \UpDelta.
	for up, it := range map[string]string{
		"Updelta": "Delta", "Upgamma": "Gamma", "Uplambda": "Lambda",
		"Upomega": "Omega", "Upphi": "Phi", "Uppi": "Pi", "Uppsi": "Psi",
		"Upsigma": "Sigma", "Uptheta": "Theta", "Upupsilon": "Upsilon", "Upxi": "Xi",
	} {
		s, ok := symbols[up]
		if !ok {
			t.Errorf(`\%s is missing`, up)
			continue
		}
		if s.r != symbols[it].r {
			t.Errorf(`\%s = %U, want %U (same as \%s)`, up, s.r, symbols[it].r, it)
		}
	}
}
