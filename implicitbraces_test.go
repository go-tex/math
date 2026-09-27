// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// latex.ltx:115 is one line: \let\bgroup={ \let\egroup=}. They are not macros that
// expand to braces — they ARE the brace tokens, so the scanner emits the brace and
// every construct that reads a group works unchanged. These tests assert that
// EQUIVALENCE rather than a rendered shape: a shape would pass just as well for a
// \bgroup that got silently dropped.
func TestImplicitBracesAreTheBraceTokens(t *testing.T) {
	for _, c := range []struct{ braces, implicit string }{
		{`x^{ab}`, `x^\bgroup ab\egroup`},
		{`\frac{a}{b}`, `\frac\bgroup a\egroup\bgroup b\egroup`},
		{`\sqrt{1+x}`, `\sqrt\bgroup 1+x\egroup`},
		{`{\bf ab}c`, `\bgroup\bf ab\egroup c`},
		{`\mathrm{d}x`, `\mathrm\bgroup d\egroup x`},
	} {
		if got, want := tokenKinds(c.implicit), tokenKinds(c.braces); got != want {
			t.Errorf("tokens(%q) = %v, tokens(%q) = %v", c.implicit, got, c.braces, want)
		}
	}
}

// tokenKinds renders a token stream as a comparable string of kinds and texts.
func tokenKinds(s string) string {
	out := ""
	for _, tk := range tokenize(s) {
		out += string(rune('A'+int(tk.kind))) + tk.text + tk.string() + ";"
	}
	return out
}

func (t token) string() string {
	if t.r != 0 {
		return string(t.r)
	}
	return ""
}

// The group must actually SCOPE, not merely open and close: a \bf inside implicit
// braces stops at \egroup exactly as it stops at }. Two renders that differ prove the
// scoping is real; equal renders would prove nothing about where the group ended.
func TestAnImplicitGroupScopesLikeABraceGroup(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderSVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	if a, b := svg(`\bgroup\bf ab\egroup c`), svg(`{\bf ab}c`); a != b {
		t.Error("an implicit group does not set the same as a brace group")
	}
	// And the scope ENDS where \egroup stands: the c AFTER it must not be bold. If
	// \egroup were merely dropped, both of these would set c bold and be equal.
	if a, b := svg(`\bgroup\bf ab\egroup c`), svg(`\bgroup\bf ab c\egroup`); a == b {
		t.Error(`\egroup did not end the group: c set the same inside and outside it`)
	}
	// An UNCLOSED \bgroup is the same error an unclosed brace gives — the scanner is
	// not treating it as a no-op it can ignore.
	if _, err := r.RenderSVG(`\bgroup\bf abc`, 32); err == nil {
		t.Error(`an unclosed \bgroup did not fail`)
	} else if _, err2 := r.RenderSVG(`{\bf abc`, 32); err2 == nil || err.Error() != err2.Error() {
		t.Errorf(`\bgroup gives %v, { gives %v — they must fail alike`, err, err2)
	}
}

// Renders, both styles, where a real brace would be consumed by an argument scan —
// which is why papers write them at all.
func TestImplicitBracesRenderWhereAPaperWritesThem(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\bgroup a+b\egroup`,
		`x_\bgroup i,j\egroup`,
		`\left(\bgroup a\egroup\right)`,
		`\bgroup\bgroup a\egroup\egroup`,
	} {
		renderOK(t, r, tex)
	}
}
