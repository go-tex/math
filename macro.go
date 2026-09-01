// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

// This file gives the math layer a TeX "mouth": a pass that executes \def
// definitions and expands their uses before the parser sees the stream. A paper's
// own inline shorthand — \def\R{\mathbb{R}}\R, or \def\abs#1{\lvert#1\rvert}\abs{x}
// written inside a formula — would otherwise reach the parser as an unknown \def
// (or an unknown \R) and drop the whole equation. Macros defined in the preamble
// are already substituted upstream by go-tex/engine; this handles the ones that
// live inside the formula itself.
//
// Scope: undelimited parameters (#1…#9) only, which is the inline form that
// occurs. A \def whose parameter text carries delimiters, or a use with too few
// arguments, is left verbatim for the parser rather than guessed at.

// macroDef is one \def'd macro: nargs undelimited parameters and the replacement
// body, in which a #n token pair marks where argument n is spliced.
type macroDef struct {
	nargs int
	body  []token
}

// expandMacros runs the mouth pass. It walks toks left to right, recording each
// \def and replacing each use of a defined macro with its body (arguments spliced
// into #1…#n), re-processing the result so nested and chained macros resolve. A
// budget bounds the number of expansions so a self- or mutually-recursive macro
// (\def\x{\x}) cannot spin forever — it stops expanding that name and lets the
// parser render what stands.
func expandMacros(toks []token) []token {
	macros := map[string]macroDef{}
	out := make([]token, 0, len(toks))
	budget := 100000
	for len(toks) > 0 {
		t := toks[0]
		if t.kind == tCtrl && t.text == "def" {
			if def, name, rest, ok := parseDef(toks[1:]); ok {
				macros[name] = def
				toks = rest
				continue
			}
			// A \def the pass cannot read (delimited parameters, no body): leave it
			// for the parser, which reports it rather than mis-expanding.
			out = append(out, t)
			toks = toks[1:]
			continue
		}
		if t.kind == tCtrl {
			if m, ok := macros[t.text]; ok {
				if budget <= 0 {
					// Runaway guard tripped: stop expanding this name, emit it verbatim.
					out = append(out, t)
					toks = toks[1:]
					continue
				}
				if sub, rest, ok := substituteMacro(m, toks[1:]); ok {
					budget--
					toks = append(sub, rest...)
					continue
				}
				// Too few arguments to expand: leave the name for the parser.
			}
		}
		out = append(out, t)
		toks = toks[1:]
	}
	return out
}

// parseDef reads a \def body starting just after the \def token: the macro name
// (a control sequence), its undelimited parameter text (#1…#n in order), and its
// {…} replacement. ok is false for a name that is not a control sequence, a
// parameter text with anything but #1…#n in order (a delimited macro), or a
// missing/unbalanced body.
func parseDef(toks []token) (def macroDef, name string, rest []token, ok bool) {
	if len(toks) == 0 || toks[0].kind != tCtrl {
		return macroDef{}, "", nil, false
	}
	name = toks[0].text
	toks = toks[1:]
	nargs := 0
	for len(toks) > 0 && toks[0].kind != tLBrace {
		if toks[0].kind == tChar && toks[0].r == '#' && len(toks) > 1 &&
			toks[1].kind == tChar && toks[1].r == rune('1'+nargs) {
			nargs++
			toks = toks[2:]
			continue
		}
		return macroDef{}, "", nil, false
	}
	body, rest, gok := readGroup(toks)
	if !gok {
		return macroDef{}, "", nil, false
	}
	return macroDef{nargs: nargs, body: body}, name, rest, true
}

// substituteMacro reads m.nargs arguments from toks (a {group} or a single token
// each, as TeX takes them) and returns the body with #n replaced by argument n,
// plus the remaining tokens. ok is false when the stream runs out of arguments.
func substituteMacro(m macroDef, toks []token) (sub []token, rest []token, ok bool) {
	args := make([][]token, m.nargs)
	for i := 0; i < m.nargs; i++ {
		if len(toks) == 0 {
			return nil, nil, false
		}
		if toks[0].kind == tLBrace {
			g, r, gok := readGroup(toks)
			if !gok {
				return nil, nil, false
			}
			args[i], toks = g, r
		} else {
			args[i], toks = []token{toks[0]}, toks[1:]
		}
	}
	for i := 0; i < len(m.body); i++ {
		b := m.body[i]
		if b.kind == tChar && b.r == '#' && i+1 < len(m.body) &&
			m.body[i+1].kind == tChar && m.body[i+1].r >= '1' && m.body[i+1].r <= '9' {
			if d := int(m.body[i+1].r - '1'); d < len(args) {
				sub = append(sub, args[d]...)
			}
			i++ // consume the digit
			continue
		}
		sub = append(sub, b)
	}
	return sub, toks, true
}

// readGroup returns the tokens inside a leading {…} group (braces stripped) and
// the tokens after it. ok is false when toks does not start with { or the group
// is unbalanced.
func readGroup(toks []token) (inner []token, rest []token, ok bool) {
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return nil, nil, false
	}
	depth := 0
	for i, t := range toks {
		switch t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			if depth--; depth == 0 {
				return toks[1:i], toks[i+1:], true
			}
		}
	}
	return nil, nil, false
}
