// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "errors"

// \mathchoice{display}{text}{script}{scriptscript} — a TeX primitive that typesets ONE
// of four bodies, chosen by the style in force, and discards the other three.
//
// The order is confirmed by two independent kernel uses rather than taken on trust:
//
//	latex.ltx:11179-11184  \def\mathpalette#1#2{\mathchoice
//	                         {#1\displaystyle{#2}}{#1\textstyle{#2}}
//	                         {#1\scriptstyle{#2}}{#1\scriptscriptstyle{#2}}}
//	amsmath.sty:266-270    \mathchoice{…\textfont…}{…\textfont…}
//	                         {…\scriptfont…}{…\scriptscriptfont…}
//
// The first names the four styles outright; the second gives the same order in fonts, and
// its first TWO branches both use \textfont — display and text share a size, which is why
// a size alone cannot select and the style LEVEL had to be added to style (math.go).
//
// 183 equations over 7 papers of a 999-paper census (go-tex/engine#466), 6 of the 7
// writing it in their own .tex. Unlike a wrapper, a SELECTOR discards three quarters of
// its content, so the argument it keeps is the only one that has to render — and the real
// corpus forms bear that out: amsmath's own \intkern@ is
// \mathchoice{\mkern-3mu}{}{}{} and a paper writes \mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu},
// both EMPTY in the branch a display picks.
func (e *engine) parseMathchoice(toks []token, sty style) (*box, atomClass, bool, []token, error) {
	// Read all four groups whatever the style, because they must all be CONSUMED: a
	// selector that reads only the branch it wants leaves the other three in the stream
	// to be typeset as ordinary material.
	var groups [4][]token
	rest := toks
	for i := 0; i < 4; i++ {
		g, r, ok := readGroupToks(rest)
		if !ok {
			// Fewer than four groups is not a \mathchoice. Leave the whole thing for the
			// caller to report rather than guessing which branches were meant: a
			// selector with a missing branch would silently pick the wrong body.
			return nil, 0, false, nil, errMathchoiceArity
		}
		groups[i], rest = g, r
	}
	b, _, err := e.parseList(groups[mathchoiceBranch(sty)], sty, stopEnd)
	if err != nil {
		return nil, 0, false, nil, err
	}
	return b, clsOrd, false, rest, nil
}

// mathchoiceBranch is the index \mathchoice selects: 0 display, 1 text, 2 script,
// 3 scriptscript.
//
// TeX has eight styles and the cramped variants (D' T' S' SS') choose as their uncramped
// partners do, so the script LEVEL plus the display flag is all this needs. Level 2 and
// beyond is scriptscript, because TeX has no style below it — a script of a
// scriptscript is scriptscript again.
func mathchoiceBranch(sty style) int {
	switch {
	case sty.level == 0 && sty.display:
		return 0
	case sty.level == 0:
		return 1
	case sty.level == 1:
		return 2
	default:
		return 3
	}
}

// errMathchoiceArity is returned when fewer than four groups follow. It is a named error
// because the caller's retry chain distinguishes "this construct is malformed" from "an
// unknown command": a malformed \mathchoice must not be retried by stripping it.
var errMathchoiceArity = errors.New(`texmath: \mathchoice needs four {…} groups`)

// readGroupToks reads one brace-delimited group as a token slice, balanced, and returns
// what follows. Leading spaces are skipped, the way TeX skips them before an argument.
//
// It returns the group's CONTENTS without the braces, so \mathchoice's branch can be
// parsed as a list in its own right — and ok=false when the next token is not an opening
// brace, which is how the arity check above works.
func readGroupToks(toks []token) (group, rest []token, ok bool) {
	// No leading-space skip: tokenize drops every space, tab and newline before a token
	// is ever emitted, so a tChar space cannot reach here. A loop for it would be dead
	// code that reads as a guard.
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return nil, toks, false
	}
	depth := 0
	for i, t := range toks {
		switch t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			depth--
			if depth == 0 {
				return toks[1:i], toks[i+1:], true
			}
		}
	}
	return nil, toks, false // unbalanced: no closing brace
}

// mathchoiceEmptyBranch reports whether the branch this style selects is EMPTY, and
// returns what follows the four groups so the caller can skip the whole construct.
//
// It reads without committing: on anything malformed it returns empty=false and leaves the
// tokens alone, so parseControl sees exactly what it would have seen and reports the arity
// error itself. Two readers of the same four groups is the price of keeping the
// no-atom case out of the box-returning path.
func (e *engine) mathchoiceEmptyBranch(toks []token, sty style) ([]token, bool) {
	var groups [4][]token
	rest := toks
	for i := 0; i < 4; i++ {
		g, r, ok := readGroupToks(rest)
		if !ok {
			return toks, false
		}
		groups[i], rest = g, r
	}
	if len(groups[mathchoiceBranch(sty)]) != 0 {
		return toks, false
	}
	return rest, true
}
