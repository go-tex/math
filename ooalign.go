// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "errors"

// \ooalign{row\cr row\cr} — plain TeX's OVERLAY: every row is set at the SAME baseline,
// one on top of another, and the result is as wide as the widest row that declares a
// width. It is how a paper draws a symbol the fonts do not have, by stacking two it does.
//
// latex.ltx gives it in four lines, and each one is load-bearing:
//
//	:492  \newskip\hideskip \hideskip=-1000pt plus 1fill  % negative but can grow
//	:615  \def\hidewidth{\hskip\hideskip}
//	:624  \def\ialign{\everycr{}\tabskip\z@skip\halign}
//	:625  \def\oalign#1{\leavevmode\vtop{\baselineskip\z@skip \lineskip.25ex%
//	        \ialign{##\crcr#1\crcr}}}
//	:628  \def\ooalign{\lineskiplimit-\maxdimen \oalign}
//
// The OVERLAY comes from the last line, not from \oalign. \baselineskip is zero, so the
// interline glue tex.web §679 computes is 0 − prevdepth − height, which is negative; and
// \lineskiplimit of −\maxdimen means a negative gap is never below the limit, so
// \baselineskip is used rather than \lineskip. Every row therefore lands on the same
// baseline. This layer already models that rule three ways (leading.go), and \ooalign is
// the glueLeading case with baselineskip 0 and an unreachable limit.
//
// ⛔ Its siblings are NOT implemented, and the reason is a principle rather than effort:
// \o@lign sets \lineskiplimit\z@ and a bare \oalign sets nothing, so both depend on the
// AMBIENT \lineskiplimit — state this layer does not carry. \ooalign fixes its own limit
// and is therefore self-contained. A construct whose result depends on state we do not
// model cannot be implemented correctly, only plausibly.
//
// 102 equations over 4 papers of a 999-paper census (go-tex/engine#466), all four writing
// it in their own .tex. It became visible only when \mathpalette landed (go-tex/math#49,
// go-tex/engine#502) and stopped masking it — one of the papers writes
//
//	\newcommand*{\@cupdot}[2]{\ooalign{$\m@th#1\cup$\cr \hidewidth$\m@th#1\cdot$\hidewidth}}
//
// which is a \cup with a \cdot centred on it.
func (e *engine) parseOoalign(toks []token, sty style) (*box, atomClass, bool, []token, error) {
	body, rest, ok := readGroupToks(toks)
	if !ok {
		return nil, 0, false, nil, errOoalignArg
	}
	type orow struct {
		b            *box
		hideL, hideR bool
	}
	var rows []orow
	for _, r := range splitOoalignRows(body) {
		hl, hr, inner := stripHidewidth(r)
		b, _, err := e.parseList(inner, sty, stopEnd)
		if err != nil {
			return nil, 0, false, nil, err
		}
		rows = append(rows, orow{b, hl, hr})
	}
	if len(rows) == 0 {
		return newBox(clsOrd), clsOrd, false, rest, nil
	}
	// The column is as wide as the widest row that DECLARES a width. \hidewidth is a
	// −1000pt skip with infinite stretch, so a row flanked by two of them contributes
	// nothing to the column and is centred in it; one on the left alone pushes the row
	// right. When every row is hidden there is no declared width, and the widest row is
	// the only sensible column — otherwise the overlay would collapse to zero.
	w := 0.0
	for _, r := range rows {
		if !(r.hideL && r.hideR) && r.b.w > w {
			w = r.b.w
		}
	}
	if w == 0 {
		for _, r := range rows {
			if r.b.w > w {
				w = r.b.w
			}
		}
	}
	out := newBox(clsOrd)
	for _, r := range rows {
		x := 0.0
		switch {
		case r.hideL && r.hideR:
			x = (w - r.b.w) / 2
		case r.hideL:
			x = w - r.b.w
		}
		// dy is ZERO for every row. That single fact is the overlay, and it is what the
		// −\maxdimen limit above buys.
		place(out, r.b, x, 0)
		if r.b.h > out.h {
			out.h = r.b.h
		}
		if r.b.d > out.d {
			out.d = r.b.d
		}
	}
	out.w = w
	return out, clsOrd, false, rest, nil
}

// splitOoalignRows splits on \cr and \crcr, at brace depth zero, and drops a trailing
// empty row — which \oalign's own template produces.
//
// ⛔ The brace depth is currently UNOBSERVABLE and kept anyway. Every input that would
// distinguish it fails the same way with or without it: a \cr inside braces belongs to a
// nested alignment, this layer has none, so the row is reported either as an unknown \cr
// or as an unbalanced group. An ablation of the depth counter breaks no test.
//
// It stays because the day a nested alignment exists, splitting inside one would tear it
// in half — silently, since both halves would parse. A guard whose job is to survive a
// change to its surroundings is worth more than its current test coverage, and saying so
// is better than leaving it looking tested.
//
// It also drops a trailing — which \oalign's own template produces, since it writes {##\crcr#1\crcr} and
// papers end their last row with \cr as well.
func splitOoalignRows(toks []token) [][]token {
	var rows [][]token
	depth, start := 0, 0
	for i, t := range toks {
		switch {
		case t.kind == tLBrace:
			depth++
		case t.kind == tRBrace:
			depth--
		case depth == 0 && t.kind == tCtrl && (t.text == "cr" || t.text == "crcr"):
			rows = append(rows, toks[start:i])
			start = i + 1
		}
	}
	if start < len(toks) {
		rows = append(rows, toks[start:])
	}
	// Drop rows that are entirely empty: a trailing \cr leaves one, and an empty row in
	// an overlay contributes nothing but would still count toward the width test above.
	out := rows[:0]
	for _, r := range rows {
		if len(r) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// stripHidewidth removes a leading and/or trailing \hidewidth and says which were there.
func stripHidewidth(toks []token) (hideL, hideR bool, inner []token) {
	for len(toks) > 0 && toks[0].kind == tCtrl && toks[0].text == "hidewidth" {
		hideL = true
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].kind == tCtrl && toks[len(toks)-1].text == "hidewidth" {
		hideR = true
		toks = toks[:len(toks)-1]
	}
	return hideL, hideR, toks
}

// errOoalignArg is returned when no brace group follows. Named, so the retry chain can
// tell "this construct is malformed" from "an unknown command": a malformed \ooalign must
// not be retried by stripping it, because the rows it holds are the content.
var errOoalignArg = errors.New(`texmath: \ooalign needs a {…} group`)
