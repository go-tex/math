// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "fmt"

type tokenKind uint8

const (
	tChar   tokenKind = iota // a single rune
	tCtrl                    // a control sequence \name
	tSup                     // ^
	tSub                     // _
	tLBrace                  // {
	tRBrace                  // }
)

type token struct {
	kind tokenKind
	text string // the rune, or the control-sequence name (without backslash)
	r    rune   // for tChar
}

// tokenize splits TeX math source into tokens, skipping ASCII spaces (math mode
// ignores them) and reading \name control sequences.
func tokenize(s string) []token {
	var out []token
	rs := []rune(s)
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			i++
		case c == '^':
			out = append(out, token{kind: tSup})
			i++
		case c == '_':
			out = append(out, token{kind: tSub})
			i++
		case c == '{':
			out = append(out, token{kind: tLBrace})
			i++
		case c == '}':
			out = append(out, token{kind: tRBrace})
			i++
		case c == '\\':
			i++
			start := i
			if i < len(rs) && !isLetter(rs[i]) { // control symbol like \{
				i++
				out = append(out, token{kind: tCtrl, text: string(rs[start:i])})
				continue
			}
			for i < len(rs) && isLetter(rs[i]) {
				i++
			}
			out = append(out, token{kind: tCtrl, text: string(rs[start:i])})
		default:
			out = append(out, token{kind: tChar, text: string(c), r: c})
			i++
		}
	}
	return out
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// parseRun lays out a sequence of atoms (each with optional scripts) into an
// hbox, stopping at a closing brace (left unconsumed) or the end of input.
func (e *engine) parseRun(toks []token, sizePx int, stopBrace bool) (*box, []token, error) {
	var items []*box
	for len(toks) > 0 {
		if toks[0].kind == tRBrace {
			if stopBrace {
				return hbox(items...), toks, nil
			}
			return nil, nil, fmt.Errorf("texmath: unexpected }")
		}
		nuc, rest, err := e.parseAtom(toks, sizePx)
		if err != nil {
			return nil, nil, err
		}
		toks = rest

		var sup, sub *box
		for len(toks) > 0 && (toks[0].kind == tSup || toks[0].kind == tSub) {
			k := toks[0].kind
			sc, rest2, err := e.parseAtom(toks[1:], e.scriptSize(sizePx))
			if err != nil {
				return nil, nil, err
			}
			toks = rest2
			if k == tSup {
				sup = sc
			} else {
				sub = sc
			}
		}
		if sup != nil || sub != nil {
			nuc = e.attachScripts(nuc, sup, sub, sizePx)
		}
		items = append(items, nuc)
	}
	if stopBrace {
		return nil, nil, fmt.Errorf("texmath: missing }")
	}
	return hbox(items...), toks, nil
}

// parseAtom parses one nucleus: a group {…}, a \frac, a control-sequence symbol,
// or a single character. It does not consume trailing scripts.
func (e *engine) parseAtom(toks []token, sizePx int) (*box, []token, error) {
	if len(toks) == 0 {
		return nil, nil, fmt.Errorf("texmath: expected an atom")
	}
	t := toks[0]
	switch t.kind {
	case tLBrace:
		b, rest, err := e.parseRun(toks[1:], sizePx, true)
		if err != nil {
			return nil, nil, err
		}
		return b, rest[1:], nil // consume the matching }
	case tRBrace, tSup, tSub:
		return nil, nil, fmt.Errorf("texmath: unexpected %q", tokenText(t))
	case tCtrl:
		if t.text == "frac" {
			num, rest, err := e.parseAtom(toks[1:], sizePx)
			if err != nil {
				return nil, nil, err
			}
			den, rest2, err := e.parseAtom(rest, sizePx)
			if err != nil {
				return nil, nil, err
			}
			return e.fraction(num, den, sizePx), rest2, nil
		}
		r, ok := symbols[t.text]
		if !ok {
			return nil, nil, fmt.Errorf(`texmath: unknown command \%s`, t.text)
		}
		return e.mustGlyph(r, sizePx), toks[1:], nil
	default: // tChar
		return e.mustGlyph(mathItalic(t.r), sizePx), toks[1:], nil
	}
}

// mustGlyph returns a glyph box, or a zero-width empty box if the font lacks the
// rune (so a missing glyph degrades gracefully rather than failing the render).
func (e *engine) mustGlyph(r rune, sizePx int) *box {
	if b, ok := e.glyphBox(r, sizePx); ok {
		return b
	}
	return &box{}
}

// tokenText names the punctuation tokens parseAtom can reject, for error
// messages (control words and chars are handled on their own paths).
func tokenText(t token) string {
	switch t.kind {
	case tRBrace:
		return "}"
	case tSup:
		return "^"
	default: // tSub
		return "_"
	}
}

// mathItalic maps ASCII letters to their Unicode mathematical-italic code
// points (the convention for variables), leaving other runes unchanged.
func mathItalic(r rune) rune {
	switch {
	case r == 'h':
		return 0x210E // PLANCK CONSTANT (the italic-h slot is a reserved hole)
	case r >= 'a' && r <= 'z':
		return 0x1D44E + (r - 'a')
	case r >= 'A' && r <= 'Z':
		return 0x1D434 + (r - 'A')
	default:
		return r
	}
}

// symbols maps a control-sequence name to its Unicode code point.
var symbols = map[string]rune{
	"alpha": 'α', "beta": 'β', "gamma": 'γ', "delta": 'δ', "epsilon": 'ε',
	"zeta": 'ζ', "eta": 'η', "theta": 'θ', "iota": 'ι', "kappa": 'κ',
	"lambda": 'λ', "mu": 'μ', "nu": 'ν', "xi": 'ξ', "pi": 'π', "rho": 'ρ',
	"sigma": 'σ', "tau": 'τ', "phi": 'φ', "chi": 'χ', "psi": 'ψ', "omega": 'ω',
	"Gamma": 'Γ', "Delta": 'Δ', "Theta": 'Θ', "Lambda": 'Λ', "Pi": 'Π',
	"Sigma": 'Σ', "Phi": 'Φ', "Psi": 'Ψ', "Omega": 'Ω',
	"sum": '∑', "prod": '∏', "int": '∫', "oint": '∮', "infty": '∞',
	"partial": '∂', "nabla": '∇', "sqrt": '√',
	"times": '×', "cdot": '⋅', "div": '÷', "pm": '±', "mp": '∓', "ast": '∗',
	"leq": '≤', "geq": '≥', "neq": '≠', "approx": '≈', "equiv": '≡', "sim": '∼',
	"rightarrow": '→', "leftarrow": '←', "Rightarrow": '⇒', "mapsto": '↦',
	"in": '∈', "notin": '∉', "subset": '⊂', "supset": '⊃',
	"cup": '∪', "cap": '∩', "forall": '∀', "exists": '∃', "emptyset": '∅',
	"langle": '⟨', "rangle": '⟩', "cdots": '⋯', "ldots": '…',
}
