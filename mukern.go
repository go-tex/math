// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"fmt"
	"strconv"
	"strings"
)

// \mkern<mudimen> and \mskip<muglue> — TeX primitives that insert horizontal space
// measured in MATH UNITS, where 18mu is one em of the current size. That is the same unit
// the space table already works in: \, is 3mu, \: 4mu, \; 5mu, \quad 18mu.
//
// They are here because amsmath's own \intkern@ is
//
//	\def\intkern@{\mkern-6mu\mathchoice{\mkern-3mu}{}{}{}}      amsmath.sty:654
//
// so every multiple integral in a document that loads amsmath carries two of them, and a
// corpus paper writes \mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu} directly. \mathchoice
// without them would select a branch it then cannot typeset.
//
// \mskip takes GLUE — a natural width with optional plus/minus stretch — and this layer
// has no glue, so only the natural width is used and any stretch is skipped. That is the
// same simplification the layer already makes for every other space: nothing here can
// stretch, so a formula's width is its natural width.
func (e *engine) parseMuKern(name string, toks []token, sty style) (*box, atomClass, bool, []token, error) {
	v, rest, err := readMuDimen(toks)
	if err != nil {
		return nil, 0, false, nil, fmt.Errorf(`texmath: \%s: %w`, name, err)
	}
	// 18mu = 1em = the current size in pixels.
	return e.kern(v * float64(sty.px) / 18), clsOrd, false, rest, nil
}

// readMuDimen reads a signed decimal followed by the unit "mu", plus — for \mskip — any
// "plus"/"minus" glue components, which are consumed and discarded.
//
// A missing or unreadable dimension is an ERROR and not a zero: \mkern with no argument
// would otherwise swallow nothing and leave the number in the formula, where it would be
// typeset as a digit. That is the failure mode this returns an error to avoid.
func readMuDimen(toks []token) (float64, []token, error) {
	var sb strings.Builder
	i := 0
	// No leading-space skip, and none inside the glue keywords below either: tokenize
	// drops every space before a token is emitted, so "\mskip3mu plus 1mu" arrives as
	// "3muplus1mu". A loop for spaces would be dead code that reads as a guard.
	for i < len(toks) && toks[i].kind == tChar {
		r := toks[i].r
		if r == '+' || r == '-' || r == '.' || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			i++
			continue
		}
		break
	}
	if sb.Len() == 0 {
		return 0, nil, fmt.Errorf("no dimension")
	}
	v, err := strconv.ParseFloat(sb.String(), 64)
	if err != nil {
		return 0, nil, fmt.Errorf("unreadable dimension %q", sb.String())
	}
	// The unit must be "mu": these two primitives take math units and nothing else.
	if i+1 >= len(toks) || toks[i].kind != tChar || toks[i+1].kind != tChar ||
		toks[i].r != 'm' || toks[i+1].r != 'u' {
		return 0, nil, fmt.Errorf("expected mu")
	}
	i += 2
	// \mskip's optional stretch and shrink: consumed and discarded, since this layer has
	// no glue. "plus 1mu minus 2mu" and "plus1fil" both go here.
	for _, kw := range []string{"plus", "minus"} {
		j := i
		if !hasWord(toks, j, kw) {
			continue
		}
		j += len(kw)
		if _, rest, err := readMuDimen(toks[j:]); err == nil {
			i = len(toks) - len(rest)
		} else {
			// ⛔ A glue component this cannot read — fil, fill, filll, or a length in a
			// unit other than mu — is skipped by its SHAPE: a number, then one KNOWN
			// unit spelling. Skipping "every letter" instead ate the material after the
			// glue: "\mskip3mu plus1fil b" came out 23.333 wide against 40.333, because
			// the b went with the fil. A test pins the width for exactly that reason.
			for j < len(toks) && toks[j].kind == tChar &&
				(toks[j].r == '.' || toks[j].r == '-' || toks[j].r == '+' ||
					(toks[j].r >= '0' && toks[j].r <= '9')) {
				j++
			}
			j += glueUnitLen(toks, j)
			i = j
		}
	}
	return v, toks[i:], nil
}

// hasWord reports whether the tokens at i spell out w as plain characters.
func hasWord(toks []token, i int, w string) bool {
	if i+len(w) > len(toks) {
		return false
	}
	for k, c := range w {
		if toks[i+k].kind != tChar || toks[i+k].r != c {
			return false
		}
	}
	return true
}

// glueUnitLen is the length of the unit spelling at i, or 0 if none is there. The infinite
// units come first because fill is a prefix of filll and fil of both: a shortest-first
// match would leave an l behind to be typeset.
func glueUnitLen(toks []token, i int) int {
	for _, u := range []string{"filll", "fill", "fil", "mu", "pt", "em", "ex",
		"in", "cm", "mm", "bp", "dd", "cc", "sp", "pc"} {
		if hasWord(toks, i, u) {
			return len(u)
		}
	}
	return 0
}
