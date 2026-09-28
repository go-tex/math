// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"regexp"
	"strconv"
	"testing"
)

// \ooalign is an OVERLAY: every row on the same baseline, the width of the widest row that
// declares one. latex.ltx:628 is \def\ooalign{\lineskiplimit-\maxdimen \oalign}, and that
// limit is where the overlay comes from — \baselineskip is zero (:625), so the interline
// glue tex.web §679 computes is negative, and a limit of −\maxdimen means it is never below
// the limit, so \baselineskip is used rather than \lineskip.
//
// 102 equations over 4 papers, all four writing it themselves. It became visible only when
// \mathpalette stopped masking it (go-tex/engine#502).

func ooWH(t *testing.T, r *Renderer, tex string) (w, h, d float64) {
	t.Helper()
	_, m, err := r.RenderSVGMetrics(tex, 32)
	if err != nil {
		t.Fatalf("render(%q): %v", tex, err)
	}
	return m.Width, m.Height, m.Depth
}

// ⛔ The rows OVERLAY: the width is the MAXIMUM, never the sum. Asserting "it renders"
// would pass for a layout that set the rows side by side, which is what a grid would do.
func TestOoalignOverlaysRatherThanStacksSideways(t *testing.T) {
	r := newRenderer(t)
	wa, _, _ := ooWH(t, r, `a`)
	wm, _, _ := ooWH(t, r, `\cup`)
	got, _, _ := ooWH(t, r, `\ooalign{a\cr\cup}`)
	if want := gomath.Max(wa, wm); gomath.Abs(got-want) > 1e-9 {
		t.Errorf("width %.2f, want %.2f (the wider of the two rows)", got, want)
	}
	if gomath.Abs(got-(wa+wm)) < 1e-9 {
		t.Errorf("width %.2f is the SUM: the rows were set side by side, not overlaid", got)
	}
	// The extents are the maxima too, and they come from the taller row.
	_, ha, da := ooWH(t, r, `a`)
	_, hm, dm := ooWH(t, r, `\cup`)
	_, h, d := ooWH(t, r, `\ooalign{a\cr\cup}`)
	if gomath.Abs(h-gomath.Max(ha, hm)) > 1e-9 || gomath.Abs(d-gomath.Max(da, dm)) > 1e-9 {
		t.Errorf("extents h=%.2f d=%.2f, want %.2f and %.2f", h, d, gomath.Max(ha, hm), gomath.Max(da, dm))
	}
}

// ⛔ Every row sits at dy = 0. That single number IS the overlay, and it is the only place
// it can be checked: every extent above is a maximum and would hold for rows placed at
// different heights too.
func TestOoalignPutsEveryRowOnTheSameBaseline(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	sty := style{px: 32, display: true, spacious: true}
	// ⚠ parseOoalign is called DIRECTLY. Going through parseList wraps the result in an
	// hlist, which adds a translate of its own, so the row placements would sit at depth 1
	// and a depth-0 scan would find only the wrapper.
	b, _, _, _, err := e.parseOoalign(tokenize(`{a\cr b\cr c}`), sty)
	if err != nil {
		t.Fatal(err)
	}
	ys := outerTranslateYs(b.String())
	if len(ys) != 3 {
		t.Fatalf("expected 3 row placements, got %d", len(ys))
	}
	for i, y := range ys {
		if gomath.Abs(y) > 1e-9 {
			t.Errorf("row %d is at dy=%.3f: this is not an overlay", i, y)
		}
	}
}

// \hidewidth is a −1000pt skip with infinite stretch (latex.ltx:492), so a row flanked by
// two of them contributes NOTHING to the width and is centred in it. The corpus form is a
// \cup with a \cdot centred on it, and it must measure exactly as the \cup alone.
func TestHidewidthRemovesTheRowsWidthContribution(t *testing.T) {
	r := newRenderer(t)
	wc, hc, dc := ooWH(t, r, `\cup`)
	w, h, d := ooWH(t, r, `\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}`)
	if gomath.Abs(w-wc) > 1e-9 || gomath.Abs(h-hc) > 1e-9 || gomath.Abs(d-dc) > 1e-9 {
		t.Errorf("the overlay is %.2f×%.2f/%.2f, \\cup alone is %.2f×%.2f/%.2f: "+
			"the hidden row is contributing", w, h, d, wc, hc, dc)
	}
	// ⛔ Without \hidewidth the same two rows DO set the width by the wider of them, so
	// the test above cannot be passing because the \cdot is simply narrower.
	wd, _, _ := ooWH(t, r, `\cdot`)
	if wd >= wc {
		t.Skipf("\\cdot (%.2f) is not narrower than \\cup (%.2f); this witness needs a narrower row", wd, wc)
	}
	// A row with no \hidewidth and a WIDER body must widen the overlay — the converse.
	wide, _, _ := ooWH(t, r, `\ooalign{$\cdot$\cr$\cup$}`)
	if gomath.Abs(wide-wc) > 1e-9 {
		t.Errorf("an unhidden wider row gave %.2f, want %.2f", wide, wc)
	}
	hidden, _, _ := ooWH(t, r, `\ooalign{$\cdot$\cr\hidewidth$\cup$\hidewidth}`)
	if gomath.Abs(hidden-wd) > 1e-9 {
		t.Errorf("hiding the WIDER row gave %.2f, want %.2f (the unhidden row's width)", hidden, wd)
	}
}

// \hidewidth on both sides CENTRES the row; on the left alone it pushes it right. Read from
// the emitted transform, because a width assertion cannot see where the ink went.
func TestHidewidthCentresOrRightAlignsTheRow(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	sty := style{px: 32, display: true, spacious: true}
	// ⚠ The x offsets wanted are the OUTERMOST translates — the ones place() emits, one
	// per row. A flat regex over the SVG finds the rows' own inner glyph placements first,
	// all of which sit at (0,0), and reports every row as unmoved. Depth matters, so the
	// scan tracks it.
	xs := func(tex string) []float64 {
		t.Helper()
		b, _, _, _, err := e.parseOoalign(tokenize(tex), sty)
		if err != nil {
			t.Fatalf("parse(%q): %v", tex, err)
		}
		return outerTranslateXs(b.String())
	}
	both := xs(`{$\cup$\cr\hidewidth$\cdot$\hidewidth}`)
	left := xs(`{$\cup$\cr\hidewidth$\cdot$}`)
	none := xs(`{$\cup$\cr$\cdot$}`)
	if len(both) < 2 || len(left) < 2 || len(none) < 2 {
		t.Fatalf("expected two rows each, got %d/%d/%d", len(both), len(left), len(none))
	}
	// The unhidden row is always at x=0; the hidden one moves.
	if none[1] != 0 {
		t.Errorf("an unhidden row is at x=%.3f, want 0", none[1])
	}
	if both[1] <= 0 {
		t.Errorf("a row hidden on both sides is at x=%.3f, want a positive (centred) offset", both[1])
	}
	if left[1] <= both[1] {
		t.Errorf("hidden on the LEFT only gives x=%.3f, which is not further right than centred %.3f",
			left[1], both[1])
	}
}

// A trailing \cr leaves no empty row — \oalign's own template writes {##\crcr#1\crcr}, so a
// paper ending its last row with \cr is the normal case, not an error.
func TestATrailingCrAddsNoRow(t *testing.T) {
	r := newRenderer(t)
	// ⛔ The measurements alone cannot see the empty-row filter: an empty row is 0 wide
	// and 0 tall, so keeping it changes no width and no extent — an ablation of the filter
	// broke nothing. What it does change is the number of row placements EMITTED, which is
	// its actual contract, so that is what is asserted first.
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	sty := style{px: 32, display: true, spacious: true}
	for _, c := range []struct {
		tex  string
		rows int
	}{
		{`{a\cr b}`, 2},
		{`{a\cr b\cr}`, 2},
		{`{a\cr b\crcr}`, 2},
		{`{a\cr\cr b}`, 2},
		{`{a}`, 1},
	} {
		b, _, _, _, err := e.parseOoalign(tokenize(c.tex), sty)
		if err != nil {
			t.Fatalf("parse(%q): %v", c.tex, err)
		}
		if got := len(outerTranslateXs(b.String())); got != c.rows {
			t.Errorf("%s emitted %d row placements, want %d", c.tex, got, c.rows)
		}
	}
	w1, h1, d1 := ooWH(t, r, `\ooalign{a\cr b}`)
	for _, tex := range []string{`\ooalign{a\cr b\cr}`, `\ooalign{a\cr b\crcr}`, `\ooalign{a\crcr b\cr}`} {
		w, h, d := ooWH(t, r, tex)
		if gomath.Abs(w-w1) > 1e-9 || gomath.Abs(h-h1) > 1e-9 || gomath.Abs(d-d1) > 1e-9 {
			t.Errorf("%s measures %.2f×%.2f/%.2f, want %.2f×%.2f/%.2f", tex, w, h, d, w1, h1, d1)
		}
	}
}

// A bare \hidewidth is CONSUMED and not turned into a −1000pt kern, which would move the
// rest of the formula a foot to the left.
func TestABareHidewidthIsConsumedAndNotAHugeNegativeKern(t *testing.T) {
	r := newRenderer(t)
	plain, _, _ := ooWH(t, r, `ab`)
	got, _, _ := ooWH(t, r, `a\hidewidth b`)
	if got < plain-1 {
		t.Errorf(`a\hidewidth b is %.2f wide against ab's %.2f: the −1000pt skip was emitted`, got, plain)
	}
}

// No group is an ERROR, because the rows it holds are the content: stripping a malformed
// \ooalign would lose them silently.
func TestOoalignWithoutAGroupIsAnError(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{`\ooalign`, `\ooalign a`, `\ooalign{a\cr b`} {
		if _, err := r.RenderSVG(tex, 32); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
}

// The corpus form, end to end, and inside the wrapper it arrives in.
func TestTheCorpusOoalignFormRenders(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}`,
		`a\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}b`,
		`x^{\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}}`,
		`\mathbin{\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}}`,
	} {
		renderOK(t, r, tex)
	}
}

// outerTranslateYs is outerTranslateXs for the second coordinate.
func outerTranslateYs(svg string) []float64 { return outerTranslates(svg, 2) }

// outerTranslateXs returns the x of every translate at nesting depth ZERO of an SVG
// fragment — the row placements, not the glyph placements inside them.
func outerTranslateXs(svg string) []float64 { return outerTranslates(svg, 1) }

func outerTranslates(svg string, group int) []float64 {
	reOpen := regexp.MustCompile(`^<g[^>]*?transform="translate\((-?[0-9.]+),(-?[0-9.]+)\)"[^>]*>`)
	var out []float64
	depth := 0
	for i := 0; i < len(svg); {
		if svg[i] == '<' {
			if depth == 0 {
				if m := reOpen.FindStringSubmatch(svg[i:]); m != nil {
					if v, err := strconv.ParseFloat(m[group], 64); err == nil {
						out = append(out, v)
					}
				}
			}
			switch {
			case len(svg) > i+2 && svg[i:i+3] == "<g ", len(svg) > i+1 && svg[i:i+2] == "<g":
				depth++
			case len(svg) > i+3 && svg[i:i+4] == "</g>":
				depth--
			}
		}
		i++
	}
	return out
}

// The remaining paths, each of which exists for a case a paper can write.
func TestOoalignEdgeCases(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	sty := style{px: 32, display: true, spacious: true}

	// A row that will not parse propagates its error rather than being dropped: the rows
	// ARE the content, so a silent drop would lose them.
	if _, _, _, _, err := e.parseOoalign(tokenize(`{a\cr\nosuchmaththing}`), sty); err == nil {
		t.Error("a row with an unknown command did not fail")
	}

	// An entirely empty group is an empty box, not an error — \ooalign{} is degenerate but
	// well-formed, and \oalign's own template can produce it.
	b, _, _, _, err := e.parseOoalign(tokenize(`{}`), sty)
	if err != nil {
		t.Fatalf(`\ooalign{} failed: %v`, err)
	}
	if b.w != 0 || b.h != 0 || b.d != 0 {
		t.Errorf(`\ooalign{} is %vx%v/%v, want an empty box`, b.w, b.h, b.d)
	}

	// ⛔ When EVERY row is hidden there is no declared width, and the fallback takes the
	// widest row: without it the overlay would collapse to zero and the ink would spill
	// outside its own box.
	all, _, _, _, err := e.parseOoalign(tokenize(`{\hidewidth$\cup$\hidewidth\cr\hidewidth$\cdot$\hidewidth}`), sty)
	if err != nil {
		t.Fatal(err)
	}
	wc, _, _ := ooWH(t, r, `\cup`)
	if gomath.Abs(all.w-wc) > 1e-9 {
		t.Errorf("all rows hidden gave width %.2f, want %.2f (the widest row)", all.w, wc)
	}

	// ⚠ Braces are tracked while splitting, and this case does NOT prove it: one \cr sits
	// at depth zero either way. Every input that WOULD distinguish the counter fails the
	// same way with or without it — see the note in splitOoalignRows. Kept as a shape the
	// corpus writes, not as a witness.
	nested, _, _, _, err := e.parseOoalign(tokenize(`{{ab}\cr c}`), sty)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(outerTranslateXs(nested.String())); got != 2 {
		t.Errorf("brace tracking miscounted the rows: %d placements, want 2", got)
	}
	// And a \cr INSIDE braces is not a row separator. It belongs to whatever construct
	// opened them — a nested alignment, which this layer does not have — so the row is
	// parsed with the \cr still in it and the parse REPORTS it. That is the honest
	// outcome: splitting there would tear the nested construct in half, and swallowing
	// the \cr would set the two halves as one row without saying so.
	if _, _, _, _, err := e.parseOoalign(tokenize(`{{a\cr b}\cr c}`), sty); err == nil {
		t.Error("a \\cr inside braces was silently accepted")
	}
}
