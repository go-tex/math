// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"testing"

	"github.com/go-opentype/opentype"
)

// A symbol table entry names a rune, and a rune the embedded font has no glyph for is
// typeset as .notdef — a hollow box. That is WORSE than the dropped equation the entry
// was added to prevent: a drop is reported, a tofu is silent and looks like content.
//
// This walks the whole table rather than the entries of any one change, because the
// risk is not in the entry someone is looking at.
func TestEverySymbolHasAGlyphInTheEmbeddedFont(t *testing.T) {
	f, err := opentype.Parse(DefaultFont())
	if err != nil {
		t.Fatal(err)
	}
	missing := 0
	for name, s := range symbols {
		if _, ok := f.GlyphIndex(s.r); !ok {
			t.Errorf("\\%s: U+%04X has no glyph in the embedded font", name, s.r)
			missing++
		}
	}
	t.Logf("%d symbols checked, %d without a glyph", len(symbols), missing)
}
