// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "fmt"

// ── tokens ──────────────────────────────────────────────────────────────────

type tokenKind uint8

const (
	tChar   tokenKind = iota // a single rune
	tCtrl                    // a control sequence \name (or control symbol)
	tSup                     // ^
	tSub                     // _
	tLBrace                  // {
	tRBrace                  // }
	tAmp                     // & (matrix column separator)
	tPrime                   // ' (prime)
)

type token struct {
	kind tokenKind
	text string
	r    rune
}

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
		case c == '&':
			out = append(out, token{kind: tAmp})
			i++
		case c == '\'':
			out = append(out, token{kind: tPrime})
			i++
		case c == '\\':
			i++
			start := i
			if i < len(rs) && !isLetter(rs[i]) { // control symbol like \{ \\ \, \|
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

func tokenText(t token) string {
	switch t.kind {
	case tCtrl:
		return "\\" + t.text
	case tRBrace:
		return "}"
	case tSup:
		return "^"
	case tSub:
		return "_"
	case tAmp:
		return "&"
	case tPrime:
		return "'"
	default:
		return t.text
	}
}

// ── parsing ─────────────────────────────────────────────────────────────────

type stopMode uint8

const (
	stopEnd   stopMode = iota // only end of input
	stopBrace                 // stop at }
	stopRight                 // stop at \right
	stopCell                  // stop at & \\ or \end (matrix cell)
)

// atStop reports whether toks begins with a terminator for the given mode.
func atStop(toks []token, stop stopMode) bool {
	if len(toks) == 0 {
		return true
	}
	t := toks[0]
	switch stop {
	case stopBrace:
		return t.kind == tRBrace
	case stopRight:
		return t.kind == tCtrl && t.text == "right"
	case stopCell:
		return t.kind == tAmp || (t.kind == tCtrl && (t.text == `\` || t.text == "end"))
	default:
		return false
	}
}

// parseList lays out a run of atoms (each with optional scripts/limits/primes)
// into an hlist, stopping (without consuming) at the terminator for stop.
func (e *engine) parseList(toks []token, sty style, stop stopMode) (*box, []token, error) {
	var items []*box
	for {
		// style switches apply to the remainder of the list.
		if len(toks) > 0 && toks[0].kind == tCtrl {
			switch toks[0].text {
			case "displaystyle":
				sty.display, sty.spacious, toks = true, true, toks[1:]
				continue
			case "textstyle":
				sty.display, toks = false, toks[1:]
				continue
			case "scriptstyle":
				sty.px, sty.display, sty.spacious, toks = e.scriptSize(sty.px), false, false, toks[1:]
				continue
			}
		}
		if atStop(toks, stop) {
			return e.hlist(items, sty), toks, nil
		}
		nuc, cls, isOp, rest, err := e.parseAtom(toks, sty)
		if err != nil {
			return nil, nil, err
		}
		toks = rest
		nuc.cls = cls

		// scripts and primes
		var sup, sub *box
		nprime := 0
		for len(toks) > 0 && (toks[0].kind == tSup || toks[0].kind == tSub || toks[0].kind == tPrime) {
			switch toks[0].kind {
			case tPrime:
				nprime++
				toks = toks[1:]
			case tSup:
				b, r, err := e.parseGroupArg(toks[1:], sty.script(e))
				if err != nil {
					return nil, nil, err
				}
				sup, toks = b, r
			case tSub:
				b, r, err := e.parseGroupArg(toks[1:], sty.script(e))
				if err != nil {
					return nil, nil, err
				}
				sub, toks = b, r
			}
		}
		if nprime > 0 {
			pb := e.primeBox(nprime, sty.script(e))
			if sup == nil {
				sup = pb
			} else {
				sup = e.hlist([]*box{pb, sup}, sty.script(e))
			}
		}
		if sup != nil || sub != nil {
			if isOp && sty.display {
				nuc = e.attachLimits(nuc, sup, sub, sty)
			} else {
				nuc = e.attachScripts(nuc, sup, sub, sty)
			}
		}
		items = append(items, nuc)
	}
}

// parseGroupArg parses one atom or {group} as an argument (for scripts, \frac,
// accents, …), returning its box.
func (e *engine) parseGroupArg(toks []token, sty style) (*box, []token, error) {
	b, _, _, rest, err := e.parseAtom(toks, sty)
	return b, rest, err
}

// parseAtom parses one nucleus and returns (box, class, isBigOp, remaining).
func (e *engine) parseAtom(toks []token, sty style) (*box, atomClass, bool, []token, error) {
	if len(toks) == 0 {
		return nil, 0, false, nil, fmt.Errorf("texmath: expected an atom")
	}
	t := toks[0]
	switch t.kind {
	case tLBrace:
		b, rest, err := e.parseList(toks[1:], sty, stopBrace)
		if err != nil {
			return nil, 0, false, nil, err
		}
		if len(rest) == 0 || rest[0].kind != tRBrace {
			return nil, 0, false, nil, fmt.Errorf("texmath: missing }")
		}
		return b, clsOrd, false, rest[1:], nil
	case tRBrace, tSup, tSub, tAmp, tPrime:
		return nil, 0, false, nil, fmt.Errorf("texmath: unexpected %q", tokenText(t))
	case tCtrl:
		return e.parseControl(t.text, toks[1:], sty)
	default: // tChar
		r := mapAlpha(t.r, sty)
		b := e.mustGlyph(r, sty.px, clsOrd)
		return b, charClass(t.r), false, toks[1:], nil
	}
}

// parseControl handles a control sequence (name already stripped of backslash).
func (e *engine) parseControl(name string, toks []token, sty style) (*box, atomClass, bool, []token, error) {
	switch name {
	case "frac", "tfrac", "dfrac":
		fsty := sty
		if name == "dfrac" {
			fsty.display = true
		} else if name == "tfrac" {
			fsty.display = false
		}
		num, r1, err := e.parseGroupArg(toks, fsty.inner())
		if err != nil {
			return nil, 0, false, nil, err
		}
		den, r2, err := e.parseGroupArg(r1, fsty.inner())
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.fraction(num, den, fsty), clsInner, false, r2, nil
	case "sqrt":
		var index *box
		rest := toks
		if len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
			ib, r, err := e.parseUntilBracket(rest[1:], sty.script(e))
			if err != nil {
				return nil, 0, false, nil, err
			}
			index, rest = ib, r
		}
		body, r, err := e.parseGroupArg(rest, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.radical(body, index, sty), clsOrd, false, r, nil
	case "left":
		open, rest, err := e.readDelim(toks)
		if err != nil {
			return nil, 0, false, nil, err
		}
		inner, r1, err := e.parseList(rest, sty, stopRight)
		if err != nil {
			return nil, 0, false, nil, err
		}
		if len(r1) == 0 {
			return nil, 0, false, nil, fmt.Errorf(`texmath: \left without \right`)
		}
		close, r2, err := e.readDelim(r1[1:]) // skip \right
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.delimited(inner, open, close, sty), clsInner, false, r2, nil
	case "overline":
		b, r, err := e.parseGroupArg(toks, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.overline(b, sty), clsOrd, false, r, nil
	case "underline":
		b, r, err := e.parseGroupArg(toks, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.underline(b, sty), clsOrd, false, r, nil
	case "text", "mathrm", "mathbf", "mathbb", "mathcal", "mathfrak", "mathsf", "mathtt", "mathit", "boldsymbol":
		asty := sty
		asty.alpha = alphabetFor(name)
		b, r, err := e.parseGroupArg(toks, asty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return b, clsOrd, false, r, nil
	case "begin":
		return e.parseEnv(toks, sty)
	}
	// accents
	if acc, ok := accents[name]; ok {
		b, r, err := e.parseGroupArg(toks, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.accent(b, acc, sty), clsOrd, false, r, nil
	}
	// explicit spacing
	if mu, ok := spaces[name]; ok {
		return e.kern(float64(sty.px) * mu / 18), clsOrd, false, toks, nil
	}
	// symbols
	if s, ok := symbols[name]; ok {
		b := e.mustGlyph(s.r, sty.px, s.cls)
		return b, s.cls, opLimits[name], toks, nil
	}
	return nil, 0, false, nil, fmt.Errorf(`texmath: unknown command \%s`, name)
}

// parseEnv handles \begin{env}…\end{env} matrix-like environments.
func (e *engine) parseEnv(toks []token, sty style) (*box, atomClass, bool, []token, error) {
	if len(toks) < 3 || toks[0].kind != tLBrace {
		return nil, 0, false, nil, fmt.Errorf(`texmath: \begin needs {env}`)
	}
	// read env name up to }
	env := ""
	i := 1
	for i < len(toks) && toks[i].kind != tRBrace {
		env += tokenText(toks[i])
		i++
	}
	if i >= len(toks) {
		return nil, 0, false, nil, fmt.Errorf(`texmath: \begin{ unterminated`)
	}
	toks = toks[i+1:]
	dl, ok := envDelims[env]
	if !ok {
		return nil, 0, false, nil, fmt.Errorf("texmath: unknown environment %q", env)
	}
	open, close := dl[0], dl[1]
	var rows [][]*box
	var row []*box
	csty := sty.inner()
	for {
		cell, rest, err := e.parseList(toks, csty, stopCell)
		if err != nil {
			return nil, 0, false, nil, err
		}
		row = append(row, cell)
		toks = rest
		if len(toks) == 0 {
			return nil, 0, false, nil, fmt.Errorf(`texmath: \begin{%s} without \end`, env)
		}
		switch {
		case toks[0].kind == tAmp:
			toks = toks[1:]
		case toks[0].kind == tCtrl && toks[0].text == `\`:
			rows = append(rows, row)
			row = nil
			toks = toks[1:]
		case toks[0].kind == tCtrl && toks[0].text == "end":
			rows = append(rows, row)
			// consume \end{env}
			r, err := e.consumeEnd(toks[1:], env)
			if err != nil {
				return nil, 0, false, nil, err
			}
			return e.matrix(rows, open, close, sty), clsInner, false, r, nil
		}
	}
}

func (e *engine) consumeEnd(toks []token, env string) ([]token, error) {
	if len(toks) < 2 || toks[0].kind != tLBrace {
		return nil, fmt.Errorf(`texmath: \end needs {env}`)
	}
	name := ""
	i := 1
	for i < len(toks) && toks[i].kind != tRBrace {
		name += tokenText(toks[i])
		i++
	}
	if i >= len(toks) {
		return nil, fmt.Errorf(`texmath: \end{ unterminated`)
	}
	if name != env {
		return nil, fmt.Errorf("texmath: \\begin{%s} closed by \\end{%s}", env, name)
	}
	return toks[i+1:], nil
}

// parseUntilBracket parses tokens up to a literal ']' (for \sqrt's optional
// index), returning the laid-out box.
func (e *engine) parseUntilBracket(toks []token, sty style) (*box, []token, error) {
	var items []*box
	for len(toks) > 0 && !(toks[0].kind == tChar && toks[0].r == ']') {
		b, _, _, rest, err := e.parseAtom(toks, sty)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, b)
		toks = rest
	}
	if len(toks) == 0 {
		return nil, nil, fmt.Errorf("texmath: missing ] after [")
	}
	return e.hlist(items, sty), toks[1:], nil
}

// readDelim reads a delimiter token for \left / \right (a char like ( [ | ., or
// a control symbol like \{ \langle), returning its rune (0 for '.').
func (e *engine) readDelim(toks []token) (rune, []token, error) {
	if len(toks) == 0 {
		return 0, nil, fmt.Errorf(`texmath: \left/\right needs a delimiter`)
	}
	t := toks[0]
	switch t.kind {
	case tChar:
		if t.r == '.' {
			return 0, toks[1:], nil
		}
		return t.r, toks[1:], nil
	case tCtrl:
		if s, ok := symbols[t.text]; ok { // includes \{ \} \langle \lfloor …
			return s.r, toks[1:], nil
		}
	}
	return 0, nil, fmt.Errorf("texmath: bad delimiter %q", tokenText(t))
}

// kern returns a zero-glyph box of the given width.
func (e *engine) kern(w float64) *box { b := newBox(clsOrd); b.w = w; return b }

// primeBox builds n stacked prime glyphs (′ ″ ‴ …).
func (e *engine) primeBox(n int, sty style) *box {
	var items []*box
	for i := 0; i < n; i++ {
		if b, ok := e.glyphBox('′', sty.px, clsOrd); ok {
			items = append(items, b)
		}
	}
	return e.hlist(items, sty)
}

// mustGlyph returns a glyph box, or a zero-width empty box if the font lacks the
// rune.
func (e *engine) mustGlyph(r rune, px int, cls atomClass) *box {
	if b, ok := e.glyphBox(r, px, cls); ok {
		return b
	}
	return newBox(cls)
}

// ── character classes & alphabets ───────────────────────────────────────────

func charClass(r rune) atomClass {
	switch r {
	case '+', '-', '*', '/':
		return clsBin
	case '=', '<', '>':
		return clsRel
	case '(', '[':
		return clsOpen
	case ')', ']':
		return clsClose
	case ',', ';':
		return clsPunct
	default:
		return clsOrd
	}
}

// mapAlpha maps a source rune to its glyph rune given the active alphabet.
func mapAlpha(r rune, sty style) rune {
	if sty.alpha != nil {
		return sty.alpha(r)
	}
	return mathItalic(r)
}

// mathItalic maps ASCII letters to Unicode mathematical-italic code points.
func mathItalic(r rune) rune {
	switch {
	case r == 'h':
		return 0x210E
	case r >= 'a' && r <= 'z':
		return 0x1D44E + (r - 'a')
	case r >= 'A' && r <= 'Z':
		return 0x1D434 + (r - 'A')
	default:
		return r
	}
}

// alphabetFor returns the letter-mapping function for a \math… alphabet command.
func alphabetFor(name string) func(rune) rune {
	switch name {
	case "text", "mathrm":
		return func(r rune) rune { return r } // upright, unchanged
	case "mathbf":
		return blockMapper(0x1D400, 0x1D41A, 0x1D7CE, nil)
	case "mathit":
		return mathItalic
	case "boldsymbol":
		return blockMapper(0x1D468, 0x1D482, 0x1D7CE, nil)
	case "mathsf":
		return blockMapper(0x1D5A0, 0x1D5BA, 0x1D7E2, nil)
	case "mathtt":
		return blockMapper(0x1D670, 0x1D68A, 0x1D7F6, nil)
	case "mathbb":
		return blockMapper(0x1D538, 0x1D552, 0x1D7D8, bbHoles)
	case "mathcal":
		return blockMapper(0x1D49C, 0x1D4B6, 0, calHoles)
	case "mathfrak":
		return blockMapper(0x1D504, 0x1D51E, 0, frakHoles)
	default:
		return nil
	}
}

// blockMapper builds a letter/digit mapper into the given Unicode base points,
// consulting a holes table for reserved code points that live elsewhere.
func blockMapper(ucBase, lcBase, digBase rune, holes map[rune]rune) func(rune) rune {
	return func(r rune) rune {
		if h, ok := holes[r]; ok {
			return h
		}
		switch {
		case r >= 'A' && r <= 'Z':
			return ucBase + (r - 'A')
		case r >= 'a' && r <= 'z':
			return lcBase + (r - 'a')
		case r >= '0' && r <= '9' && digBase != 0:
			return digBase + (r - '0')
		default:
			return r
		}
	}
}

var bbHoles = map[rune]rune{'C': 0x2102, 'H': 0x210D, 'N': 0x2115, 'P': 0x2119, 'Q': 0x211A, 'R': 0x211D, 'Z': 0x2124}
var calHoles = map[rune]rune{'B': 0x212C, 'E': 0x2130, 'F': 0x2131, 'H': 0x210B, 'I': 0x2110, 'L': 0x2112, 'M': 0x2133, 'R': 0x211B, 'e': 0x212F, 'g': 0x210A, 'o': 0x2134}
var frakHoles = map[rune]rune{'C': 0x212D, 'H': 0x210C, 'I': 0x2111, 'R': 0x211C, 'Z': 0x2128}

// ── tables ──────────────────────────────────────────────────────────────────

type sym struct {
	r   rune
	cls atomClass
}

var accents = map[string]rune{
	"hat": 'ˆ', "widehat": 'ˆ', "bar": '¯', "vec": '⃗',
	"tilde": '˜', "widetilde": '˜', "dot": '˙', "ddot": '¨',
	"check": 'ˇ', "breve": '˘', "acute": '´', "grave": '`', "mathring": '˚',
}

// spaces maps a spacing command to its width in mu (em/18).
var spaces = map[string]float64{
	",": 3, ":": 4, ";": 5, "!": -3, "quad": 18, "qquad": 36, " ": 6,
}

// opLimits marks big operators that set sub/superscripts as limits in display.
var opLimits = map[string]bool{
	"sum": true, "prod": true, "coprod": true, "bigcup": true, "bigcap": true,
	"bigvee": true, "bigwedge": true, "bigoplus": true, "bigotimes": true,
	"bigodot": true, "biguplus": true, "bigsqcup": true, "lim": true, "max": true,
	"min": true, "sup": true, "inf": true,
}

var envDelims = map[string][2]rune{
	"matrix": {0, 0}, "pmatrix": {'(', ')'}, "bmatrix": {'[', ']'},
	"Bmatrix": {'{', '}'}, "vmatrix": {'|', '|'}, "Vmatrix": {'‖', '‖'},
	"cases": {'{', 0},
}

// symbols maps a control-sequence name to its glyph and atom class.
var symbols = map[string]sym{
	// greek lower
	"alpha": {'α', clsOrd}, "beta": {'β', clsOrd}, "gamma": {'γ', clsOrd}, "delta": {'δ', clsOrd},
	"epsilon": {'ε', clsOrd}, "varepsilon": {'ϵ', clsOrd}, "zeta": {'ζ', clsOrd}, "eta": {'η', clsOrd},
	"theta": {'θ', clsOrd}, "vartheta": {'ϑ', clsOrd}, "iota": {'ι', clsOrd}, "kappa": {'κ', clsOrd},
	"lambda": {'λ', clsOrd}, "mu": {'μ', clsOrd}, "nu": {'ν', clsOrd}, "xi": {'ξ', clsOrd},
	"pi": {'π', clsOrd}, "varpi": {'ϖ', clsOrd}, "rho": {'ρ', clsOrd}, "varrho": {'ϱ', clsOrd},
	"sigma": {'σ', clsOrd}, "varsigma": {'ς', clsOrd}, "tau": {'τ', clsOrd}, "upsilon": {'υ', clsOrd},
	"phi": {'φ', clsOrd}, "varphi": {'ϕ', clsOrd}, "chi": {'χ', clsOrd}, "psi": {'ψ', clsOrd}, "omega": {'ω', clsOrd},
	// greek upper
	"Gamma": {'Γ', clsOrd}, "Delta": {'Δ', clsOrd}, "Theta": {'Θ', clsOrd}, "Lambda": {'Λ', clsOrd},
	"Xi": {'Ξ', clsOrd}, "Pi": {'Π', clsOrd}, "Sigma": {'Σ', clsOrd}, "Upsilon": {'Υ', clsOrd},
	"Phi": {'Φ', clsOrd}, "Psi": {'Ψ', clsOrd}, "Omega": {'Ω', clsOrd},
	// big operators
	"sum": {'∑', clsOp}, "prod": {'∏', clsOp}, "coprod": {'∐', clsOp}, "int": {'∫', clsOp},
	"oint": {'∮', clsOp}, "iint": {'∬', clsOp}, "iiint": {'∭', clsOp},
	"bigcup": {'⋃', clsOp}, "bigcap": {'⋂', clsOp}, "bigvee": {'⋁', clsOp}, "bigwedge": {'⋀', clsOp},
	"bigoplus": {'⨁', clsOp}, "bigotimes": {'⨂', clsOp}, "bigodot": {'⨀', clsOp}, "bigsqcup": {'⨆', clsOp},
	"biguplus": {'⨄', clsOp}, "lim": {'l', clsOp}, // lim rendered as text elsewhere ideally
	// binary operators
	"times": {'×', clsBin}, "div": {'÷', clsBin}, "pm": {'±', clsBin}, "mp": {'∓', clsBin},
	"cdot": {'⋅', clsBin}, "ast": {'∗', clsBin}, "star": {'⋆', clsBin}, "circ": {'∘', clsBin},
	"bullet": {'∙', clsBin}, "oplus": {'⊕', clsBin}, "ominus": {'⊖', clsBin}, "otimes": {'⊗', clsBin},
	"oslash": {'⊘', clsBin}, "odot": {'⊙', clsBin}, "cup": {'∪', clsBin}, "cap": {'∩', clsBin},
	"uplus": {'⊎', clsBin}, "sqcup": {'⊔', clsBin}, "sqcap": {'⊓', clsBin}, "vee": {'∨', clsBin},
	"wedge": {'∧', clsBin}, "setminus": {'∖', clsBin}, "wr": {'≀', clsBin}, "diamond": {'⋄', clsBin},
	"bigtriangleup": {'△', clsBin}, "bigtriangledown": {'▽', clsBin},
	// relations
	"leq": {'≤', clsRel}, "le": {'≤', clsRel}, "geq": {'≥', clsRel}, "ge": {'≥', clsRel},
	"neq": {'≠', clsRel}, "ne": {'≠', clsRel}, "equiv": {'≡', clsRel}, "approx": {'≈', clsRel},
	"cong": {'≅', clsRel}, "simeq": {'≃', clsRel}, "sim": {'∼', clsRel}, "propto": {'∝', clsRel},
	"ll": {'≪', clsRel}, "gg": {'≫', clsRel}, "doteq": {'≐', clsRel}, "asymp": {'≍', clsRel},
	"in": {'∈', clsRel}, "notin": {'∉', clsRel}, "ni": {'∋', clsRel}, "subset": {'⊂', clsRel},
	"supset": {'⊃', clsRel}, "subseteq": {'⊆', clsRel}, "supseteq": {'⊇', clsRel},
	"sqsubseteq": {'⊑', clsRel}, "sqsupseteq": {'⊒', clsRel}, "prec": {'≺', clsRel}, "succ": {'≻', clsRel},
	"preceq": {'⪯', clsRel}, "succeq": {'⪰', clsRel}, "models": {'⊨', clsRel}, "vdash": {'⊢', clsRel},
	"dashv": {'⊣', clsRel}, "perp": {'⊥', clsRel}, "mid": {'∣', clsRel}, "parallel": {'∥', clsRel},
	"doteqdot": {'≑', clsRel}, "bowtie": {'⋈', clsRel},
	// arrows
	"leftarrow": {'←', clsRel}, "gets": {'←', clsRel}, "rightarrow": {'→', clsRel}, "to": {'→', clsRel},
	"leftrightarrow": {'↔', clsRel}, "Leftarrow": {'⇐', clsRel}, "Rightarrow": {'⇒', clsRel},
	"Leftrightarrow": {'⇔', clsRel}, "iff": {'⇔', clsRel}, "mapsto": {'↦', clsRel},
	"longrightarrow": {'⟶', clsRel}, "longleftarrow": {'⟵', clsRel}, "implies": {'⟹', clsRel},
	"uparrow": {'↑', clsRel}, "downarrow": {'↓', clsRel}, "updownarrow": {'↕', clsRel},
	"nearrow": {'↗', clsRel}, "searrow": {'↘', clsRel}, "swarrow": {'↙', clsRel}, "nwarrow": {'↖', clsRel},
	"hookrightarrow": {'↪', clsRel}, "hookleftarrow": {'↩', clsRel},
	// misc ordinary / symbols
	"infty": {'∞', clsOrd}, "partial": {'∂', clsOrd}, "nabla": {'∇', clsOrd}, "forall": {'∀', clsOrd},
	"exists": {'∃', clsOrd}, "nexists": {'∄', clsOrd}, "emptyset": {'∅', clsOrd}, "varnothing": {'∅', clsOrd},
	"neg": {'¬', clsOrd}, "lnot": {'¬', clsOrd}, "top": {'⊤', clsOrd}, "bot": {'⊥', clsOrd},
	"aleph": {'ℵ', clsOrd}, "hbar": {'ℏ', clsOrd}, "ell": {'ℓ', clsOrd}, "wp": {'℘', clsOrd},
	"Re": {'ℜ', clsOrd}, "Im": {'ℑ', clsOrd}, "angle": {'∠', clsOrd}, "triangle": {'△', clsOrd},
	"backslash": {'\\', clsOrd}, "prime": {'′', clsOrd}, "surd": {'√', clsOrd},
	"clubsuit": {'♣', clsOrd}, "diamondsuit": {'♢', clsOrd}, "heartsuit": {'♡', clsOrd}, "spadesuit": {'♠', clsOrd},
	"flat": {'♭', clsOrd}, "natural": {'♮', clsOrd}, "sharp": {'♯', clsOrd},
	"dagger": {'†', clsOrd}, "ddagger": {'‡', clsOrd}, "S": {'§', clsOrd}, "P": {'¶', clsOrd},
	"cdots": {'⋯', clsOrd}, "ldots": {'…', clsOrd}, "vdots": {'⋮', clsOrd}, "ddots": {'⋱', clsOrd},
	"dots": {'…', clsOrd}, "sqrt": {'√', clsOrd},
	// delimiters (also used by \left \right)
	"langle": {'⟨', clsOpen}, "rangle": {'⟩', clsClose}, "lceil": {'⌈', clsOpen}, "rceil": {'⌉', clsClose},
	"lfloor": {'⌊', clsOpen}, "rfloor": {'⌋', clsClose}, "vert": {'|', clsOrd}, "Vert": {'‖', clsOrd},
	"lbrace": {'{', clsOpen}, "rbrace": {'}', clsClose}, "|": {'‖', clsOrd},
	// control symbols
	"{": {'{', clsOpen}, "}": {'}', clsClose},
}
