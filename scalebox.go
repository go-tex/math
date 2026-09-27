// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"fmt"
	"strconv"
	"strings"
)

// \scalebox{h}[v]{content} scales a box, and \reflectbox mirrors it. graphics.sty gives
// both exactly:
//
//	:519  \protected\def\scalebox#1{\@ifnextchar[{\Gscale@box{#1}}{\Gscale@box{#1}[#1]}}
//	:521  \long\def\Gscale@box#1[#2]#3{…}
//	:538  \protected\def\reflectbox{\Gscale@box-1[1]}
//
// So the vertical factor defaults to the horizontal one, and \reflectbox is
// \scalebox{-1}[1] — a horizontal mirror, which falls out of the same code.
//
// It is the largest single item in the dropped-equation census: 3624 equations across 16
// papers of a 999-paper sample, 23% of every equation dropped. The factors those papers
// write are 0.25 to 0.9, mostly 0.5 to 0.8, so stripping the command and keeping the
// content — this repository's usual fallback — would set the material at up to four
// times its intended size. A real scale is barely more work, because place() already
// emits an SVG transform.
func (e *engine) scaleBox(b *box, h, v float64) *box {
	out := newBox(b.cls)
	// A negative horizontal factor mirrors about x=0, so the content would draw to the
	// LEFT of the origin. graphics.sty:534-536 compensates with a kern inside an \hbox of
	// the scaled width; the same correction here is a translate of that width.
	pre := ""
	if h < 0 {
		pre = fmt.Sprintf("translate(%s,0) ", ftoa(-h*b.w))
	}
	fmt.Fprintf(&out.svg, `<g transform="%sscale(%s,%s)">%s</g>`,
		pre, ftoa(h), ftoa(v), b.String())
	// graphics.sty:526-532: a negative vertical factor SWAPS height and depth, because
	// scaling about the baseline turns what was above it into what is below.
	if v < 0 {
		out.h, out.d = -v*b.d, -v*b.h
	} else {
		out.h, out.d = v*b.h, v*b.d
	}
	// :533-536: the width is |h| times the original either way.
	if h < 0 {
		out.w = -h * b.w
	} else {
		out.w = h * b.w
	}
	return out
}

// readScaleFactor reads one scale factor: a {group} or a bare run of number characters,
// the latter because \reflectbox's own definition writes -1 unbraced.
//
// A factor that does not parse is an ERROR and not a silent 1: scaling by the wrong
// amount is invisible on the page, while a refused command is one the census can rank.
func readScaleFactor(toks []token) (float64, []token, error) {
	txt, rest := readArgText(toks)
	v, err := strconv.ParseFloat(strings.TrimSpace(txt), 64)
	if err != nil {
		return 0, nil, fmt.Errorf("texmath: \\scalebox factor %q is not a number", txt)
	}
	return v, rest, nil
}

// readOptScaleFactor reads \scalebox's optional [v] and reports whether it was there.
func readOptScaleFactor(toks []token) (float64, []token, bool) {
	if len(toks) == 0 || toks[0].kind != tChar || toks[0].r != '[' {
		return 0, toks, false
	}
	var b strings.Builder
	i := 1
	for ; i < len(toks) && !(toks[i].kind == tChar && toks[i].r == ']'); i++ {
		if toks[i].kind == tChar {
			b.WriteRune(toks[i].r)
		}
	}
	if i >= len(toks) {
		return 0, toks, false // unclosed [ : not an optional argument
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(b.String()), 64)
	if err != nil {
		return 0, toks, false
	}
	return v, toks[i+1:], true
}
