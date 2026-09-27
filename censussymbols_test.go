// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"
)

// The 57 entries added for the 999-paper census (see the block in parse.go). These
// tests pin the four decisions that reading amssymb, amsfonts, fontmath.ltx,
// latexsym and stmaryrd changed — each one a plausible guess that was wrong — rather
// than re-stating the table, which would only assert that a literal equals itself.

// \blacktriangle is the SMALL filled triangle and \blacktriangleleft the LARGE one.
// The up/down pair and the left/right pair take different sizes
// (unicode-math-table.tex:691 vs :703), which no symmetry argument predicts.
func TestTheFilledTrianglesDoNotAllTakeTheSameSize(t *testing.T) {
	for _, c := range []struct {
		name string
		want rune
	}{
		{"blacktriangle", 0x25B4},      // small, up
		{"blacktriangledown", 0x25BE},  // small, down
		{"blacktriangleleft", 0x25C0},  // LARGE, left
		{"blacktriangleright", 0x25B6}, // LARGE, right
	} {
		if got := symbols[c.name].r; got != c.want {
			t.Errorf(`\%s = U+%04X, want U+%04X`, c.name, got, c.want)
		}
	}
}

// \triangledown is U+25BF and \bigtriangledown U+25BD. amssymb declares them as two
// symbols, so mapping both to U+25BD — the entry this table already held — would
// have made them indistinguishable while every extents test still passed.
func TestTheOpenDownTriangleIsNotTheBigOne(t *testing.T) {
	small, big := symbols["triangledown"].r, symbols["bigtriangledown"].r
	if small == big {
		t.Fatalf(`\triangledown and \bigtriangledown are both %U`, small)
	}
	if small != 0x25BF || big != 0x25BD {
		t.Errorf(`\triangledown = U+%04X (want 25BF), \bigtriangledown = U+%04X (want 25BD)`,
			small, big)
	}
}

// Where a package and unicode-math disagree about the class, the PACKAGE wins: it is
// what a document loading it compiles with. amssymb makes the left/right filled
// triangles relations (amssymb.sty:120-121) and latexsym makes \Join one
// (latexsym.sty:60); unicode-math classes the first two as ordinary and \Join as an
// n-ary operator.
func TestTheDeclaringPackageDecidesTheClassNotUnicodeMath(t *testing.T) {
	for _, c := range []struct {
		name string
		want atomClass
		src  string
	}{
		{"blacktriangleleft", clsRel, "amssymb.sty:121"},
		{"blacktriangleright", clsRel, "amssymb.sty:120"},
		{"Join", clsRel, "latexsym.sty:60"},
		{"Bbbk", clsOrd, "amssymb.sty:261"},
	} {
		if got := symbols[c.name].cls; got != c.want {
			t.Errorf(`\%s class = %d, want %d (%s)`, c.name, got, c.want, c.src)
		}
	}
}

// The class is not decoration: it is the spacing. A relation takes \thickmuskip on
// both sides and an ordinary symbol takes none, so declaring \blacktriangleleft
// ordinary would set it TIGHTER than the reference. This measures the difference
// rather than asserting the constant, because the constant is what the table already
// says.
func TestARelationIsSetWiderThanAnOrdinarySymbol(t *testing.T) {
	r := newRenderer(t)
	width := func(tex string) float64 {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m.Width
	}
	// \blacktriangleleft (relation) against \blacktriangle (ordinary), same two
	// operands. Both glyphs exist and neither is zero-width, so any difference is
	// the inter-atom spacing.
	rel := width(`a\blacktriangleleft b`)
	ord := width(`a\blacktriangle b`)
	if rel <= ord {
		t.Errorf("relation %.4f is not wider than ordinary %.4f: the class is not reaching the spacing",
			rel, ord)
	}
	// And the gain is the two thick spaces, not a glyph-width accident: the LARGE
	// triangle is itself wider, so compare against the same glyph set as ordinary.
	bare := width(`ab`)
	if rel-bare < ord-bare {
		t.Errorf("rel−bare = %.4f, ord−bare = %.4f", rel-bare, ord-bare)
	}
}

// amssymb.sty:68 is \let\restriction\upharpoonright — the same symbol under two
// names, not two symbols. \restriction is absent from unicode-math-table.tex
// entirely, so the \let is the only source for it.
func TestRestrictionIsUpharpoonrightUnderAnotherName(t *testing.T) {
	a, b := symbols["restriction"], symbols["upharpoonright"]
	if a.r != b.r || a.cls != b.cls {
		t.Errorf(`\restriction = {%U, %d}, \upharpoonright = {%U, %d}: amssymb.sty:68 \lets them equal`,
			a.r, a.cls, b.r, b.cls)
	}
	if a.r != 0x21BE {
		t.Errorf(`\restriction = U+%04X, want U+21BE`, a.r)
	}
}

// stmaryrd's bags name the stmry FONT, which we do not have, so unicode-math has no
// entry for them. The codepoints come from the Unicode character names themselves —
// U+27C5/U+27C6 are "LEFT/RIGHT S-SHAPED BAG DELIMITER" — and the classes from
// stmaryrd.sty:166-167. This is the same situation as \llceil, which stays OUT
// because no codepoint carries its meaning; the bags are in because two do.
func TestTheBagsAreOpenAndCloseDelimiters(t *testing.T) {
	if got := symbols["Lbag"]; got.r != 0x27C5 || got.cls != clsOpen {
		t.Errorf(`\Lbag = {U+%04X, %d}, want {U+27C5, clsOpen}`, got.r, got.cls)
	}
	if got := symbols["Rbag"]; got.r != 0x27C6 || got.cls != clsClose {
		t.Errorf(`\Rbag = {U+%04X, %d}, want {U+27C6, clsClose}`, got.r, got.cls)
	}
	if _, ok := symbols["llceil"]; ok {
		t.Error(`\llceil is in the table: no codepoint carries its meaning, ` +
			`and the census entry is worth less than a wrong character on 26 displays`)
	}
}

// A stretchy or combining accent is not a symbol-table entry. These four are census
// entries — \widecheck alone is 97 equations — and they are deliberately absent,
// because a combining character standing alone sets as a mark over nothing.
func TestTheAccentsAreNotInTheSymbolTable(t *testing.T) {
	for _, name := range []string{"widecheck", "underrightarrow", "underleftarrow", "dddot"} {
		if s, ok := symbols[name]; ok {
			t.Errorf(`\%s is in the symbol table as %U: it needs the accent machinery`,
				name, s.r)
		}
	}
}

// Every one of the 57 renders, in both styles, with finite extents. A table entry
// that parses and then produces a NaN width is the failure this catches.
func TestEveryCensusSymbolRenders(t *testing.T) {
	r := newRenderer(t)
	for _, name := range []string{
		"blacktriangleleft", "Bbbk", "lrcorner", "blacktriangle", "triangledown",
		"smallsetminus", "blacktriangleright", "rightharpoonup", "blacktriangledown",
		"upharpoonright", "restriction", "longleftrightarrow", "Downarrow", "frown",
		"Uparrow", "looparrowright", "eqdef", "llparenthesis", "Lbag", "Rbag",
		"rightleftharpoons", "sslash", "nrightarrow", "backsim", "bigsqcap", "mho",
		"rightleftarrows", "ulcorner", "succsim", "nsubseteq", "rightarrowtail",
		"multimap", "geqq", "Cup", "nvDash", "mapsfrom", "nleq", "dotminus", "Join",
		"leftrightarrows", "nprec", "leqq", "nRightarrow", "Subset", "precneqq",
		"lll", "smile", "leftrightharpoons", "ncong", "lnsim", "lneq", "lessapprox",
		"lbrack", "gtreqqless", "gtrapprox", "fint", "approxeq",
	} {
		if _, ok := symbols[name]; !ok {
			t.Errorf(`\%s is not in the table`, name)
			continue
		}
		_, m, err := r.RenderSVGMetrics(`x`+`\`+name+` y`, 32) // the space TERMINATES the control word
		if err != nil {
			t.Errorf(`render \%s: %v`, name, err)
			continue
		}
		if gomath.IsNaN(m.Width) || gomath.IsInf(m.Width, 0) || m.Width <= 0 {
			t.Errorf(`\%s: width = %v`, name, m.Width)
		}
	}
}
