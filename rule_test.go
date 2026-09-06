package math

import (
	"strings"
	"testing"
)

// \rule is a LaTeX command and is valid in math mode: \rule{0pt}{2.6ex} struts an
// array row, \rule{w}{h} draws a bar. It used to be unknown here, so the caller
// dropped the whole formula — go-tex/engine aborted the document and wrote a
// zero-byte PDF (go-tex/engine#294).
func TestRuleInMath(t *testing.T) {
	r := newRenderer(t)
	near := func(a, b float64) bool { return a-b < 0.02 && b-a < 0.02 }
	for _, c := range []struct {
		src                 string
		wantRect            bool
		wantW, wantH, wantD float64
	}{
		{`\rule{40pt}{5pt}`, true, 40, 5, 0},
		{`\rule{0pt}{10pt}`, false, 0, 10, 0},      // an invisible strut: no ink, full height
		{`\rule{10pt}{0pt}`, false, 10, 0, 0},      // zero height: no ink
		{`\rule[2pt]{10pt}{4pt}`, true, 10, 6, 0},  // lifted: the box is lift+height high
		{`\rule[-3pt]{10pt}{4pt}`, true, 10, 1, 3}, // lowered: the rest becomes depth
		{`\rule{1pc}{1in}`, true, 12, 72.27, 0},    // tex.web:9020-9022, exactly
	} {
		svg, m, err := r.RenderSVGMetrics(c.src, 10)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if got := strings.Contains(svg, "<rect"); got != c.wantRect {
			t.Errorf("%s: rect drawn = %v, want %v", c.src, got, c.wantRect)
		}
		if !near(m.Width, c.wantW) || !near(m.Height, c.wantH) || !near(m.Depth, c.wantD) {
			t.Errorf("%s: w=%.2f h=%.2f d=%.2f, want %.2f/%.2f/%.2f",
				c.src, m.Width, m.Height, m.Depth, c.wantW, c.wantH, c.wantD)
		}
	}
}

// An unreadable length is zero rather than an error: \rule is used for spacing, and
// a formula that loses a strut beats a document that loses the formula.
func TestRuleWithAnUnreadableLength(t *testing.T) {
	r := newRenderer(t)
	if _, _, err := r.RenderSVGMetrics(`\rule{bogus}{5pt}`, 10); err != nil {
		t.Errorf("an unreadable width should not fail the formula: %v", err)
	}
}

// Every length unit tex.web:9019-9034 lists, plus the font-relative ones. The
// height is what is checked because it is the box dimension a rule sets exactly.
func TestRuleLengthUnits(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct {
		src  string
		want float64
	}{
		{`\rule{1pt}{1pt}`, 1},
		{`\rule{1pt}{1in}`, 7227.0 / 100},
		{`\rule{1pt}{1pc}`, 12},
		{`\rule{1pt}{1cm}`, 7227.0 / 254},
		{`\rule{1pt}{1mm}`, 7227.0 / 2540},
		{`\rule{1pt}{1bp}`, 7227.0 / 7200},
		{`\rule{1pt}{1dd}`, 1238.0 / 1157},
		{`\rule{1pt}{1cc}`, 14856.0 / 1157},
		{`\rule{1pt}{65536sp}`, 1},
		{`\rule{1pt}{1em}`, 10}, // the size the probe renders at
		{`\rule{1pt}{5}`, 5},    // a bare number is points
		{`\rule{1pt}{1zz}`, 0},  // an unknown unit reads zero, it does not fail
	} {
		_, m, err := r.RenderSVGMetrics(c.src, 10)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		if d := m.Height - c.want; d > 0.02 || d < -0.02 {
			t.Errorf("%s: height %.4f, want %.4f", c.src, m.Height, c.want)
		}
	}
}

// 1ex is MEASURED — the height of "x" in the font at that size — so it is neither
// zero nor a round fraction of the size, and it scales with the size.
func TestRuleExIsMeasured(t *testing.T) {
	r := newRenderer(t)
	h := func(src string, px int) float64 {
		_, m, err := r.RenderSVGMetrics(src, px)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		return m.Height
	}
	one := h(`\rule{1pt}{1ex}`, 10)
	if one <= 0 || one >= 10 {
		t.Errorf("1ex at 10px = %.2f, want a positive x-height below the em", one)
	}
	if two := h(`\rule{1pt}{2ex}`, 10); two-2*one > 0.02 || two-2*one < -0.02 {
		t.Errorf("2ex = %.2f, want twice 1ex (%.2f)", two, 2*one)
	}
	if big := h(`\rule{1pt}{1ex}`, 20); big <= one {
		t.Errorf("1ex at 20px = %.2f, should exceed %.2f at 10px", big, one)
	}
}

// The forms that cannot be read are errors on the FORMULA, not silent nonsense:
// a missing brace or an unterminated [lift] means the source is not \rule.
func TestRuleMalformed(t *testing.T) {
	r := newRenderer(t)
	for _, src := range []string{
		`\rule[2pt{10pt}{4pt}`, // unterminated [lift]
		`\rule`,                // no arguments at all
		`\rule{10pt}`,          // only one
	} {
		if _, _, err := r.RenderSVGMetrics(src, 10); err == nil {
			t.Errorf("%s: want an error", src)
		}
	}
}

// A [lift] holding a control sequence is carried through to the length reader
// rather than swallowed, so it reads zero instead of eating the rest of the line.
func TestRuleLiftWithControlSequence(t *testing.T) {
	r := newRenderer(t)
	_, m, err := r.RenderSVGMetrics(`\rule[\baselineskip]{10pt}{4pt}`, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d := m.Height - 4; d > 0.02 || d < -0.02 {
		t.Errorf("height %.2f, want 4 (an unreadable lift is zero)", m.Height)
	}
}

// A lift that cancels the height leaves no box height at all, and no negative one.
func TestRuleLiftBelowItsOwnHeight(t *testing.T) {
	r := newRenderer(t)
	_, m, err := r.RenderSVGMetrics(`\rule[-9pt]{10pt}{4pt}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if m.Height != 0 || m.Depth-9 > 0.02 || m.Depth-9 < -0.02 {
		t.Errorf("h=%.2f d=%.2f, want h0 d9 (clamped, never negative)", m.Height, m.Depth)
	}
}
