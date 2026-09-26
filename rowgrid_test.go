// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"math"
	"testing"
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
//	cases            18.00    9.33    16.67                43.20   31.01   43.20
//	aligned          12.00    7.02    12.00                42.00   31.06   42.00
//	gathered         12.00    5.45    12.00                42.00   24.67   42.00
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
		// cases carries \arraystretch 1.2, so 3 x 14.40. Its one row is the brace's.
		{"cases", `\begin{cases}x & y\end{cases}`,
			`\begin{cases}x & y\\x & y\\x & y\end{cases}`, 16.67, 43.20},
		// \jot adds 3pt at 10pt BETWEEN rows: 12 + 2 x 15.
		{"aligned", `\begin{aligned}x &= y\end{aligned}`,
			`\begin{aligned}x &= y\\x &= y\\x &= y\end{aligned}`, 12, 42},
		{"gathered", `\begin{gathered}x\end{gathered}`,
			`\begin{gathered}x\\x\\x\end{gathered}`, 12, 42},
	} {
		t.Run(c.name, func(t *testing.T) {
			if v := envTotal(t, r, c.one); math.Abs(v-c.wantOne) > 0.05 {
				t.Errorf("une rangée = %.3f, attendu %.2f", v, c.wantOne)
			}
			if v := envTotal(t, r, c.three); math.Abs(v-c.wantThree) > 0.05 {
				t.Errorf("trois rangées = %.3f, attendu %.2f", v, c.wantThree)
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
	// cases' rows are 14.40pt: three of them come to 43.20, which is 3 x 14.40 and
	// not 3 x 12.00 (the unstretched strut) nor 3 x 15.40 (§679 against a 12pt
	// leading, the model this replaced).
	three := envTotal(t, r, `\begin{cases}x & y\\x & y\\x & y\end{cases}`)
	if math.Abs(three-43.20) > 0.05 {
		t.Errorf("cases trois rangées = %.3f, tectonic donne 43.20 (= 3 x 14.40); "+
			"36.00 voudrait dire que le stretch est ignoré, et 45.20 est ce que "+
			"produit le modèle §679 contre un interligne de 12pt — mesuré en le "+
			"remettant, pas calculé", three)
	}
	// And the unstretched control, so this is a statement about the stretch and not
	// about matrices in general.
	if v := envTotal(t, r, `\begin{pmatrix}x\\x\\x\end{pmatrix}`); math.Abs(v-36) > 0.05 {
		t.Errorf("pmatrix trois rangées = %.3f, tectonic donne 36.00", v)
	}
}

// A row taller than the strut takes its own height, which is the other half of the
// model: the strut is a FLOOR, not a fixed pitch.
func TestATallRowTakesItsOwnHeight(t *testing.T) {
	r := newRenderer(t)
	base := envTotal(t, r, `\begin{pmatrix}x\\x\end{pmatrix}`)
	flat := envTotal(t, r, `\begin{pmatrix}x\\x\\x\end{pmatrix}`) - base
	tall := envTotal(t, r, `\begin{pmatrix}x\\x\\\frac{\frac{a}{b}}{c}\end{pmatrix}`) - base
	t.Logf("rangée ajoutée: plate %.3f, haute %.3f (tectonic: 12.00 et 13.95)", flat, tall)
	if math.Abs(flat-12) > 0.05 {
		t.Errorf("rangée plate ajoutée = %.3f, tectonic donne 12.00", flat)
	}
	// Bounded against the reference's own opening, 13.95-12.00, rather than a round
	// number: we open by 2.20 against its 1.95, a 0.25pt residual on the fraction's
	// height that this change does not address. 0.4pt of slack admits it and still
	// refuses the old behaviour, which opened by 16.74-9.58 = 7.16.
	const refOpening = 13.95 - 12.00
	if d := (tall - flat) - refOpening; d > 0.4 || d < -0.4 {
		t.Errorf("une rangée haute ouvre le pas de %.3f, tectonic de %.2f (écart %+.3f)",
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
		t.Errorf("l'accolade n'élève pas le total: %.3f contre %.2f pour les rangées seules",
			full, rows)
	}
	if math.Abs(full-16.67) > 0.05 {
		t.Errorf("cases une rangée = %.3f, cet état vaut 16.67 (tectonic 18.00) — "+
			"si c'est voulu, mettre à jour ici ET le tableau au-dessus", full)
	}
}

// smallmatrix is untouched: script size, and it wants a smaller strut.
//
//	             one row   three rows
//	tectonic        4.01        15.01
//	here            4.71        15.17
//
// Pinned to this engine's values so a change is noticed, not claimed to be right.
func TestSmallmatrixIsNotYetOnTheGrid(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{`\begin{smallmatrix}x\end{smallmatrix}`, 4.712},
		{`\begin{smallmatrix}x\\x\\x\end{smallmatrix}`, 15.169},
	} {
		if v := envTotal(t, r, c.tex); math.Abs(v-c.want) > 0.05 {
			t.Errorf("%s = %.3f, cet état vaut %.3f", c.tex, v, c.want)
		}
	}
}
