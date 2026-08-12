// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"strings"
	"testing"
)

// pathCount returns the number of glyph <path> elements in an SVG (rules are
// <rect>, so this counts inked glyphs).
func pathCount(t *testing.T, r *Renderer, tex string) int {
	t.Helper()
	return strings.Count(mustRender(t, r, tex), "<path")
}

// eqRender reports whether two TeX strings render to byte-identical SVG.
func eqRender(t *testing.T, r *Renderer, a, b string) bool {
	t.Helper()
	return mustRender(t, r, a) == mustRender(t, r, b)
}

// TestFontSwitches covers the declarative two-letter math font switches
// (\rm \bf \it \sf \tt \cal \sl): each must equal its \math…{…} counterpart on
// the same content, differ from the default (math-italic) rendering, and be
// scoped to the current {…} group (resetting at the closing brace).
func TestFontSwitches(t *testing.T) {
	r := newRenderer(t)
	// group-scoped switch on the group remainder == the single-arg \math… form.
	same := [][2]string{
		{`{\rm th}`, `\mathrm{th}`},
		{`{\bf v}\cdot w`, `\mathbf{v}\cdot w`},
		{`{\it a}`, `\mathit{a}`},
		{`{\sf y}`, `\mathsf{y}`},
		{`{\tt z}`, `\mathtt{z}`},
		{`{\cal A}`, `\mathcal{A}`},
		{`{\sl a}`, `\mathit{a}`}, // \sl approximated by math italic (no slanted alphabet)
	}
	for _, c := range same {
		if !eqRender(t, r, c[0], c[1]) {
			t.Errorf("%q should render like %q", c[0], c[1])
		}
	}
	// the switch changes the glyphs: upright/bold differ from math-italic/roman.
	if eqRender(t, r, `{\rm ab}`, `ab`) {
		t.Error(`\rm should be upright, not math-italic`)
	}
	if eqRender(t, r, `{\bf v}`, `{\rm v}`) {
		t.Error(`\bf should be bold, not roman`)
	}
	// group scoping: the switch affects only the current group's remainder.
	if !eqRender(t, r, `{\rm a}b`, `\mathrm{a}b`) {
		t.Error(`{\rm a}b must keep b outside the switch`)
	}
	if eqRender(t, r, `{\rm a}b`, `{\rm ab}`) {
		t.Error(`\rm must not leak past the closing brace`)
	}
	// works inside a superscript group (x^{\rm th}): upright, unlike x^{th}.
	if eqRender(t, r, `x^{\rm th}`, `x^{th}`) {
		t.Error(`x^{\rm th} must set the superscript upright`)
	}
	// every switch renders in both styles without error.
	for _, sw := range []string{`\rm`, `\bf`, `\it`, `\sf`, `\tt`, `\cal`, `\sl`} {
		renderOK(t, r, `{`+sw+` xyz}`)
	}
}

// TestNamedOperators checks that each log-like operator now renders (previously
// most were "unknown command"), is set upright with one glyph per letter, and
// that the limit-taking members stack scripts as limits in display style.
func TestNamedOperators(t *testing.T) {
	r := newRenderer(t)
	ops := map[string]int{
		`\log`: 3, `\ln`: 2, `\lg`: 2, `\exp`: 3,
		`\sin`: 3, `\cos`: 3, `\tan`: 3, `\cot`: 3, `\sec`: 3, `\csc`: 3,
		`\sinh`: 4, `\cosh`: 4, `\tanh`: 4, `\coth`: 4,
		`\arcsin`: 6, `\arccos`: 6, `\arctan`: 6,
		`\min`: 3, `\max`: 3, `\inf`: 3, `\sup`: 3, `\lim`: 3,
		`\limsup`: 6, `\liminf`: 6, // "lim sup" / "lim inf": 6 letters (space is a kern)
		`\det`: 3, `\dim`: 3, `\ker`: 3, `\deg`: 3, `\gcd`: 3, `\hom`: 3,
		`\arg`: 3, `\Pr`: 2,
	}
	for op, want := range ops {
		if got := pathCount(t, r, op); got != want {
			t.Errorf("%s upright glyphs = %d, want %d", op, got, want)
		}
	}
	// \lim was previously a lone 'l' glyph; it is now the three-letter word.
	if got := pathCount(t, r, `\lim`); got != 3 {
		t.Errorf(`\lim glyphs = %d, want 3`, got)
	}
	// upright, not math-italic: \sin differs from the italic letters s i n.
	if eqRender(t, r, `\sin`, `sin`) {
		t.Error(`\sin must be upright, not italic`)
	}
	// limit flag: a limit op stacks its subscript below in display (≠ text),
	// while a non-limit op keeps a right-set subscript in both.
	dLim, _ := r.RenderDisplaySVG(`\lim_{x} a`, 32)
	tLim, _ := r.RenderSVG(`\lim_{x} a`, 32)
	if dLim == tLim {
		t.Error(`\lim must set limits in display style`)
	}
	dLog, _ := r.RenderDisplaySVG(`\log_{x} a`, 32)
	tLog, _ := r.RenderSVG(`\log_{x} a`, 32)
	if dLog != tLog {
		t.Error(`\log must never take limits`)
	}
}

// TestOperatorname covers \operatorname{…} and the starred \operatorname*{…}.
func TestOperatorname(t *testing.T) {
	r := newRenderer(t)
	if got := pathCount(t, r, `\operatorname{argmax}`); got != 6 {
		t.Errorf(`\operatorname{argmax} glyphs = %d, want 6`, got)
	}
	// \operatorname{sin} is exactly \sin (both upright with operator spacing).
	if !eqRender(t, r, `\operatorname{sin}`, `\sin`) {
		t.Error(`\operatorname{sin} should equal \sin`)
	}
	// upright, not italic.
	if eqRender(t, r, `\operatorname{argmax}`, `argmax`) {
		t.Error(`\operatorname must be upright`)
	}
	// nested braces and an internal thin space inside the name are accepted.
	renderOK(t, r, `\operatorname{a{b}c}`)
	renderOK(t, r, `\operatorname{a\,b}`)
	renderOK(t, r, `\operatorname{a\nope b}`) // unknown control seq inside is ignored
	// starred form: sub/superscripts become limits in display (≠ the plain form).
	star, _ := r.RenderDisplaySVG(`\operatorname*{argmax}_i f`, 32)
	plain, _ := r.RenderDisplaySVG(`\operatorname{argmax}_i f`, 32)
	if star == plain {
		t.Error(`\operatorname* must set limits in display`)
	}
	// error branches.
	for _, tex := range []string{
		`\operatorname`,         // no {name}
		`\operatorname*`,        // starred, no {name}
		`\operatorname x`,       // {name} required, not a bare atom
		`\operatorname{argmax`,  // unterminated {name}
		`\operatorname*{argmax`, // starred, unterminated
	} {
		if _, err := r.RenderSVG(tex, 32); err == nil {
			t.Errorf("render(%q) should error", tex)
		}
	}
}

// TestModular covers \bmod, \pmod, \mod and \pod.
func TestModular(t *testing.T) {
	r := newRenderer(t)
	// \bmod is the three-letter word "mod" as a binary operator.
	if got := pathCount(t, r, `\bmod`); got != 3 {
		t.Errorf(`\bmod glyphs = %d, want 3`, got)
	}
	renderOK(t, r, `a \bmod b`)
	// \pmod{n} → "(mod n)": ( + m o d + n + ) = 6 glyphs (leading quad is a kern).
	if got := pathCount(t, r, `\pmod n`); got != 6 {
		t.Errorf(`\pmod n glyphs = %d, want 6`, got)
	}
	// \mod{n} → "mod n" (no parens): m o d + n = 4 glyphs.
	if got := pathCount(t, r, `\mod n`); got != 4 {
		t.Errorf(`\mod n glyphs = %d, want 4`, got)
	}
	// \pod{n} → "(n)": ( + n + ) = 3 glyphs.
	if got := pathCount(t, r, `\pod n`); got != 3 {
		t.Errorf(`\pod n glyphs = %d, want 3`, got)
	}
	// parenthesised form differs from the bare \mod form.
	if eqRender(t, r, `\pmod n`, `\mod n`) {
		t.Error(`\pmod must add parentheses`)
	}
	renderOK(t, r, `x \pmod n`)
	// error branches: each modular macro needs its {n} argument.
	for _, tex := range []string{`\pmod`, `\mod`, `\pod`} {
		if _, err := r.RenderSVG(tex, 32); err == nil {
			t.Errorf("render(%q) should error", tex)
		}
	}
}

// TestLongArrows covers the long relation arrows added for the study.
func TestLongArrows(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\Longrightarrow`, `\Longleftarrow`, `\Longleftrightarrow`, `\longmapsto`,
	} {
		if got := pathCount(t, r, tex); got != 1 {
			t.Errorf("%s glyphs = %d, want 1", tex, got)
		}
	}
	renderOK(t, r, `A \Longrightarrow B`)
	// the long double arrow is a different glyph from the single \longrightarrow.
	if eqRender(t, r, `\Longrightarrow`, `\longrightarrow`) {
		t.Error(`\Longrightarrow must differ from \longrightarrow`)
	}
}

// TestOpNameHelpers exercises the helpers directly for full branch coverage.
func TestOpNameHelpers(t *testing.T) {
	// fontSwitch table is populated for every two-letter switch.
	for _, sw := range []string{"rm", "bf", "it", "sf", "tt", "cal", "sl"} {
		if fontSwitch[sw] == nil {
			t.Errorf("fontSwitch[%q] is nil", sw)
		}
	}
	// readOpName error branches.
	if _, _, err := readOpName(nil); err == nil {
		t.Error("readOpName(nil) should error")
	}
	if _, _, err := readOpName([]token{{kind: tChar, text: "x", r: 'x'}}); err == nil {
		t.Error("readOpName without a brace should error")
	}
	if _, _, err := readOpName([]token{{kind: tLBrace}, {kind: tChar, text: "a", r: 'a'}}); err == nil {
		t.Error("readOpName unterminated should error")
	}
	// readOpName success with a control-space token → a space in the name.
	name, rest, err := readOpName([]token{
		{kind: tLBrace},
		{kind: tChar, text: "a", r: 'a'},
		{kind: tCtrl, text: ","},
		{kind: tCtrl, text: "nope"}, // ignored
		{kind: tChar, text: "b", r: 'b'},
		{kind: tRBrace},
		{kind: tChar, text: "z", r: 'z'},
	})
	if err != nil || name != "a b" || len(rest) != 1 {
		t.Errorf("readOpName = (%q, %d, %v), want (%q, 1, nil)", name, len(rest), err, "a b")
	}
}
