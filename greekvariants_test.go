// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// Two Greek pairs were the wrong way up, and nothing exercised them.
//
// \epsilon is the LUNATE epsilon and \varepsilon the rounded one; \phi is the
// STRAIGHT phi and \varphi the curly open one. LaTeX says so by the cmmi slot it
// points each name at:
//
//	\DeclareMathSymbol{\epsilon}   {\mathord}{letters}{"0F}   fontmath.ltx:179
//	\DeclareMathSymbol{\phi}       {\mathord}{letters}{"1E}   fontmath.ltx:194
//	\DeclareMathSymbol{\varepsilon}{\mathord}{letters}{"22}   fontmath.ltx:198
//	\DeclareMathSymbol{\varphi}    {\mathord}{letters}{"27}   fontmath.ltx:203
//
// and unicode-math-table.tex names the characters outright:
//
//	U+03F5  \epsilon     greek lunate varepsilon symbol
//	U+03B5  \varepsilon  rounded small varepsilon, greek
//	U+03D5  \phi         /straightphi - small phi, greek
//	U+03C6  \varphi      curly or open small phi, greek
//
// Rendered side by side with tectonic on `$\epsilon \varepsilon \phi \varphi$`,
// the reference set ϵ ε ϕ φ and this package set ε ϵ φ ϕ — each pair exactly
// exchanged. 84 of the 154 papers of the go-tex fidelity corpus use at least one
// of the four, 2564 times in all, so every one of them carried the wrong glyph.
//
// The bug survived because no test named a codepoint. This one does.
func TestGreekVariantsMatchTheirCodepoints(t *testing.T) {
	for name, want := range map[string]rune{
		"epsilon": 0x03F5, "varepsilon": 0x03B5,
		"phi": 0x03D5, "varphi": 0x03C6,
		// The pairs that were already right, so a future swap cannot pass by
		// exchanging two others.
		"theta": 0x03B8, "vartheta": 0x03D1,
		"rho": 0x03C1, "varrho": 0x03F1,
		"pi": 0x03C0, "varpi": 0x03D6,
		"kappa": 0x03BA, "varkappa": 0x03F0,
		"sigma": 0x03C3, "varsigma": 0x03C2,
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
}
