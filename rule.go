// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"fmt"
	"strconv"
	"strings"
)

// This file implements \rule in math mode.
//
// \rule is a LaTeX command, not a text-mode one: \rule{0pt}{2.6ex} inside an array
// cell or a \frac is the ordinary way to strut a row or open a fraction up, and
// \rule{0pt}{h} is how a formula is given a minimum height. Without it the command
// was unknown here and the caller dropped — in go-tex/engine, ABORTED — the whole
// document: `\[ \rule{40pt}{5pt} \]` produced a zero-byte PDF (go-tex/engine#294).

// parseRule implements \rule[lift]{width}{height}. The rule's bottom edge sits
// `lift` above the baseline (default 0) and it is `height` tall, so the box is
// `lift+height` high and, for a negative lift, `-lift` deep — LaTeX's own
// definition. A zero width or height draws nothing and still occupies the box, so
// \rule{0pt}{2.6ex} is an invisible strut, which is most of what it is used for.
func (e *engine) parseRule(toks []token, sty style) (*box, atomClass, bool, []token, error) {
	lift := 0.0
	if len(toks) > 0 && toks[0].kind == tChar && toks[0].r == '[' {
		s, rest, ok := readToBracket(toks[1:])
		if !ok {
			return nil, 0, false, nil, fmt.Errorf(`texmath: \rule: unterminated [lift]`)
		}
		lift, toks = e.dimen(s, sty), rest
	}
	ws, rest, err := readOpName(toks)
	if err != nil {
		return nil, 0, false, nil, fmt.Errorf(`texmath: \rule needs {width}{height}`)
	}
	hs, rest, err := readOpName(rest)
	if err != nil {
		return nil, 0, false, nil, fmt.Errorf(`texmath: \rule needs {width}{height}`)
	}
	w, h := e.dimen(ws, sty), e.dimen(hs, sty)

	out := newBox(clsOrd)
	out.w = w
	if out.h = h + lift; out.h < 0 {
		out.h = 0
	}
	if lift < 0 {
		out.d = -lift
	}
	if w > 0 && h > 0 {
		rule(out, 0, -(h + lift), w, h) // y grows downward; the baseline is 0
	}
	return out, clsOrd, false, rest, nil
}

// readToBracket collects the characters up to the matching ']'.
func readToBracket(toks []token) (string, []token, bool) {
	var sb strings.Builder
	for i, t := range toks {
		if t.kind == tChar && t.r == ']' {
			return sb.String(), toks[i+1:], true
		}
		switch t.kind {
		case tChar:
			sb.WriteString(t.text)
		case tCtrl:
			sb.WriteString(`\` + t.text)
		}
	}
	return "", nil, false
}

// texUnits are TeX's length units as exact rational multiples of a point,
// verbatim from tex.web:9019-9034 ("Scan for all other units"). pt is the base;
// sp is a point over 65536. em and ex come from the font and are handled by dimen.
var texUnits = map[string][2]float64{
	"pt": {1, 1},
	"in": {7227, 100},
	"pc": {12, 1},
	"cm": {7227, 254},
	"mm": {7227, 2540},
	"bp": {7227, 7200},
	"dd": {1238, 1157},
	"cc": {14856, 1157},
	"sp": {1, 65536},
}

// dimen reads a TeX length and returns it in the box coordinate system, which is
// points at the current size. An unreadable length is zero rather than an error:
// \rule is used for spacing, and a formula that loses a strut is far better than a
// document that loses the formula.
func (e *engine) dimen(s string, sty style) float64 {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] == '+' || s[i] == '-' || s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s[:i]), 64)
	if err != nil {
		return 0
	}
	unit := strings.ToLower(strings.TrimSpace(s[i:]))
	switch unit {
	case "", "pt":
		return v
	case "em":
		return v * float64(sty.px)
	case "ex":
		// 1ex is MEASURED — the height of "x" in this font at this size — not a
		// guessed fraction of the em. mustGlyph already stands in an empty box for
		// a font that has no "x", which reads as zero like any other length this
		// cannot work out.
		return v * e.mustGlyph('x', sty.px, clsOrd).h
	}
	if c, ok := texUnits[unit]; ok {
		return v * c[0] / c[1]
	}
	return 0
}
