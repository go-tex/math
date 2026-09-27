// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"fmt"
	"math"
	"testing"

	"github.com/go-opentype/opentype"
)

func envTotal(t *testing.T, r *Renderer, tex string) float64 {
	t.Helper()
	_, m, err := r.RenderSVGMetrics(tex, 10)
	if err != nil {
		t.Fatalf("%s: %v", tex, err)
	}
	return m.Height + m.Depth
}

// gridLayout separated rows by a gap proportional to the point size (rowGap:
// p*0.4 and friends). TeX's model for anything built on \array — every matrix,
// and cases — is different in KIND: \@arstrut puts a rule of
// \arraystretch(.7/.3 of \baselineskip) in every row (latex.ltx:12103) and the
// rows BUTT, the struts alone providing the pitch. \jot, where an environment has
// one, is the only thing added between them.
//
// \ht+\dp of \hbox{$\begin{env}…\end{env}$} at 10pt, one row and three, against
// tectonic:
//
//	              one row: ref / before / after      three rows: ref / before / after
//	array            12.00    5.45    12.00                36.00   22.67   36.00
//	matrix           12.00    9.32    12.00                36.00   23.84   36.00
//	pmatrix          12.00    9.32    12.00                36.00   23.84   36.00
//	bmatrix          12.00    9.32    12.00                36.00   23.85   36.00
//	vmatrix          12.00       —    12.00                36.00       —    36.00
//	cases            18.00    9.33    16.67                43.20   31.01   45.00
//	aligned          12.00    7.02    12.00                42.00   31.06   42.00
//	gathered         12.00    5.45    12.00                42.00   24.67   42.00
//
// ⚠ cases at three rows read 43.20 — exactly the reference — while the extensible
// brace was CAPPED at the largest size variant. The brace was too short, the rows'
// own 43.20 decided the total, and the number matched for a reason that had nothing
// to do with the brace. Removing the cap (#36) lets the assembled brace reach 45.00,
// which exceeds the rows, so the total is the brace's. The 1.80 is the face's
// assembly granularity: the recipe advances 65.00 over four joints that compress by
// at most 5/5/5/6, so 45.00 is the smallest it can be built at, and the next count
// down cannot reach 43.20 at all.
//
// The cap it replaced was 21x worse where it mattered. Against tectonic over content
// heights 2 to 200pt, sum of |error| on the delimiter total: 340.33 capped, 16.06
// assembled. At 200pt of content the capped brace was 176.40 too short.
//
// TOTALS are asserted, not a derived pitch. For cases the derived pitch is
// meaningless: its one-row total is floored by the brace (see
// TestCasesOneRowIsFlooredByItsBrace), so subtracting it from the three-row total
// gives an increment that belongs to neither the rows nor the delimiter — I read
// 12.60 that way and carried it through a whole PR as though it were the pitch.
func TestRowsStackOnStrutsWithNoInterlineGlue(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct {
		name       string
		one, three string
		wantOne    float64
		wantThree  float64
	}{
		{"array", `\begin{array}{l}x\end{array}`, `\begin{array}{l}x\\x\\x\end{array}`, 12, 36},
		{"matrix", `\begin{matrix}x\end{matrix}`, `\begin{matrix}x\\x\\x\end{matrix}`, 12, 36},
		{"pmatrix", `\begin{pmatrix}x\end{pmatrix}`, `\begin{pmatrix}x\\x\\x\end{pmatrix}`, 12, 36},
		{"bmatrix", `\begin{bmatrix}x\end{bmatrix}`, `\begin{bmatrix}x\\x\\x\end{bmatrix}`, 12, 36},
		{"vmatrix", `\begin{vmatrix}x\end{vmatrix}`, `\begin{vmatrix}x\\x\\x\end{vmatrix}`, 12, 36},
		// cases carries \arraystretch 1.2, so 3 x 14.40 of rows — but BOTH totals here
		// are the brace's, not the rows': 16.67 at one row and 45.00 at three. See the
		// note above for why the three-row figure moved away from the reference.
		{"cases", `\begin{cases}x & y\end{cases}`,
			`\begin{cases}x & y\\x & y\\x & y\end{cases}`, 16.67, 45.00},
		// \jot adds 3pt at 10pt BETWEEN rows: 12 + 2 x 15.
		{"aligned", `\begin{aligned}x &= y\end{aligned}`,
			`\begin{aligned}x &= y\\x &= y\\x &= y\end{aligned}`, 12, 42},
		{"gathered", `\begin{gathered}x\end{gathered}`,
			`\begin{gathered}x\\x\\x\end{gathered}`, 12, 42},
	} {
		t.Run(c.name, func(t *testing.T) {
			if v := envTotal(t, r, c.one); math.Abs(v-c.wantOne) > 0.05 {
				t.Errorf("one row = %.3f, want %.2f", v, c.wantOne)
			}
			if v := envTotal(t, r, c.three); math.Abs(v-c.wantThree) > 0.05 {
				t.Errorf("three rows = %.3f, want %.2f", v, c.wantThree)
			}
		})
	}
}

// ⭐ The case whose ABSENCE let the wrong model look exact, exercised through the
// one environment that carries a stretch: cases is \arraystretch 1.2
// (amsmath.sty:1106), so its rows are 14.40pt and three of them are 43.20pt — the
// pitch tracks the stretch LINEARLY, because the rows butt and the strut is the
// pitch.
//
// The first version of this fix computed the pitch as tex.web §679 does for a
// vertical list — d = \baselineskip - prevDepth - height, floored at \lineskip —
// which gives the SAME answer whenever the strut exactly fills the leading, i.e. at
// \arraystretch 1. Every test held it at 1, so the two models were
// indistinguishable and the wrong one measured exact on seven environments. At 1.2
// it gives 15.40 per row instead of 14.40, which is what this asserts.
//
// The lesson generalises past this file: a test set that holds a parameter at the
// value where two models agree cannot tell them apart.
//
// ⚠ NOT asserted here, because it is not implemented: a \def\arraystretch in the
// SOURCE is not read — the stretch comes from envTable and only cases has one.
// tectonic gives \def\arraystretch{1.2}\begin{array}{@{}l@{}}x…: 14.40 for one row
// and 43.20 for three, and 18.00/54.00 at 1.5; we give 12.00/36.00 at every value.
// That is a separate feature from this model and wants its own change.
func TestTheStretchedEnvironmentTracksItsStrutLinearly(t *testing.T) {
	r := newRenderer(t)
	// ⛔ This assertion USED to run through \begin{cases}, whose three rows came to
	// 43.20 while the extensible brace was capped and too short to decide the total.
	// Uncapping it (#36) lets the brace reach 45.00 — which is 0.20 from the 45.20
	// that the WRONG model produces. The delimiter had swallowed the very distinction
	// this test exists to make, and a test that cannot separate the two models is the
	// defect this whole file was written against.
	//
	// So the model is asserted where no delimiter can reach it: on the leading itself.
	// A strutted row is \arraystretch x (.7 + .3) x \baselineskip and the rows butt,
	// so three of them are exactly three struts — linear in the stretch, which is the
	// property. §679 against a 12pt leading would give 15.40 per row at stretch 1.2.
	for _, c := range []struct {
		stretch, perRow float64
	}{{1.0, 12.00}, {1.2, 14.40}, {1.5, 18.00}} {
		h, d := strutLeading{baselineskip: bl(10), stretch: c.stretch}.strut()
		if got := h + d; math.Abs(got-c.perRow) > 0.005 {
			t.Errorf("strut at \\arraystretch %g = %.3f, tectonic gives %.2f per row",
				c.stretch, got, c.perRow)
		}
		// The rows butt: no gap, so n rows are n struts and nothing else.
		if gap := (strutLeading{baselineskip: bl(10), stretch: c.stretch}).gap(d, h); gap != 0 {
			t.Errorf("strutted rows must butt, gap = %.3f at stretch %g", gap, c.stretch)
		}
	}
	// And end to end, with the brace's contribution stated rather than mixed in: the
	// rows are 43.20 and the assembled brace is 45.00, so the total is the brace's.
	three := envTotal(t, r, `\begin{cases}x & y\\x & y\\x & y\end{cases}`)
	if math.Abs(three-45.00) > 0.05 {
		t.Errorf("cases three rows = %.3f, this state is 45.00 — the ROWS are 43.20 "+
			"(= 3 x 14.40, the reference) and the brace exceeds them", three)
	}
	// And the unstretched control, so this is a statement about the stretch and not
	// about matrices in general.
	if v := envTotal(t, r, `\begin{pmatrix}x\\x\\x\end{pmatrix}`); math.Abs(v-36) > 0.05 {
		t.Errorf("pmatrix three rows = %.3f, tectonic gives 36.00", v)
	}
}

// A row taller than the strut takes its own height, which is the other half of the
// model: the strut is a FLOOR, not a fixed pitch.
func TestATallRowTakesItsOwnHeight(t *testing.T) {
	r := newRenderer(t)
	base := envTotal(t, r, `\begin{pmatrix}x\\x\end{pmatrix}`)
	flat := envTotal(t, r, `\begin{pmatrix}x\\x\\x\end{pmatrix}`) - base
	tall := envTotal(t, r, `\begin{pmatrix}x\\x\\\frac{\frac{a}{b}}{c}\end{pmatrix}`) - base
	t.Logf("row added: flat %.3f, tall %.3f (tectonic: 12.00 and 13.95)", flat, tall)
	if math.Abs(flat-12) > 0.05 {
		t.Errorf("flat row added = %.3f, tectonic gives 12.00", flat)
	}
	// Bounded against the reference's own opening, 13.95-12.00, rather than a round
	// number: we open by 2.20 against its 1.95, a 0.25pt residual on the fraction's
	// height that this change does not address. 0.4pt of slack admits it and still
	// refuses the old behaviour, which opened by 16.74-9.58 = 7.16.
	const refOpening = 13.95 - 12.00
	if d := (tall - flat) - refOpening; d > 0.4 || d < -0.4 {
		t.Errorf("a tall row opens the pitch by %.3f, tectonic by %.2f (difference %+.3f)",
			tall-flat, refOpening, d)
	}
}

// cases' one-row total is the BRACE, not its rows, and the decomposition took two
// wrong attempts to get right:
//
//	array, plain                        12.00   the strut
//	array with \arraystretch{1.2}       14.40   = 1.2 x 12.00, and cases' rows
//	  plus \left\lbrace                 18.00   = 14.40 + 3.60
//	cases itself                        18.00   h=11.50 d=6.50, identical
//
// amsmath.sty:1106 is \left\lbrace \def\arraystretch{1.2} \array{@{}l@{\quad}l@{}},
// and rebuilding exactly that reproduces cases to the last digit. From three rows
// on the brace no longer floors anything and cases is exact (43.20).
//
// ⚠ Two wrong turns worth keeping: I first said the 18.00 was "NOT the brace",
// having tested \left\{ x \right. — a TINY content, where the brace is at its
// smallest and measures 10.00pt exactly as \left( x \right) does — and generalised
// from it. A \left delimiter grows with what it spans. Then I read the 18.00-to-43.20
// difference as a 12.60pt "pitch", which is neither the rows' nor the brace's.
//
// So what is left here is our delimiter ladder choosing a smaller brace than the
// reference's at a 14.40pt content: 16.67 against 18.00. Pinned to what this engine
// produces, not to the reference — the point is to notice a change.
func TestCasesOneRowIsFlooredByItsBrace(t *testing.T) {
	r := newRenderer(t)
	// cases' rows come to 14.40pt, which three of them show (43.20, asserted in
	// TestTheStretchedEnvironmentTracksItsStrutLinearly). A single row is 16.67 here
	// and 18.00 in the reference: both exceed the 14.40 of the rows, so the brace
	// does floor it in both — ours just picks a smaller step from the ladder.
	const rows = 14.40
	full := envTotal(t, r, `\begin{cases}x & y\end{cases}`)
	if full <= rows {
		t.Errorf("the brace does not raise the total: %.3f against %.2f for the rows alone",
			full, rows)
	}
	if math.Abs(full-16.67) > 0.05 {
		t.Errorf("cases one row = %.3f, this state is 16.67 (tectonic 18.00) - "+
			"if that is intended, update here AND the table above", full)
	}
}

// smallmatrix takes TeX's interline glue, not a strut. amsmath.sty:1045-1047 sets
// \baselineskip6\ex@ \lineskip1.5\ex@ \lineskiplimit\lineskip and struts nothing,
// so tex.web §679 decides each gap from the rows' own extents — a different model
// in kind from the \array family, which is why leading.go has three types.
//
// Measured off tectonic at 10pt with `measure boxhd` and `measure dimens`
// (go-tex/measure). The registers, read out of the reference's own log rather than
// inferred: \ex@ = 1.0000, \baselineskip = 6.0000, \lineskip = \lineskiplimit =
// 1.5000.
//
// The subjects are \rule boxes, whose height and depth are STATED and not the
// face's business. That matters twice over: the model is what is under test here,
// and the residual difference on glyph content is entirely the face (see
// TestSmallmatrixResidualIsTheFaceNotTheGrid).
//
//	rows of                     \ht+\dp: tectonic / here
//	\rule{1pt}{1pt}   x2                  7.0000 / 7.0000
//	\rule{1pt}{1pt}   x3                 13.0000 / 13.0000
//	\rule{1pt}{4pt}   x2                 10.0000 / 10.0000    gap 2.0 ≥ limit
//	\rule{1pt}{4.5pt} x2                 10.5000 / 10.5000    gap exactly at limit
//	\rule{1pt}{5pt}   x2                 11.5000 / 11.5000    gap 1.0 < limit → \lineskip
//	\rule{1pt}{10pt}  x2                 21.5000 / 21.5000    far past the limit
//	\rule{1pt}{10pt}  x3                 33.0000 / 33.0000
//
// The 5pt row is the one that separates the models: a grid that always advanced by
// \baselineskip would give 11.0000 there, and §679 gives 11.5000 because the gap
// falls under \lineskiplimit and is replaced by \lineskip outright — the floor is
// not a clamp to the limit but a substitution.
func TestSmallmatrixTakesInterlineGlue(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{`\begin{smallmatrix}\rule{1pt}{1pt}\\\rule{1pt}{1pt}\end{smallmatrix}`, 7.0},
		{`\begin{smallmatrix}\rule{1pt}{1pt}\\\rule{1pt}{1pt}\\\rule{1pt}{1pt}\end{smallmatrix}`, 13.0},
		{`\begin{smallmatrix}\rule{1pt}{4pt}\\\rule{1pt}{4pt}\end{smallmatrix}`, 10.0},
		{`\begin{smallmatrix}\rule{1pt}{4.5pt}\\\rule{1pt}{4.5pt}\end{smallmatrix}`, 10.5},
		{`\begin{smallmatrix}\rule{1pt}{5pt}\\\rule{1pt}{5pt}\end{smallmatrix}`, 11.5},
		{`\begin{smallmatrix}\rule{1pt}{10pt}\\\rule{1pt}{10pt}\end{smallmatrix}`, 21.5},
		{`\begin{smallmatrix}\rule{1pt}{10pt}\\\rule{1pt}{10pt}\\\rule{1pt}{10pt}\end{smallmatrix}`, 33.0},
	} {
		if v := envTotal(t, r, c.tex); math.Abs(v-c.want) > 0.01 {
			t.Errorf("%s = %.4f, tectonic gives %.4f", c.tex, v, c.want)
		}
	}
}

// TestSmallmatrixPitchIsSixExAt states the property, not one operating point: the
// pitch of rows that fit inside the leading IS \baselineskip = 6\ex@, whatever the
// rows are. A table of totals can be reproduced by a wrong model that happens to
// agree there — which is exactly what happened to the \array grid (#26, #28) — so
// the pitch is asserted on three different row heights that all take the same
// branch.
func TestSmallmatrixPitchIsSixExAt(t *testing.T) {
	r := newRenderer(t)
	const want = 6.0 // 6\ex@ at 10pt, \ex@ = 1.0000 measured
	for _, row := range []string{`\rule{1pt}{1pt}`, `\rule{1pt}{2pt}`, `\rule{1pt}{4pt}`} {
		two := envTotal(t, r, `\begin{smallmatrix}`+row+`\\`+row+`\end{smallmatrix}`)
		three := envTotal(t, r, `\begin{smallmatrix}`+row+`\\`+row+`\\`+row+`\end{smallmatrix}`)
		if p := three - two; math.Abs(p-want) > 0.01 {
			t.Errorf("rows of %s: pitch %.4f, 6\\ex@ is %.4f", row, p, want)
		}
	}
}

// TestSmallmatrixResidualIsTheFaceNotTheGrid separates what this grid fixed from
// what it did not, so neither is mistaken for the other later.
//
// On glyph content the totals still differ from the reference, and the whole
// difference is the FACE's MATH table, not the vertical model:
//
//	                      tectonic    here
//	math axis at 10pt       2.5000  3.0000   \fontdimen22 vs MATH AxisHeight
//	script size at 10pt          7       7   identical, so not the cause
//	x at script size        3.0138  3.4230   the face's x-height
//
// The axis shifts height against depth and leaves the TOTAL alone, which is why
// every \rule measurement above is exact while \begin{smallmatrix}x\end{smallmatrix}
// is not. Pinned so a change in either is noticed; neither is claimed to be right.
func TestSmallmatrixResidualIsTheFaceNotTheGrid(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	if got := e.axis(10); math.Abs(got-3.0) > 0.001 {
		t.Errorf("axis = %.4f, this state is 3.0000 (tectonic 2.5000)", got)
	}
	// The script size is the half of this that does NOT differ. Asserting it keeps
	// the attribution honest: if it ever drifts, the glyph residual below stops
	// being about the x-height alone.
	if got := e.scriptSize(10); got != 7 {
		t.Errorf("script size = %d, tectonic uses 7", got)
	}
	// h = T/2 + axe for a \vcenter, so T = 2(h - axe) recovers the row's own extent
	// from a total whose depth the leading \null clamps at zero — in the reference
	// AND here, which is why the one-row totals are compared this way and not
	// directly.
	_, m, err := r.RenderSVGMetrics(`\begin{smallmatrix}x\end{smallmatrix}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if T := 2 * (m.Height - e.axis(10)); math.Abs(T-3.4230) > 0.01 {
		t.Errorf("x at script size = %.4f, this state is 3.4230 (tectonic 3.0138)", T)
	}
}

// smallmatrix's HORIZONTAL spacing is stated in mu, and mu is 1/18 of the OUTER
// size — not of the script size the cells are set in. The template's \thickspace
// and the two \, sit outside the $\m@th\scriptstyle##$ that each cell is
// (amsmath.sty:1045-1052).
//
// Measured off tectonic at 10pt with `measure dimens` reading \wd, on
// \rule{1pt}{1pt} cells so the face is not involved. R is one such rule and [S] a
// one-cell smallmatrix:
//
//	subject       tectonic     before      after
//	[S]             4.3332     1.0000     4.3333
//	R[S]            5.3332     6.2000     5.3333   ← no space at the edge
//	[S]R            5.3332     6.2000     5.3333
//	R[S]R           6.3332    11.4000     6.3333
//	two columns     8.1102     6.2000     8.1111
//	three columns  11.8872    11.4000    11.8889
//
// Two defects of OPPOSITE sign were cancelling: the two \, were missing (−3.3333)
// and every column gap was 0.6 x the cell size instead of 5mu at the outer size
// (+1.4222 each). At three columns the net error was −0.4872, 4% of the width, and
// it changed sign at four — a width check at one column count would have called
// this nearly right. See #31.
//
// The edge rows are what fix the atom CLASS: the environment is an Ord, so no
// inter-atom space is inserted beside it, where gridLayout's clsInner would have
// charged thin space. \vcenter is an Ord for spacing purposes, and this is the
// measurement that says so rather than a reading of tex.web.
//
// Tolerance 0.002: TeX converts mu in scaled points and truncates, so 5mu comes out
// 2.7770 where 5/18 x 10 is 2.7778 — 0.0008 per mu-space, twice over at three
// columns. The bound is that arithmetic and nothing looser.
func TestSmallmatrixSpacesInMuAtTheOuterSize(t *testing.T) {
	r := newRenderer(t)
	const R = `\rule{1pt}{1pt}`
	const S = `\begin{smallmatrix}` + R + `\end{smallmatrix}`
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{S, 4.3332},
		{R + S, 5.3332},
		{S + R, 5.3332},
		{R + S + R, 6.3332},
		{`\begin{smallmatrix}` + R + `&` + R + `\end{smallmatrix}`, 8.1102},
		{`\begin{smallmatrix}` + R + `&` + R + `&` + R + `\end{smallmatrix}`, 11.8872},
	} {
		_, m, err := r.RenderSVGMetrics(c.tex, 10)
		if err != nil {
			t.Fatalf("%s: %v", c.tex, err)
		}
		if math.Abs(m.Width-c.want) > 0.002 {
			t.Errorf("%s width = %.4f, tectonic gives %.4f", c.tex, m.Width, c.want)
		}
	}
}

// The extensible delimiter used to stop growing. stretchVertical took the largest
// MATH size variant as its fallback, so a brace's depth was frozen at 16.105 for
// every content height from 24pt to 200pt: a \left\lbrace over 200pt of material was
// set with a brace about a fifth of the height it was meant to embrace (#36).
//
// An OpenType MATH font also carries MathGlyphAssembly, a recipe for building the
// glyph from top, extender and bottom parts, and that is what had no implementation.
//
// \ht+\dp of \hbox{$\left\lbrace\rule{1pt}{H}\right.$} at 10pt, measured off tectonic
// with `measure dimens` reading \ht0 and \dp0:
//
//	content   tectonic     capped   assembled
//	      2      10.000      9.330       9.330
//	     10      18.000     14.270      14.270
//	     16      30.000     26.250      26.250
//	     20      36.000     35.810      35.810
//	     24      42.500     40.105      42.000
//	     28      49.500     44.105      50.000
//	     34      61.500     50.105      62.000
//	     40      73.500     56.105      74.000
//	     60     114.501     76.105     114.000
//	    100     193.501    116.105     194.000
//	    200     392.502    216.105     394.000
//
// Sum of |error| over those thirteen heights: 340.33 capped, 16.06 assembled. Below
// 20pt of content nothing changes — the variants still cover it — and the error there
// is the face's chain being finer than Computer Modern's, which is #36 and not this.
func TestTheExtensibleDelimiterDoesNotStopGrowing(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct {
		content, want float64
	}{
		{24, 42.0}, {28, 50.0}, {34, 62.0}, {40, 74.0},
		{60, 114.0}, {100, 194.0}, {200, 394.0},
	} {
		tex := fmt.Sprintf(`\left\lbrace\rule{1pt}{%gpt}\right.`, c.content)
		_, m, err := r.RenderSVGMetrics(tex, 10)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		if got := m.Height + m.Depth; math.Abs(got-c.want) > 0.05 {
			t.Errorf("content %gpt: delimiter total %.3f, this state is %.2f", c.content, got, c.want)
		}
	}
}

// TestTheDelimiterGrowsStrictlyWithItsContent states the property the cap broke,
// rather than only the table above: past the variants' reach every increase in
// content must increase the delimiter. The capped version was CONSTANT here — the
// same 16.105 of depth at 24pt of content and at 200 — and a table of totals hid it,
// because the content's own height kept the TOTAL rising while the brace stood still.
func TestTheDelimiterGrowsStrictlyWithItsContent(t *testing.T) {
	r := newRenderer(t)
	prev := 0.0
	for _, h := range []float64{24, 40, 60, 100, 160, 200} {
		tex := fmt.Sprintf(`\left\lbrace\rule{1pt}{%gpt}\right.`, h)
		_, m, err := r.RenderSVGMetrics(tex, 10)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		// The DEPTH is the delimiter's alone: the rule has none, so nothing else can
		// contribute to it. That is what makes it the witness here and the total not.
		if m.Depth <= prev {
			t.Errorf("content %gpt: delimiter depth %.4f, no greater than %.4f at the "+
				"previous height — the delimiter has stopped growing", h, m.Depth, prev)
		}
		prev = m.Depth
	}
}

// TestTheAssemblyIsStackedByItsOwnArithmetic checks the stacking rather than the
// outcome: k parts advancing `adv` in total, joined at overlap `ov`, reach
// adv - (k-1)*ov. The brace's recipe is 5 parts advancing 65.00 with joints that
// compress between 1.00 and 5.00, so it spans 45.00 to 61.00 at one repetition —
// and a first version of this code fixed the overlap at the MINIMUM, which is the
// largest of those, jumping to 61.00 where the reference gives 42.50.
func TestTheAssemblyIsStackedByItsOwnArithmetic(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	gid, ok := e.font.GlyphIndex('{')
	if !ok {
		t.Skip("the face has no brace")
	}
	_, asm := e.face(10).MathVariants(gid, true)
	if asm == nil {
		t.Skip("the face has no assembly for the brace")
	}
	// The recipe this test's numbers come from, asserted so a face change is noticed
	// rather than silently changing what the bounds below mean.
	if len(asm.Parts) != 5 || asm.MinConnectorOverlap != 1 {
		t.Fatalf("recipe changed: %d parts, min overlap %d — re-derive the bounds",
			len(asm.Parts), asm.MinConnectorOverlap)
	}
	adv := 0.0
	for _, p := range asm.Parts {
		adv += float64(p.FullAdvance)
	}
	if math.Abs(adv-65) > 0.01 {
		t.Fatalf("parts advance %.2f in total, the bounds below assume 65.00", adv)
	}
	// A target inside [45.00, 61.00] must be met by compressing, not by adding parts.
	for _, target := range []float64{45, 50, 55, 61} {
		b := e.assembleVertical(asm, target, 10, clsOpen)
		if b == nil {
			t.Fatalf("target %.2f: no assembly", target)
		}
		if b.h < target-0.01 {
			t.Errorf("target %.2f: assembled to %.3f, which does not reach it", target, b.h)
		}
		if b.h > target+5 {
			t.Errorf("target %.2f: assembled to %.3f, overshooting by more than one "+
				"joint's worth — the overlap is not being solved for", target, b.h)
		}
	}
}

// TestTheAssemblyRefusesRecipesItCannotUse exercises the three guards that the
// shipped face never reaches. They are not dead code — assembleVertical takes the
// recipe as a PARAMETER, so any face can bring one that hits them — so they are
// tested with recipes rather than deleted.
//
// The one that matters is the third: a recipe whose extender adds nothing once its
// overlap is paid would make the search loop forever, and the bound of 64 would then
// silently cap the result instead of the caller learning the recipe is unusable.
func TestTheAssemblyRefusesRecipesItCannotUse(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	part := func(adv, start, end int, ext bool) opentype.MathAssemblyPart {
		return opentype.MathAssemblyPart{
			FullAdvance: adv, StartConnector: start, EndConnector: end, Extender: ext,
		}
	}
	for _, c := range []struct {
		name string
		asm  *opentype.MathAssembly
		want bool // an assembly is expected
	}{
		{"nil recipe", nil, false},
		{"no parts", &opentype.MathAssembly{MinConnectorOverlap: 1}, false},
		{"no extender", &opentype.MathAssembly{MinConnectorOverlap: 1,
			Parts: []opentype.MathAssemblyPart{part(10, 0, 5, false), part(10, 5, 0, false)}}, false},
		// The extender advances exactly what its two overlaps cost, so each repetition
		// adds nothing and no count can ever reach a larger target.
		{"extender that pays for itself", &opentype.MathAssembly{MinConnectorOverlap: 10,
			Parts: []opentype.MathAssemblyPart{part(10, 0, 10, false), part(10, 10, 10, true),
				part(10, 10, 0, false)}}, false},
		// The extender FIRST, which is the bar's own recipe shape in this face. That is
		// the only arrangement where its self-join can be strictly the tightest: with
		// an extender between two fixed parts, the joint before it is already
		// min(prev.End, ext.Start) and so never looser than min(ext.Start, ext.End).
		{"extender at the end of the chain", &opentype.MathAssembly{MinConnectorOverlap: 1,
			Parts: []opentype.MathAssemblyPart{part(20, 1, 8, true), part(20, 8, 0, false)}}, true},
		// Connectors below MinConnectorOverlap, so the maxOv floor applies.
		{"tight extender", &opentype.MathAssembly{MinConnectorOverlap: 4,
			Parts: []opentype.MathAssemblyPart{part(20, 0, 9, false), part(20, 2, 2, true),
				part(20, 9, 0, false)}}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := e.assembleVertical(c.asm, 100, 10, clsOpen)
			if got := b != nil; got != c.want {
				t.Errorf("assembly built = %v, want %v", got, c.want)
			}
			if b != nil && b.h <= 0 {
				t.Errorf("assembled height %.3f, an assembly has positive extent", b.h)
			}
		})
	}
}
