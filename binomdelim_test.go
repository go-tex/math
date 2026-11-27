// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// make_fraction sizes the delimiters of a fraction from the font, not from the
// fraction: delta = delim1 in display style, delim2 otherwise, and var_delimiter
// takes the first variant whose height+depth reaches delta (tex.web:14656-14661,
// :13952-13969). The tests re-derive the delimiter from stretchVertical at that
// target and require the box to be exactly max(delimiter, fraction) — and to come
// out the same however the delimiters were asked for.

func delimBoxOf(r *Renderer, ch rune, display bool) *box {
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	d := fracDelim2
	if display {
		d = fracDelim1
	}
	return e.axisCentre(e.stretchVertical(ch, d*shiftPx, shiftPx, clsOpen), shiftPx)
}

func TestFractionDelimitersTakeDelim1AndDelim2(t *testing.T) {
	r := newRenderer(t)
	for _, tc := range []struct {
		name, pre string
		display   bool
	}{{"text", "", false}, {"display", `\displaystyle `, true}} {
		d := delimBoxOf(r, '(', tc.display)
		for _, c := range [][2]string{{"n", "k"}, {"a", "b"}, {"x^2", "y_j"}} {
			inner := boxOf(t, r, tc.pre+`\genfrac{}{}{0pt}{}{`+c[0]+`}{`+c[1]+`}`)
			want := boxOf(t, r, tc.pre+`\binom{`+c[0]+`}{`+c[1]+`}`)
			h, dp := d.h, d.d
			if inner.Height > h {
				h = inner.Height
			}
			if inner.Depth > dp {
				dp = inner.Depth
			}
			near(t, tc.name+` \binom{`+c[0]+`}{`+c[1]+`} height`, want.Height, h)
			near(t, tc.name+` \binom{`+c[0]+`}{`+c[1]+`} depth`, want.Depth, dp)
			// the delimiter reaches delta: its height+depth is at least delim1/delim2
			// unless the face has no variant that large.
			if d.h+d.d < 0.9*shiftPx {
				t.Errorf("%s: delimiter %.2f+%.2f is below delim2", tc.name, d.h, d.d)
			}
		}
	}
}

func TestFractionDelimiterSpellingsAgree(t *testing.T) {
	r := newRenderer(t)
	for _, pre := range []string{"", `\displaystyle `} {
		want := boxOf(t, r, pre+`\binom{n}{k}`)
		for _, s := range []string{
			`\genfrac(){0pt}{}{n}{k}`,
		} {
			got := boxOf(t, r, pre+s)
			near(t, pre+s+" height", got.Height, want.Height)
			near(t, pre+s+" depth", got.Depth, want.Depth)
		}
		// a rule fraction is delimited the same way
		fr := boxOf(t, r, pre+`\genfrac(){}{}{n}{k}`)
		plain := boxOf(t, r, pre+`\genfrac{}{}{}{}{n}{k}`)
		if fr.Height < plain.Height || fr.Depth < plain.Depth {
			t.Errorf("%s delimited fraction %.2f/%.2f smaller than bare %.2f/%.2f", pre, fr.Height, fr.Depth, plain.Height, plain.Depth)
		}
	}
}

func TestDisplayFractionDelimiterIsLargerThanText(t *testing.T) {
	r := newRenderer(t)
	text := boxOf(t, r, `\binom{n}{k}`)
	disp := boxOf(t, r, `\displaystyle\binom{n}{k}`)
	if disp.Height+disp.Depth <= text.Height+text.Depth {
		t.Errorf("display %.2f not larger than text %.2f", disp.Height+disp.Depth, text.Height+text.Depth)
	}
}
