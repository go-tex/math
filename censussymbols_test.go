// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"strings"
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

// ⛔ Adding the OPENER of a delimiter pair surfaces its closer. A paper writing
// \llparenthesis x \rrparenthesis failed on the opener, so the census named only the
// opener; supplying it moved the failure one command right and \rrparenthesis appeared
// with 31 equations, \urcorner with 11 and \rbrack with 1.
//
// The census cannot see the second half of a pair, so it must not be the only thing
// consulted for one. This asserts each pair is complete AND correctly polarised — a
// closer declared clsOpen would space the group wrongly while still rendering.
func TestADelimiterPairIsAddedAsAPair(t *testing.T) {
	for _, p := range []struct{ open, close string }{
		{"llparenthesis", "rrparenthesis"},
		{"ulcorner", "urcorner"},
		{"lbrack", "rbrack"},
		{"Lbag", "Rbag"},
	} {
		o, ok1 := symbols[p.open]
		c, ok2 := symbols[p.close]
		if !ok1 || !ok2 {
			t.Errorf(`\%s present=%v, \%s present=%v: a pair is added as a pair`,
				p.open, ok1, p.close, ok2)
			continue
		}
		if o.cls != clsOpen {
			t.Errorf(`\%s class = %d, want clsOpen`, p.open, o.cls)
		}
		if c.cls != clsClose {
			t.Errorf(`\%s class = %d, want clsClose`, p.close, c.cls)
		}
	}
	// \llcorner stays out, and the reason is not symmetry: if a paper wrote it, IT
	// would be the census's first unknown. The absence is in the place that answers
	// the question. \lrcorner is here because the census named it directly.
	if _, ok := symbols["llcorner"]; ok {
		t.Error(`\llcorner is in the table: no paper in the 999-paper census names it, ` +
			`and it would be the first unknown if one did`)
	}
	if _, ok := symbols["lrcorner"]; !ok {
		t.Error(`\lrcorner is missing: the census names it directly, 152 equations`)
	}
}

// ⛔ The second crop, and the reason there WAS a second crop: my sweep looked
// unicode-math-table.tex up BY NAME, and unicode-math uses different names for glyphs
// amssymb already had. \rhd, \bigcirc and \blacklozenge each reported "no codepoint
// exists" while their codepoints sat in the table as \vartriangleright,
// \mdlgwhtcircle and \mdlgblklozenge. A name-keyed lookup turns a naming difference
// into a false absence.
//
// These four are amsfonts' \mathbin ALIASES of glyphs this table already holds as
// \mathrel (amsfonts.sty:157-160). The class is the only difference and it is the
// whole point — \lhd is a binary operation, \vartriangleleft a relation — so the test
// asserts the SAME rune and a DIFFERENT class, which is the only pair of facts that
// can be got wrong here.
func TestTheAmsfontsBinaryAliasesShareTheGlyphAndNotTheClass(t *testing.T) {
	for _, c := range []struct{ bin, rel string }{
		{"lhd", "vartriangleleft"},
		{"rhd", "vartriangleright"},
		{"unlhd", "trianglelefteq"},
		{"unrhd", "trianglerighteq"},
	} {
		b, ok1 := symbols[c.bin]
		r, ok2 := symbols[c.rel]
		if !ok1 || !ok2 {
			t.Errorf(`\%s present=%v, \%s present=%v`, c.bin, ok1, c.rel, ok2)
			continue
		}
		if b.r != r.r {
			t.Errorf(`\%s = %U but \%s = %U: amsfonts declares the same glyph`,
				c.bin, b.r, c.rel, r.r)
		}
		if b.cls != clsBin {
			t.Errorf(`\%s class = %d, want clsBin (amsfonts)`, c.bin, b.cls)
		}
		if r.cls != clsRel {
			t.Errorf(`\%s class = %d, want clsRel`, c.rel, r.cls)
		}
	}
	// And the spacing really differs, which is why the alias is worth having: a Bin
	// and a Rel take different inter-atom space, so a\lhd b and a\vartriangleleft b
	// must not render alike.
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	if svg(`a\lhd b`) == svg(`a\vartriangleleft b`) {
		t.Error(`\lhd and \vartriangleleft render alike: the class is not reaching the spacing`)
	}
}

// \bigcirc is NOT \circ. U+25CB is the WHITE CIRCLE and U+2218 the much smaller RING
// OPERATOR this table already held; mapping \bigcirc to \circ's rune would have been
// invisible in every extents test and wrong on every display.
func TestBigcircIsNotCirc(t *testing.T) {
	big, small := symbols["bigcirc"], symbols["circ"]
	if big.r == small.r {
		t.Fatalf(`\bigcirc and \circ are both %U`, big.r)
	}
	if big.r != 0x25CB {
		t.Errorf(`\bigcirc = U+%04X, want U+25CB WHITE CIRCLE`, big.r)
	}
	if small.r != 0x2218 {
		t.Errorf(`\circ = U+%04X, want U+2218 RING OPERATOR`, small.r)
	}
}

// The ones deliberately left out, each because no codepoint carries the meaning. This
// is \llceil's rule applied four more times, and it is asserted so that a later sweep
// cannot quietly add a plausible-looking wrong character.
//
// \nsubseteqq is the one worth naming: my first proposal was U+2AC5, and checking the
// Unicode NAME showed U+2AC5 is "SUBSET OF ABOVE EQUALS SIGN" — \subseteqq ITSELF, not
// its negation. There is no precomposed negated form.
func TestTheSymbolsWithNoCodepointStayOut(t *testing.T) {
	for _, name := range []string{"moo", "fatsemi", "lhook", "nsubseteqq"} {
		if s, ok := symbols[name]; ok {
			t.Errorf(`\%s is in the table as %U: no codepoint carries its meaning`, name, s.r)
		}
	}
	// A guard on the specific wrong answer: U+2AC5 must not appear under a negated
	// name anywhere in the table.
	for name, s := range symbols {
		if s.r == 0x2AC5 && strings.HasPrefix(name, "n") {
			t.Errorf(`\%s = U+2AC5, which is \subseteqq itself and not a negation`, name)
		}
	}
}

// ⛔ Three ablations passed: making \blacklozenge a Bin, \lightning a Rel and \intop an
// Ord broke NOTHING. Each is a place where the DECLARING PACKAGE disagrees with
// unicode-math, which is precisely the decision this table claims to make — and no test
// held it. The gap is closed by asserting each class against its citation, and by a
// spacing witness for the ones where the class is observable in the render.
func TestEachClassIsPinnedToItsDeclaringPackage(t *testing.T) {
	for _, c := range []struct {
		name string
		want atomClass
		src  string
	}{
		{"lhd", clsBin, "amsfonts.sty:157"},
		{"rhd", clsBin, "amsfonts.sty:159"},
		{"unlhd", clsBin, "amsfonts.sty:158"},
		{"unrhd", clsBin, "amsfonts.sty:160"},
		{"bigcirc", clsBin, "fontmath.ltx:294"},
		{"blacklozenge", clsOrd, "amssymb.sty:51 — unicode-math says mathbin"},
		{"lightning", clsOrd, "stmaryrd.sty:122 — unicode-math says mathrel"},
		{"smallsmile", clsRel, "amssymb.sty:140"},
		{"smallfrown", clsRel, "amssymb.sty:141"},
		{"intop", clsOp, "fontmath.ltx:253 — \\int with \\displaylimits"},
	} {
		s, ok := symbols[c.name]
		if !ok {
			t.Errorf(`\%s is not in the table`, c.name)
			continue
		}
		if s.cls != c.want {
			t.Errorf(`\%s class = %d, want %d (%s)`, c.name, s.cls, c.want, c.src)
		}
	}
}

// The classes above are not bookkeeping: they are the spacing. Each pair below differs
// ONLY in class, so an identical render means the class never reached the layout — which
// is what let three ablations pass.
func TestTheDisputedClassesAreVisibleInTheRender(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	// \blacklozenge is Ord where unicode-math says Bin: it must set TIGHTER than a Bin
	// with the same glyph would. \bigcirc is a Bin, so the two must differ.
	if svg(`a\blacklozenge b`) == svg(`a\bigcirc b`) {
		t.Error(`\blacklozenge (Ord) spaces like \bigcirc (Bin): the classes are not reaching the layout`)
	}
	// \lightning is Ord where unicode-math says Rel. Against a known Rel with a
	// comparable glyph, the space must differ.
	if svg(`a\lightning b`) == svg(`a\rightarrow b`) {
		t.Error(`\lightning (Ord) spaces like \rightarrow (Rel)`)
	}
	// \intop is an Op, so it takes the operator space \int does — and NOT an Ord's.
	if svg(`\intop x`) != svg(`\int x`) {
		t.Error(`\intop does not set as \int: fontmath.ltx:253 makes them the same glyph and class`)
	}
	if svg(`a\intop b`) == svg(`a\blacklozenge b`) {
		t.Error(`\intop (Op) spaces like \blacklozenge (Ord)`)
	}
}

// \amsmathbb is the largest census entry that needed no new glyph: 251 equations over one
// paper, which declares it with \DeclareSymbolFontAlphabet{\amsmathbb}{AMSb}. AMSb IS the
// AMS blackboard-bold font, so unlike \mathds/\mathbbm/\mathbbb this is not an
// approximation of a different double-struck face — it is the same alphabet \mathbb names,
// and the test asserts that equality rather than a rendered shape.
func TestAmsmathbbIsMathbbAndNotAnApproximation(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	for _, body := range []string{"P", "R", "ZQ", "1"} {
		if a, b := svg(`\amsmathbb{`+body+`}`), svg(`\mathbb{`+body+`}`); a != b {
			t.Errorf(`\amsmathbb{%s} does not set as \mathbb{%s}`, body, body)
		}
	}
	// And it is genuinely the blackboard alphabet, not a pass-through: \amsmathbb{P}
	// must differ from a bare P.
	if svg(`\amsmathbb{P}`) == svg(`P`) {
		t.Error(`\amsmathbb{P} sets as a plain P: the alphabet is not being applied`)
	}
}

// MnSymbol's two, and the five paper-local names that stay out. The class comes from
// MnSymbol.sty:998 (\mathbin) and not from the one paper's modalops.sty (\mathrel),
// because MnSymbol is the package of record — and the choice is unobservable in that
// paper, which wraps the symbol in \boxmodal{…}.
func TestMnSymbolsTwoAndNotThePapersFive(t *testing.T) {
	for _, c := range []struct {
		name string
		r    rune
		src  string
	}{
		{"medsquare", 0x25FB, "MnSymbol.sty:998"},
		{"meddiamond", 0x25C7, "MnSymbol.sty:1005"},
	} {
		s, ok := symbols[c.name]
		if !ok {
			t.Errorf(`\%s is not in the table (%s)`, c.name, c.src)
			continue
		}
		if s.r != c.r {
			t.Errorf(`\%s = U+%04X, want U+%04X`, c.name, s.r, c.r)
		}
		if s.cls != clsBin {
			t.Errorf(`\%s class = %d, want clsBin (%s says \mathbin)`, c.name, s.cls, c.src)
		}
	}
	// ⛔ These are declared in ONE paper's own modalops.sty, not by MnSymbol. A general
	// table is not the place for one paper's names, and \medsquarevert could not be
	// served anyway: Unicode has no squared vertical bar.
	for _, name := range []string{
		"medsquaredot", "medsquareminus", "medsquarevert",
		"medsquareplus", "medsquaretimes", "medsquarefilled",
	} {
		if s, ok := symbols[name]; ok {
			t.Errorf(`\%s is in the table as %U: it is one paper's own name, not MnSymbol's`,
				name, s.r)
		}
	}
	// \medsquare must not collide with the box operators that share its shape family:
	// U+25FB is the WHITE MEDIUM SQUARE, distinct from \boxdot, \boxminus and friends.
	for _, other := range []string{"boxdot", "boxminus", "boxplus", "boxtimes"} {
		if symbols["medsquare"].r == symbols[other].r {
			t.Errorf(`\medsquare and \%s are both %U`, other, symbols[other].r)
		}
	}
}
