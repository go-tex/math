// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

// leading says HOW consecutive rows of a grid are separated. TeX has three models
// here and they differ in KIND, not by a constant — which is why each is a type of
// its own rather than a set of fields on one struct.
//
// That shape is the fix for a defect this package already shipped. gridOpts had a
// single \baselineskip field, so the strutted model and the interline-glue model
// looked like the same model with different numbers; the glue model was written
// first, agrees with the strutted one at \arraystretch 1, and every test held the
// stretch at 1. Seven environments measured exact on the wrong model (#26, #28).
// With three types, an environment cannot be given \arraystretch and \lineskip at
// once, and choosing a model is a visible decision at the call site.
type leading interface {
	// strut is the minimum height and depth every row is raised to, before any gap
	// is computed. \array and its descendants get theirs from \@arstrut; the other
	// two models have none.
	strut() (h, d float64)
	// gap is the space inserted between a row of depth prevDepth and the next of
	// height height. Only the interline-glue model reads its arguments.
	gap(prevDepth, height float64) float64
}

// flatLeading separates rows by a fixed gap. It is not a TeX model — it is what
// this package did everywhere before the grid landed, and it survives only where
// no reference measurement has been made yet (\substack).
type flatLeading struct{ rowGap float64 }

func (l flatLeading) strut() (float64, float64) { return 0, 0 }
func (l flatLeading) gap(_, _ float64) float64  { return l.rowGap }

// strutLeading is the model of \array and everything built on it: matrix, pmatrix
// and friends, cases, aligned, gathered. \@arstrut puts a rule of
// \arraystretch(.7/.3 of \baselineskip) in every row (latex.ltx:12103) and the
// rows BUTT — the struts alone set the pitch. \jot, where the environment has one,
// is the only thing added between them.
//
// The pitch therefore tracks \arraystretch LINEARLY, which is what separates this
// model from glueLeading:
//
//	\arraystretch   1 row    pitch     glue model would give
//	1.0             12.00    12.00     12.00   agrees
//	1.2             14.40    14.40     15.40   wrong
//	1.5             18.00    18.00     19.00   wrong
type strutLeading struct {
	baselineskip float64
	jot          float64 // added to the PITCH, never to the strut
	stretch      float64 // \arraystretch; zero means 1
}

func (l strutLeading) strut() (float64, float64) {
	st := l.stretch
	if st <= 0 {
		st = 1
	}
	return st * 0.7 * l.baselineskip, st * 0.3 * l.baselineskip
}

func (l strutLeading) gap(_, _ float64) float64 { return l.jot }

// glueLeading is TeX's interline glue, tex.web §679: between a box of depth
// prev_depth and the next of height h, append \baselineskip - prev_depth - h, but
// substitute \lineskip when that falls below \lineskiplimit. There is no strut, so
// the rows' own extents decide which branch is taken.
//
// amsmath's smallmatrix is the one environment here that takes it: it sets
// \baselineskip6\ex@ \lineskip1.5\ex@ \lineskiplimit\lineskip and struts nothing
// (amsmath.sty:1045-1047).
type glueLeading struct {
	baselineskip  float64
	lineskip      float64
	lineskiplimit float64
}

func (l glueLeading) strut() (float64, float64) { return 0, 0 }

func (l glueLeading) gap(prevDepth, height float64) float64 {
	if g := l.baselineskip - prevDepth - height; g >= l.lineskiplimit {
		return g
	}
	return l.lineskip
}
