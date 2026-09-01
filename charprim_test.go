// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// \char typesets the character at a code point. It reaches the math layer mostly
// through macro expansion — \char`\^ for a literal caret is the commonest arXiv
// form — and the unknown command dropped the whole formula in 11 of the 200
// reference papers. Every TeX number form that reaches math is accepted.
func TestCharPrimitive(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		"\\char`\\^",         // `\<sym>: code point of ^ (the commonest corpus form)
		"a + \\char`\\^ + b", // the same, inside a larger formula
		"x = \\char`A",       // `<char>: code point of A
		`y = \char"ff`,       // "<hex> with letters: ÿ
		`z = \char98 + 1`,    // decimal, followed by a non-digit
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
}

// \char with no valid character-code number is a normal error (the formula is
// reported, not silently mis-rendered) — every rejecting branch of parseCharCode.
func TestCharPrimitiveErrors(t *testing.T) {
	r := newRenderer(t)
	for name, tex := range map[string]string{
		"no number":        `\char`,        // nothing follows
		"non-digit":        `\char x`,      // no `/"/digit
		"backtick alone":   "\\char`",      // ` then end of input
		"backtick nonchar": "\\char`^",     // ` then a superscript, not a character
		"backtick bare cs": "\\char`\\",    // ` then a control seq with empty name
		"out of range":     `\char"200000`, // a valid number above U+10FFFF
		"overflows int32":  `\char"FFFFFFFF`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.RenderSVG(tex, 32); err == nil {
				t.Errorf("%q (%s): want an error, got none", tex, name)
			}
		})
	}
}
