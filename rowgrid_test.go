// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"math"
	"testing"
)

// gridLayout separated rows by a gap proportional to the point size (rowGap:
// p*0.4 and friends). TeX's model for anything built on \array — which is every
// matrix, and cases — is different in KIND: \@arstrut puts a rule of
// .7\baselineskip by .3\baselineskip in every row (latex.ltx:12103) and the rows
// stack on a baseline grid, so the pitch has a FLOOR of \baselineskip and opens up
// only for a row that exceeds it.
//
// What identifies it as the model and not the constant is that the error changed
// SIGN. \ht+\dp of a pmatrix, adding a third row to two rows of x, against
// tectonic at 10pt:
//
//	added row                     tectonic   before   after
//	flat (x)                        12.00      9.58   12.00
//	tall (\frac{\frac{a}{b}}{c})    13.95     16.74   13.95 → 14.20
//
// Raising rowGap to fix the first makes the second worse, which is why no tuned
// number was proposed. See go-tex/math#25.
//
// Both columns are asserted — the one-row total and the per-row increment — and
// the one-row case is not redundant: it is what caught \jot being folded into the
// STRUT rather than the pitch, which had aligned's pitch right at 15.00 and its
// single row wrong at 15.00 where the reference gives 12.00.
func TestRowsStackOnABaselineGrid(t *testing.T) {
	r := newRenderer(t)
	total := func(tex string) float64 {
		_, m, err := r.RenderSVGMetrics(tex, 10)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		return m.Height + m.Depth
	}
	for _, c := range []struct {
		name       string
		one, three string
		wantOne    float64 // \ht+\dp of one row, tectonic at 10pt
		wantPitch  float64 // per-row increment from 1 to 3 rows
	}{
		// The strut alone: one row of x is smaller than it, so the total IS the strut.
		{"array", `\begin{array}{l}x\end{array}`, `\begin{array}{l}x\\x\\x\end{array}`, 12, 12},
		{"matrix", `\begin{matrix}x\end{matrix}`, `\begin{matrix}x\\x\\x\end{matrix}`, 12, 12},
		{"pmatrix", `\begin{pmatrix}x\end{pmatrix}`, `\begin{pmatrix}x\\x\\x\end{pmatrix}`, 12, 12},
		{"bmatrix", `\begin{bmatrix}x\end{bmatrix}`, `\begin{bmatrix}x\\x\\x\end{bmatrix}`, 12, 12},
		{"vmatrix", `\begin{vmatrix}x\end{vmatrix}`, `\begin{vmatrix}x\\x\\x\end{vmatrix}`, 12, 12},
		// \openup\jot adds 3pt at 10pt to the PITCH and leaves the strut alone.
		{"aligned", `\begin{aligned}x &= y\end{aligned}`,
			`\begin{aligned}x &= y\\x &= y\\x &= y\end{aligned}`, 12, 15},
		{"gathered", `\begin{gathered}x\end{gathered}`,
			`\begin{gathered}x\\x\\x\end{gathered}`, 12, 15},
	} {
		t.Run(c.name, func(t *testing.T) {
			t1, t3 := total(c.one), total(c.three)
			pitch := (t3 - t1) / 2
			if math.Abs(t1-c.wantOne) > 0.02 {
				t.Errorf("une rangée = %.3f, tectonic donne %.2f", t1, c.wantOne)
			}
			if math.Abs(pitch-c.wantPitch) > 0.02 {
				t.Errorf("incrément par rangée = %.3f, tectonic donne %.2f (totaux %.3f et %.3f)",
					pitch, c.wantPitch, t1, t3)
			}
		})
	}
}

// A row taller than the strut opens the pitch by what it needs and no more, which
// is the half of §679 a floor alone does not give. The pitch is not asserted
// against a constant here — it is asserted to lie BETWEEN the flat pitch and the
// tall row's own size, which is the property that distinguishes a grid with a
// floor from either a fixed gap or a fixed pitch.
func TestATallRowOpensThePitchAndNoMore(t *testing.T) {
	r := newRenderer(t)
	total := func(tex string) float64 {
		_, m, err := r.RenderSVGMetrics(tex, 10)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		return m.Height + m.Depth
	}
	base := total(`\begin{pmatrix}x\\x\end{pmatrix}`)
	flat := total(`\begin{pmatrix}x\\x\\x\end{pmatrix}`) - base
	tall := total(`\begin{pmatrix}x\\x\\\frac{\frac{a}{b}}{c}\end{pmatrix}`) - base
	t.Logf("rangée ajoutée: plate %.3f, haute %.3f (tectonic: 12.00 et 13.95)", flat, tall)
	if math.Abs(flat-12) > 0.02 {
		t.Errorf("rangée plate ajoutée = %.3f, tectonic donne 12.00", flat)
	}
	if tall <= flat {
		t.Errorf("une rangée HAUTE (%.3f) doit ouvrir le pas plus qu'une plate (%.3f): "+
			"un pas fixe ne suivrait pas le contenu", tall, flat)
	}
	// Bounded against the reference's own opening rather than a round number: the
	// reference opens by 13.95-12.00 = 1.95 and we open by 2.20, a 0.25pt residual
	// on the fraction's own height that this change does not address. 0.4pt of slack
	// admits that and still refuses the old behaviour, which opened by
	// 16.74-9.58 = 7.16 — a gap proportional to the size, with no floor.
	const refOpening = 13.95 - 12.00
	if d := (tall - flat) - refOpening; d > 0.4 || d < -0.4 {
		t.Errorf("une rangée haute ouvre le pas de %.3f, tectonic de %.2f (écart %+.3f)",
			tall-flat, refOpening, d)
	}
}

// Left alone deliberately, with their reference values recorded so the next
// measurement starts from here rather than from scratch:
//
//	              one row              pitch
//	              tectonic  here       tectonic  here
//	cases           18.00   12.00        12.60   12.00
//	smallmatrix      4.01    4.71         5.50    5.23
//
// cases moved from 9.33/10.84 to 12.00/12.00 — closer in both columns, exact in
// neither. Its one-row total of 18.00 is now fully decomposed, and the first
// version of this comment got it wrong (it said "NOT the brace"):
//
//	array, plain                                12.00   the strut
//	array with \arraystretch{1.2}               14.40   = 1.2 x 12.00
//	  plus \left\lbrace                         18.00   = 14.40 + 3.60
//	cases itself                                18.00   h=11.50 d=6.50, identical
//
// amsmath:1106 is \left\lbrace \def\arraystretch{1.2} \array{@{}l@{\quad}l@{}},
// and rebuilding exactly that reproduces cases to the last digit including the
// height/depth split. So the brace contributes 3.60 of the 6.00 and \arraystretch
// the other 2.40.
//
// ⚠ The refutation that produced the old comment tested \left\{ x \right. — a
// TINY content, where the brace measures 10.00pt exactly as \left( x \right) does
// — and generalised from it. A \left delimiter grows with what it spans
// (\delimiterfactor, \delimitershortfall), so the one operating point where it is
// smallest says nothing about the one that matters. Test a hypothesis where the
// quantity is LARGE.
//
// The pitch of 12.60 is still unexplained: \arraystretch{1.2} on a strut of
// 10.08/4.32 does not give it under §679 with \baselineskip 12 (that would be
// 15.40), so something else sets the leading inside \array. Not guessed at here.
//
// smallmatrix is untouched: it is set at script size and wants a smaller strut.
func TestCasesAndSmallmatrixAreNotYetOnTheGrid(t *testing.T) {
	r := newRenderer(t)
	total := func(tex string) float64 {
		_, m, err := r.RenderSVGMetrics(tex, 10)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		return m.Height + m.Depth
	}
	// Pinned to what this engine produces, not to the reference: the point is to
	// notice a change here, not to claim it is right.
	for _, c := range []struct {
		name string
		tex  string
		want float64
	}{
		{"cases", `\begin{cases}x & y\end{cases}`, 12.00},
		{"smallmatrix", `\begin{smallmatrix}x\end{smallmatrix}`, 4.712},
	} {
		if v := total(c.tex); math.Abs(v-c.want) > 0.02 {
			t.Errorf("%s une rangée = %.3f, cet état vaut %.3f — si c'est voulu, "+
				"mettre à jour ici ET la table de la référence au-dessus", c.name, v, c.want)
		}
	}
}
