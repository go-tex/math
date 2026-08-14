// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"fmt"
	"strings"
)

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
	stopEnd      stopMode = iota // only end of input
	stopBrace                    // stop at }
	stopRight                    // stop at \right
	stopCell                     // stop at & \\ or \end (matrix cell)
	stopStackRow                 // stop at \\ or } (a \substack line)
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
	case stopStackRow:
		return t.kind == tRBrace || (t.kind == tCtrl && t.text == `\`)
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
			case "rm", "bf", "it", "sf", "tt", "cal", "sl":
				// Declarative, group-scoped font switches (\rm, \bf, …): they alter
				// the active alphabet for the REST of the current {…} group, unlike
				// \mathrm{…} which takes a single argument. Because parseList runs
				// afresh for each brace group and receives its style by value, the
				// switch neither leaks past the closing brace nor into sibling
				// groups — exactly TeX's scoping. \sl (slanted) has no dedicated
				// Unicode math alphabet in the MATH font, so it is approximated by
				// math italic.
				sty.alpha, toks = fontSwitch[toks[0].text], toks[1:]
				continue
			}
		}
		if atStop(toks, stop) {
			return e.hlist(items, sty), toks, nil
		}
		var nuc *box
		var cls atomClass
		var isOp bool
		if k := toks[0].kind; k == tSup || k == tSub || k == tPrime {
			// A script or prime with no preceding nucleus — e.g. $^1$ (a footnote or
			// citation mark) or a leading prime. TeX attaches it to an empty nucleus
			// rather than erroring; the script loop below consumes the ^/_/' tokens.
			nuc, cls, isOp = newBox(clsOrd), clsOrd, false
		} else {
			var rest []token
			var err error
			nuc, cls, isOp, rest, err = e.parseAtom(toks, sty)
			if err != nil {
				return nil, nil, err
			}
			toks = rest
		}
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
	case "operatorname":
		// \operatorname{name} sets name upright with operator spacing; the starred
		// form \operatorname*{name} sets sub/superscripts as limits in display.
		star := false
		if len(toks) > 0 && toks[0].kind == tChar && toks[0].r == '*' {
			star, toks = true, toks[1:]
		}
		name, rest, err := readOpName(toks)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.opName(name, clsOp, sty), clsOp, star && sty.display, rest, nil
	case "bmod":
		// Binary "mod" operator (\bmod): "mod" set upright with binary spacing.
		return e.opName("mod", clsBin, sty), clsBin, false, toks, nil
	case "pmod":
		n, rest, err := e.parseGroupArg(toks, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.modBox('(', ')', "mod", n, sty), clsOrd, false, rest, nil
	case "mod":
		n, rest, err := e.parseGroupArg(toks, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.modBox(0, 0, "mod", n, sty), clsOrd, false, rest, nil
	case "pod":
		n, rest, err := e.parseGroupArg(toks, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.modBox('(', ')', "", n, sty), clsOrd, false, rest, nil
	case "binom", "dbinom", "tbinom", "choose":
		fsty := sty
		if name == "dbinom" {
			fsty.display = true
		} else if name == "tbinom" {
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
		return e.delimited(e.binom(num, den, fsty), '(', ')', fsty), clsInner, false, r2, nil
	case "overset", "underset", "stackrel":
		// \overset{top}{base} (and \stackrel) put a small box above the base;
		// \underset{bottom}{base} below it.
		extra, r1, err := e.parseGroupArg(toks, sty.script(e))
		if err != nil {
			return nil, 0, false, nil, err
		}
		base, r2, err := e.parseGroupArg(r1, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		cls := clsOrd
		if name == "stackrel" {
			cls = clsRel
		}
		return e.overUnder(base, extra, name != "underset", sty), cls, false, r2, nil
	case "substack":
		// \substack{a \\ b \\ c}: script-size lines stacked and centred, used as a
		// multi-line sub/superscript under a big operator.
		if len(toks) == 0 || toks[0].kind != tLBrace {
			return nil, 0, false, nil, fmt.Errorf(`texmath: \substack needs {…}`)
		}
		ssty := sty.script(e)
		rest := toks[1:]
		var rows [][]*box
		for {
			cell, r, err := e.parseList(rest, ssty, stopStackRow)
			if err != nil {
				return nil, 0, false, nil, err
			}
			rows = append(rows, []*box{cell})
			rest = r
			if len(rest) == 0 {
				return nil, 0, false, nil, fmt.Errorf(`texmath: \substack without closing }`)
			}
			if rest[0].kind == tRBrace {
				rest = rest[1:]
				break
			}
			rest = rest[1:] // consume \\
		}
		return e.gridLayout(rows, gridOpts{rowGap: float64(ssty.px) * 0.18}, ssty), clsOrd, false, rest, nil
	case "not":
		// \not X overlays a negation slash on the following atom (e.g. \not= ⇒ ≠,
		// \not\subset ⇒ ⊄), keeping that atom's class.
		b, cls, _, rest, err := e.parseAtom(toks, sty)
		if err != nil {
			return nil, 0, false, nil, err
		}
		return e.negate(b, sty), cls, false, rest, nil
	}
	// \big \Big \bigg \Bigg (and the l/r/m variants) size the FOLLOWING delimiter to
	// a fixed height instead of stretching it to surrounding content.
	if f, ok := bigDelimFactor(name); ok {
		d, rest, err := e.readDelim(toks)
		if err != nil {
			return nil, 0, false, nil, err
		}
		cls := bigDelimClass(name)
		b := e.axisCentre(e.stretchVertical(d, f*float64(sty.px), sty.px, cls), sty.px)
		return b, cls, false, rest, nil
	}
	// named operators (\log \sin \lim …): upright name, operator class/spacing.
	// The limit-taking members set sub/superscripts above/below in display.
	if op, ok := namedOps[name]; ok {
		return e.opName(op.text, clsOp, sty), clsOp, op.limits, toks, nil
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
	info, ok := envTable[env]
	if !ok {
		return nil, 0, false, nil, fmt.Errorf("texmath: unknown environment %q", env)
	}
	// array reads a {lcr|…} column specification before its body. A missing or
	// unterminated spec falls back to all-centred columns rather than erroring.
	var aligns []colAlign
	var vrules []int
	if info.kind == kindArray {
		aligns, vrules, toks = readColSpec(toks)
	}
	// smallmatrix typesets its cells at script size; other envs use text style.
	csty := sty.inner()
	if info.kind == kindSmall {
		csty.px = e.scriptSize(csty.px)
	}
	var rows [][]*box
	var row []*box
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
			return e.finishEnv(info, rows, aligns, vrules, csty.px, sty), clsInner, false, r, nil
		}
	}
}

// readColSpec parses an array column specification such as {|c|c|} or {lcr}.
// It returns the per-column alignments, the gap indices (0..ncol) carrying a
// vertical rule, and the remaining tokens. If no brace group follows, all
// columns default to centred (aligns nil) — a sensible fallback. Paragraph
// columns p{…}/m{…}/b{…} are NOT supported: their width is ignored and they are
// treated as flush-left. Unknown column letters fall back to centred.
func readColSpec(toks []token) (aligns []colAlign, vrules []int, rest []token) {
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return nil, nil, toks // no spec: fall back to all-centred columns
	}
	// collect the tokens between the outer braces (nested braces included).
	depth := 1
	i := 1
	var spec []token
	for ; i < len(toks); i++ {
		t := toks[i]
		switch t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			depth--
			if depth == 0 {
				i++ // consume the closing brace
				goto interpret
			}
		}
		spec = append(spec, t)
	}
interpret:
	col := 0
	for j := 0; j < len(spec); j++ {
		t := spec[j]
		if t.kind != tChar {
			continue // ignore stray control sequences / braces
		}
		switch t.r {
		case '|':
			vrules = append(vrules, col)
		case 'l':
			aligns = append(aligns, alignL)
			col++
		case 'c':
			aligns = append(aligns, alignC)
			col++
		case 'r':
			aligns = append(aligns, alignR)
			col++
		case 'p', 'm', 'b':
			// paragraph/vertically-aligned column: width unsupported → treat as l.
			aligns = append(aligns, alignL)
			col++
			if j+1 < len(spec) && spec[j+1].kind == tLBrace { // skip the {width} group
				d := 0
				for j++; j < len(spec); j++ {
					if spec[j].kind == tLBrace {
						d++
					} else if spec[j].kind == tRBrace {
						if d--; d == 0 {
							break
						}
					}
				}
			}
		default:
			if isLetter(t.r) { // unknown column letter → sensible fallback: centred
				aligns = append(aligns, alignC)
				col++
			}
			// other punctuation (spaces, @, *) is ignored.
		}
	}
	return aligns, vrules, toks[i:]
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

// bigDelimFactor reports the fixed delimiter-height multiple (of the em size) for
// the \big family — \big/\bigl/\bigr/\bigm, \Big…, \bigg…, \Bigg… — and false for
// any other command name.
func bigDelimFactor(name string) (float64, bool) {
	base := strings.TrimRight(name, "lrm")
	switch base {
	case "big":
		return 1.2, true
	case "Big":
		return 1.8, true
	case "bigg":
		return 2.4, true
	case "Bigg":
		return 3.0, true
	}
	return 0, false
}

// bigDelimClass maps a \big-family suffix to its atom class: l→open, r→close,
// m→relation, none→ordinary.
func bigDelimClass(name string) atomClass {
	switch name[len(name)-1] {
	case 'l':
		return clsOpen
	case 'r':
		return clsClose
	case 'm':
		return clsRel
	}
	return clsOrd
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

// opName typesets s as an upright operator name (roman glyphs, no math-italic
// mapping and no inter-atom spacing between the letters). A space in s becomes a
// thin (3mu) kern — used by "lim sup"/"lim inf". The returned box carries class
// cls (clsOp for operators, clsBin for \bmod).
func (e *engine) opName(s string, cls atomClass, sty style) *box {
	var items []*box
	for _, r := range s {
		if r == ' ' {
			items = append(items, e.kern(float64(sty.px)*3/18))
			continue
		}
		items = append(items, e.mustGlyph(r, sty.px, clsOrd))
	}
	b := e.hlist(items, style{px: sty.px})
	b.cls = cls
	return b
}

// modBox builds the parenthesised/unparenthesised modular annotations produced
// by \pmod{n} → "(mod n)", \mod{n} → "mod n" and \pod{n} → "(n)". A quad space
// (18mu) always precedes, as in plain TeX. open/close are 0 for the unbracketed
// \mod form, and label is "" for \pod.
func (e *engine) modBox(open, close rune, label string, n *box, sty style) *box {
	items := []*box{e.kern(float64(sty.px))} // leading \quad
	if open != 0 {
		items = append(items, e.mustGlyph(open, sty.px, clsOrd))
	}
	if label != "" {
		items = append(items, e.opName(label, clsOrd, sty), e.kern(float64(sty.px)*3/18))
	}
	if n != nil {
		items = append(items, n)
	}
	if close != 0 {
		items = append(items, e.mustGlyph(close, sty.px, clsOrd))
	}
	b := e.hlist(items, style{px: sty.px})
	b.cls = clsOrd
	return b
}

// readOpName reads the {name} argument of \operatorname, returning the plain
// text between the (possibly nested) braces. tChar tokens contribute their rune;
// thin/med spacing control symbols (\, \: \; \ ) become a space; other control
// sequences are ignored. It errors on a missing or unterminated brace group.
func readOpName(toks []token) (string, []token, error) {
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return "", nil, fmt.Errorf(`texmath: \operatorname needs {name}`)
	}
	var sb strings.Builder
	depth := 1
	for i := 1; i < len(toks); i++ {
		switch t := toks[i]; t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			if depth--; depth == 0 {
				return sb.String(), toks[i+1:], nil
			}
		case tChar:
			sb.WriteString(t.text)
		case tCtrl:
			switch t.text {
			case ",", ":", ";", " ":
				sb.WriteByte(' ')
			}
		}
	}
	return "", nil, fmt.Errorf(`texmath: \operatorname unterminated {name}`)
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

// opLimits marks the big-operator SYMBOLS (\sum, \prod, …) that set
// sub/superscripts as limits in display. Named operators (\lim, \max, …) carry
// their own limit flag via namedOps.
var opLimits = map[string]bool{
	"sum": true, "prod": true, "coprod": true, "bigcup": true, "bigcap": true,
	"bigvee": true, "bigwedge": true, "bigoplus": true, "bigotimes": true,
	"bigodot": true, "biguplus": true, "bigsqcup": true,
}

// fontSwitch maps a declarative two-letter font command to the alphabet it makes
// active for the rest of the current group (see parseList).
var fontSwitch = map[string]func(rune) rune{
	"rm":  alphabetFor("mathrm"),
	"bf":  alphabetFor("mathbf"),
	"it":  alphabetFor("mathit"),
	"sf":  alphabetFor("mathsf"),
	"tt":  alphabetFor("mathtt"),
	"cal": alphabetFor("mathcal"),
	"sl":  alphabetFor("mathit"), // no slanted math alphabet → approximate with italic
}

// opInfo describes a named operator: the upright text to typeset and whether it
// takes limits (sub/superscripts stacked above/below in display style).
type opInfo struct {
	text   string
	limits bool
}

// namedOps are the log-like operators (\log, \sin, …). limits members follow
// plain TeX: \lim, \liminf, \limsup, \max, \min, \sup, \inf, \det, \gcd and \Pr
// set limits in display; the rest use ordinary scripts.
var namedOps = map[string]opInfo{
	"log": {"log", false}, "ln": {"ln", false}, "lg": {"lg", false}, "exp": {"exp", false},
	"sin": {"sin", false}, "cos": {"cos", false}, "tan": {"tan", false}, "cot": {"cot", false},
	"sec": {"sec", false}, "csc": {"csc", false},
	"sinh": {"sinh", false}, "cosh": {"cosh", false}, "tanh": {"tanh", false}, "coth": {"coth", false},
	"arcsin": {"arcsin", false}, "arccos": {"arccos", false}, "arctan": {"arctan", false},
	"min": {"min", true}, "max": {"max", true}, "inf": {"inf", true}, "sup": {"sup", true},
	"lim": {"lim", true}, "limsup": {"lim sup", true}, "liminf": {"lim inf", true},
	"det": {"det", true}, "gcd": {"gcd", true}, "Pr": {"Pr", true},
	"dim": {"dim", false}, "ker": {"ker", false}, "deg": {"deg", false},
	"hom": {"hom", false}, "arg": {"arg", false},
}

// envKind selects how a math environment is laid out.
type envKind uint8

const (
	kindMatrix   envKind = iota // centred columns, optional stretchy delimiters
	kindArray                   // per-column l/c/r alignment with | vertical rules
	kindAligned                 // alternating right/left columns (aligned/split)
	kindGathered                // each row a single centred block
	kindSmall                   // matrix at script size, no delimiters
)

// envInfo describes a supported \begin…\end environment.
type envInfo struct {
	kind        envKind
	open, close rune // outer delimiters (kindMatrix only)
}

var envTable = map[string]envInfo{
	"matrix":      {kindMatrix, 0, 0},
	"pmatrix":     {kindMatrix, '(', ')'},
	"bmatrix":     {kindMatrix, '[', ']'},
	"Bmatrix":     {kindMatrix, '{', '}'},
	"vmatrix":     {kindMatrix, '|', '|'},
	"Vmatrix":     {kindMatrix, '‖', '‖'},
	"cases":       {kindMatrix, '{', 0},
	"array":       {kindArray, 0, 0},
	"aligned":     {kindAligned, 0, 0},
	"split":       {kindAligned, 0, 0},
	"gathered":    {kindGathered, 0, 0},
	"smallmatrix": {kindSmall, 0, 0},
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
	"biguplus": {'⨄', clsOp}, // named operators (\lim, \max, …) live in namedOps, not here.
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
	"Longrightarrow": {'⟹', clsRel}, "Longleftarrow": {'⟸', clsRel}, "Longleftrightarrow": {'⟺', clsRel},
	"longmapsto": {'⟼', clsRel},
	"uparrow":    {'↑', clsRel}, "downarrow": {'↓', clsRel}, "updownarrow": {'↕', clsRel},
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
	// amsmath paired delimiters (\lvert…\rvert = |x|, \lVert…\rVert = ‖x‖): the
	// bodies \abs and \norm expand to, and used directly.
	"lvert": {'|', clsOpen}, "rvert": {'|', clsClose},
	"lVert": {'‖', clsOpen}, "rVert": {'‖', clsClose},
	"lgroup": {'⟮', clsOpen}, "rgroup": {'⟯', clsClose},
	// amsmath colon-relations and negated / slanted relations
	"colon": {':', clsPunct}, "coloneqq": {'≔', clsRel}, "coloneq": {'≔', clsRel},
	"eqqcolon": {'≕', clsRel}, "eqcolon": {'≕', clsRel},
	"nmid": {'∤', clsRel}, "nparallel": {'∦', clsRel},
	"subsetneq": {'⊊', clsRel}, "supsetneq": {'⊋', clsRel},
	"subsetneqq": {'⫋', clsRel}, "supsetneqq": {'⫌', clsRel},
	"leqslant": {'⩽', clsRel}, "geqslant": {'⩾', clsRel},
	"lesssim": {'≲', clsRel}, "gtrsim": {'≳', clsRel},
	"lessgtr": {'≶', clsRel}, "gtrless": {'≷', clsRel},
	"trianglelefteq": {'⊴', clsRel}, "trianglerighteq": {'⊵', clsRel},
	"vartriangle": {'△', clsRel}, "triangleq": {'≜', clsRel},
	// amssymb ordinaries seen in the corpus
	"Diamond": {'◇', clsOrd}, "Box": {'□', clsOrd}, "square": {'□', clsOrd},
	"blacksquare": {'■', clsOrd}, "lozenge": {'◊', clsOrd}, "complement": {'∁', clsOrd},
	"circledast": {'⊛', clsBin}, "circledcirc": {'⊚', clsBin},
	// control symbols
	"{": {'{', clsOpen}, "}": {'}', clsClose},
}
