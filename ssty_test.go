// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"os"
	"testing"

	"github.com/go-opentype/opentype"
)

// A script glyph is the font's 'ssty' alternate (OpenType registered features,
// features_pt): the first for scripts, the second for scripts of scripts. The box
// must be the width of THAT glyph, not of the text glyph scaled down.
func TestScriptGlyphIsTheSstyAlternate(t *testing.T) {
	// Latin Modern Math (GUST font licence, testdata/): k is 528 units, its first
	// ssty alternate 600, which Computer Modern's cmmi7 (631) is closer to.
	data, err := os.ReadFile("testdata/latinmodern-math.otf")
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	const px = 7
	fc := e.face(px)
	text, _ := e.font.GlyphIndex('k')
	alt1 := fc.ShapeAlternate("k", 0, "ssty")[0]
	alt2 := fc.ShapeAlternate("k", 1, "ssty")[0]
	if alt1 == text || fc.AdvanceIndexUnits(alt1) != 600 {
		t.Fatalf("k: glyph %d, ssty %d (advance %v), want an alternate of 600 units", text, alt1, fc.AdvanceIndexUnits(alt1))
	}
	for _, c := range []struct {
		level int
		gid   opentype.GlyphIndex
	}{{0, text}, {1, alt1}, {2, alt2}} {
		b := e.mustGlyphLevel('k', px, c.level, clsOrd)
		want := fc.AdvanceIndexUnits(c.gid) * e.scale(px)
		if gomath.Abs(b.w-want) > 1e-9 {
			t.Errorf("level %d: width %v, want %v (glyph %d)", c.level, b.w, want, c.gid)
		}
	}
	// A rune the feature does not cover keeps its glyph at every level.
	plain := e.mustGlyphLevel('=', px, 0, clsOrd).w
	if got := e.mustGlyphLevel('=', px, 1, clsOrd).w; got != plain {
		t.Errorf("= at level 1 is %v, at level 0 %v", got, plain)
	}
}
