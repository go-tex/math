// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// A \def inside a formula — a paper's own inline shorthand reaching the math
// layer — used to be an unknown command that dropped the whole equation. It now
// expands like TeX's mouth: the definition is recorded and its uses substituted
// before the parser runs, so the equation typesets.
func TestDefMacros(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\def\R{\mathbb{R}}\R`,                        // parameterless
		`\def\R{\mathbb{R}}x \in \R^n`,                // used in context
		`\def\abs#1{\lvert #1 \rvert}\abs{x}`,         // one parameter, group arg
		`\def\abs#1{\lvert #1 \rvert}\abs x`,          // one parameter, single-token arg
		`\def\pair#1#2{(#1, #2)}\pair{a}{b}`,          // two parameters
		`\def\a{\b}\def\b{x+y}\a`,                     // chained: \a → \b → x+y
		`\def\sq#1{#1^2}\frac{\sq{x}}{\sq{y}}`,        // expansion inside \frac arguments
		`\def\R{\mathbb{R}}\def\C{\mathbb{C}}\R\to\C`, // two macros
		`\def\g#1{#1_{ij}}\g{A} + \g{B}`,              // repeated use
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
}

// A \def references a parameter it did not declare (#2 with one parameter): the
// undeclared reference is dropped, the rest expands — never a panic or a drop of
// the whole equation.
func TestDefBodyReferencesUndeclaredParam(t *testing.T) {
	r := newRenderer(t)
	renderOK(t, r, `\def\x#1{#1#2}\x{a}`)
}

// A \def or a use the mouth cannot resolve is left for the parser (which reports
// it), rather than being guessed at or silently swallowed. And a self-recursive
// macro terminates instead of hanging.
func TestDefUnresolvedIsLeftToParser(t *testing.T) {
	r := newRenderer(t)
	for name, tex := range map[string]string{
		"name not a control seq": `\def{x}{y} z`,        // \def{…} — no macro name
		"delimited parameter":    `\def\x a#1{#1} \x b`, // 'a' delimiter in the parameter text
		"missing body":           `\def\x`,              // nothing after the name
		"unbalanced body":        `\def\x{a+b`,          // no closing brace
		"too few arguments":      `\def\p#1#2{#1#2}\p{a}`,
		"unbalanced argument":    `\def\x#1{#1}\x{a+b`, // the { argument never closes
		"self-recursion halts":   `\def\x{\x}\x`,       // must terminate (budget), then be unknown
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.RenderSVG(tex, 32); err == nil {
				t.Errorf("%q (%s): want an error, got none", tex, name)
			}
		})
	}
}

// The pass is transparent to a stream with no \def: every token passes through
// unchanged, so ordinary formulas are unaffected.
func TestExpandMacrosNoOpWithoutDef(t *testing.T) {
	in := tokenize(`\frac{a}{b} + x^2`)
	out := expandMacros(in)
	if len(out) != len(in) {
		t.Fatalf("expandMacros changed a def-free stream: %d tokens in, %d out", len(in), len(out))
	}
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("token %d changed: %+v → %+v", i, in[i], out[i])
		}
	}
}
