package math

import (
	"os"
	"testing"
)

// \scriptstyle sets its letters in the ssty alternate, as a script does: LM's math
// italic k is 521 units, its alternate 607, so 7pt is 4.249pt and not 3.647pt.
func TestScriptstyleUsesTheSstyAlternate(t *testing.T) {
	data, err := os.ReadFile("testdata/latinmodern-math.otf")
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	_, m, err := r.RenderSVGMetrics(`\scriptstyle k`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Width - 4.249; d > 0.001 || d < -0.001 {
		t.Errorf(`\scriptstyle k is %.3fpt wide, want 4.249 (the alternate; 3.647 is the scaled text glyph)`, m.Width)
	}
}
