// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// \char typesets the character at a code point. It reaches the math layer mostly
// through macro expansion — \char`\^ for a literal caret is the commonest arXiv
// form — and the unknown command dropped the whole formula in 11 of the 200
// reference papers. All the TeX number forms that reach math are accepted.
func TestCharPrimitive(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		"\\char`\\^",         // `\<sym>: code point of ^ (the commonest corpus form)
		"a + \\char`\\^ + b", // the same, inside a larger formula
		"x = \\char`A",       // `<char>: code point of A
		`y = \char"41`,       // "<hex>: A
		`z = \char98`,        // decimal: b
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
}
