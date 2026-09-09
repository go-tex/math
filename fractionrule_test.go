// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"regexp"
	"strconv"
	"testing"
)

var rectRe = regexp.MustCompile(`<rect x="([-0-9.]+)" y="[-0-9.]+" width="([0-9.]+)"`)

// ruleSpan returns the x and width of the widest <rect> in svg — for a fraction,
// its bar.
func ruleSpan(t *testing.T, svg string) (x, w float64) {
	t.Helper()
	for _, m := range rectRe.FindAllStringSubmatch(svg, -1) {
		rw, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			t.Fatalf("width %q: %v", m[2], err)
		}
		if rw > w {
			w = rw
			x, _ = strconv.ParseFloat(m[1], 64)
		}
	}
	if w == 0 {
		t.Fatal("no rule in the fraction")
	}
	return x, w
}

// TeX flanks a fraction with the two delimiters its noad names, and \frac names
// none: var_delimiter then returns an EMPTY box — "use this width if no delimiter
// was found" (tex.web:13930), that width being \nulldelimiterspace — and the two
// boxes are hpacked OUTSIDE the vlist holding the rule. The rule is a node INSIDE
// that vlist, so it takes the vlist's width: width(x), "this also equals
// width(z)", the equalised numerator/denominator width.
//
// So the bar spans max(num, den) and the padding stays blank on both sides. Drawn
// across the whole box it overhung by 2.4pt at 10pt — invisible on a wide
// fraction and enormous on a narrow one: against tectonic at 600dpi \frac{a}{b}
// was 59% too wide (70px against 44) where \frac{a+b}{c+d} was 3.9%.
func TestFractionRuleSpansTheContentNotThePadding(t *testing.T) {
	r := newRenderer(t)
	const px = 10
	// \nulldelimiterspace is 1.2pt at 10pt, one on each side.
	const want = 2 * 1.2
	for _, tex := range []string{`\frac{a}{b}`, `\frac{a+b}{c+d}`, `\frac{x}{yyyyy}`, `\tfrac{a}{b}`} {
		svg, m, err := r.RenderSVGMetrics(tex, px)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		x, w := ruleSpan(t, svg)
		if got := m.Width - w; got < want-0.01 || got > want+0.01 {
			t.Errorf("%s: box %.3f, rule %.3f — the padding left blank is %.3f, want %.3f (2 x \\nulldelimiterspace)",
				tex, m.Width, w, got, want)
		}
		// and it is centred: the blank is the same on each side.
		if got := m.Width - w - x; got < x-0.01 || got > x+0.01 {
			t.Errorf("%s: rule starts at %.3f but leaves %.3f on the right — the two null delimiters are equal",
				tex, x, got)
		}
	}
}

// The padding is a CONSTANT of the size, not a share of the content: two
// fractions of very different widths leave exactly the same blank.
func TestFractionPaddingDoesNotScaleWithContent(t *testing.T) {
	r := newRenderer(t)
	blank := func(tex string) float64 {
		svg, m, err := r.RenderSVGMetrics(tex, 10)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		_, w := ruleSpan(t, svg)
		return m.Width - w
	}
	narrow, wide := blank(`\frac{a}{b}`), blank(`\frac{abcdefgh}{ijklmnop}`)
	if narrow < wide-0.01 || narrow > wide+0.01 {
		t.Errorf("a narrow fraction leaves %.3f blank and a wide one %.3f", narrow, wide)
	}
}
