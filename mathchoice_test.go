// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	gomath "math"
	"testing"
)

// \mathchoice{D}{T}{S}{SS} typesets ONE body and discards three. The order is confirmed by
// latex.ltx:11179-11184 (\mathpalette names all four styles) and amsmath.sty:266-270 (the
// same order in fonts, whose first TWO branches both use \textfont — which is why a size
// alone cannot select and the style LEVEL had to be added).
//
// The tests put a DIFFERENT, recognisable body in each branch and assert which one came
// out, because that is the only claim: "it renders" passes for any of the four.

// Display and text style pick branches 0 and 1, and they are told apart by the display
// flag alone — both sit at script level 0.
func TestMathchoicePicksDisplayAndTextByTheFlag(t *testing.T) {
	r := newRenderer(t)
	// Four branches, each a different width: a, ab, abc, abcd.
	const mc = `\mathchoice{a}{ab}{abc}{abcd}`
	w := func(tex string, display bool) float64 {
		t.Helper()
		var m Metrics
		var err error
		if display {
			_, m, err = r.RenderDisplaySVGMetrics(tex, 32)
		} else {
			_, m, err = r.RenderSVGMetrics(tex, 32)
		}
		if err != nil {
			t.Fatalf("render(%q, display=%v): %v", tex, display, err)
		}
		return m.Width
	}
	if got, want := w(mc, true), w(`a`, true); gomath.Abs(got-want) > 1e-9 {
		t.Errorf("display style chose a body %.2f wide, want %.2f (branch 0, `a`)", got, want)
	}
	if got, want := w(mc, false), w(`ab`, false); gomath.Abs(got-want) > 1e-9 {
		t.Errorf("text style chose a body %.2f wide, want %.2f (branch 1, `ab`)", got, want)
	}
	// ⛔ And the two must DIFFER, or the test proves nothing about the flag.
	if gomath.Abs(w(mc, true)-w(mc, false)) < 1e-9 {
		t.Error("display and text chose the same branch: the display flag is not reaching the selection")
	}
}

// A script picks branch 2 and a script-of-a-script branch 3. Asserted through the BRANCH
// INDEX rather than a rendered width, because a script's body is set at a smaller size and
// comparing widths across sizes proves nothing.
func TestMathchoiceBranchIndexFollowsTheScriptLevel(t *testing.T) {
	for _, c := range []struct {
		sty  style
		want int
		name string
	}{
		{style{level: 0, display: true}, 0, "display"},
		{style{level: 0}, 1, "text"},
		{style{level: 1}, 2, "script"},
		{style{level: 2}, 3, "scriptscript"},
		{style{level: 3}, 3, "a script of a scriptscript is scriptscript again"},
		{style{level: 1, display: true}, 2, "the display flag does not survive a script"},
	} {
		if got := mathchoiceBranch(c.sty); got != c.want {
			t.Errorf("%s: branch %d, want %d", c.name, got, c.want)
		}
	}
}

// The style level must actually be carried by the constructors, or the branch function
// above is exercised on values nothing produces.
func TestTheScriptConstructorsRaiseTheLevel(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	base := style{px: 32, display: true, spacious: true}
	if got := base.script(e).level; got != 1 {
		t.Errorf("script level = %d, want 1", got)
	}
	if got := base.scriptScript(e).level; got != 2 {
		t.Errorf("scriptScript level = %d, want 2", got)
	}
	if got := base.script(e).script(e).level; got != 2 {
		t.Errorf("a script of a script = %d, want 2", got)
	}
	if got := base.inner().level; got != 0 {
		t.Errorf("inner() changed the level to %d", got)
	}
	// Appendix G rule 15b: a TEXT-style fraction's parts drop one style, a display-style
	// one's do not.
	if got := base.fracInner(e).level; got != 0 {
		t.Errorf("a display fraction's parts are at level %d, want 0", got)
	}
	textSty := style{px: 32, spacious: true}
	if got := textSty.fracInner(e).level; got != 1 {
		t.Errorf("a text fraction's parts are at level %d, want 1", got)
	}
}

// End to end: the branch really changes inside a script.
func TestMathchoiceChoosesADifferentBranchInsideAScript(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	// Branch 2 is the only one carrying a glyph, so the script form has ink and the
	// display form has none.
	const mc = `\mathchoice{}{}{x}{}`
	if svg(`a`+mc) != svg(`a`) {
		t.Error("display style did not choose the EMPTY branch 0")
	}
	if svg(`a^{`+mc+`}`) == svg(`a^{}`) {
		t.Error("a script did not choose branch 2, which carries the x")
	}
}

// ⛔ All four groups are CONSUMED whatever the style. A selector that reads only the branch
// it wants leaves the other three in the stream, and they are then typeset as ordinary
// material — which renders, so nothing would report it.
func TestMathchoiceConsumesAllFourGroups(t *testing.T) {
	r := newRenderer(t)
	svg := func(tex string) string {
		t.Helper()
		s, err := r.RenderDisplaySVG(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return s
	}
	// Branch 0 is empty and the other three carry ink: a display must show NOTHING of
	// them, and the z that follows must land where it would have with no \mathchoice.
	if got, want := svg(`\mathchoice{}{a}{b}{c}z`), svg(`z`); got != want {
		t.Error("the unchosen branches leaked into the formula")
	}
}

// Fewer than four groups is an ERROR, not a guess. A selector with a missing branch would
// silently pick the wrong body, which renders.
func TestMathchoiceWithTooFewGroupsIsAnError(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\mathchoice{a}{b}{c}`,
		`\mathchoice{a}{b}`,
		`\mathchoice{a}`,
		`\mathchoice`,
		`\mathchoice{a}{b}{c}x`, // an x is not a group
	} {
		if _, err := r.RenderSVG(tex, 32); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
}

// The corpus forms, including amsmath's own \intkern@ shape and a paper's negative skips.
func TestTheCorpusMathchoiceFormsRender(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\mathchoice{\mkern-3mu}{}{}{}`,
		`\mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}`,
		`{\mathrm{1}\mkern-4mu{\mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}}}`,
		`\mathchoice{\sum}{\sum}{\textstyle\sum}{\textstyle\sum}`,
		`x^{\mathchoice{a}{b}{c}{d}}`,
		`\frac{\mathchoice{a}{b}{c}{d}}{2}`,
	} {
		renderOK(t, r, tex)
	}
}

// \mkern and \mskip insert space in MATH UNITS, 18mu to the em — the same unit the space
// table works in. They are here because amsmath's \intkern@ is
// \mkern-6mu\mathchoice{\mkern-3mu}{}{}{} (amsmath.sty:654), so \mathchoice would select a
// branch it could not typeset without them.
func TestMuKernMeasuresInMathUnits(t *testing.T) {
	r := newRenderer(t)
	w := func(tex string) float64 {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m.Width
	}
	base := w(`ab`)
	// 18mu is one em, which is the size in pixels.
	if got, want := w(`a\mkern18mu b`)-base, 32.0; gomath.Abs(got-want) > 1e-9 {
		t.Errorf(`\mkern18mu added %.3f, want %.3f (one em at 32px)`, got, want)
	}
	// 3mu is what \, gives, so the two must agree exactly.
	if got, want := w(`a\mkern3mu b`), w(`a\,b`); gomath.Abs(got-want) > 1e-9 {
		t.Errorf(`\mkern3mu gives %.3f, \, gives %.3f`, got, want)
	}
	// A NEGATIVE kern pulls back, which is what every corpus use of it does.
	if got := w(`a\mkern-3mu b`); got >= base {
		t.Errorf(`\mkern-3mu gave %.3f, not less than %.3f`, got, base)
	}
	// \mskip takes glue; the natural width is used and the stretch is discarded, so these
	// three must all measure the same.
	plain := w(`a\mskip3mu b`)
	for _, tex := range []string{`a\mskip3mu plus 1mu b`, `a\mskip3mu plus 1mu minus 2mu b`} {
		if got := w(tex); gomath.Abs(got-plain) > 1e-9 {
			t.Errorf("%s gives %.3f, want %.3f: the stretch must be discarded, not added", tex, got, plain)
		}
	}
}

// A missing or non-mu dimension is an ERROR, not a zero: \mkern with nothing readable
// would leave its argument in the formula to be typeset as digits.
func TestMuKernWithoutAReadableDimensionIsAnError(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`a\mkern b`,
		`a\mkern`,
		`a\mkern3pt b`, // pt is not a math unit
		`a\mkern3 b`,
	} {
		if _, err := r.RenderSVG(tex, 32); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
}

// The error paths, each of which leaves something for the caller to report rather than
// guessing. They exist because a selector that guesses picks the WRONG body, which renders.
func TestMathchoiceAndMuKernErrorPaths(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct{ what, tex string }{
		{"an unknown command inside the SELECTED branch", `\mathchoice{\nosuchmaththing}{b}{c}{d}`},
		{"an unbalanced last group", `\mathchoice{a}{b}{c}{d`},
		{"a dimension ParseFloat cannot read", `a\mkern.mu b`},
	} {
		if _, err := r.RenderDisplaySVG(c.tex, 32); err == nil {
			t.Errorf("%s did not fail: %s", c.what, c.tex)
		}
	}
	// A glue component this cannot read is SKIPPED, not an error: \mskip3mu plus1fil must
	// still give its 3mu natural width, because refusing the whole skip over an
	// unreadable stretch would lose space the document asked for.
	w := func(tex string) float64 {
		t.Helper()
		_, m, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		return m.Width
	}
	if got, want := w(`a\mskip3mu plus1fil b`), w(`a\mskip3mu b`); gomath.Abs(got-want) > 1e-9 {
		t.Errorf(`\mskip3mu plus1fil is %.3f, want %.3f: the unreadable stretch changed the width`, got, want)
	}
}

// glueUnitLen matches the longest spelling first, because fil is a prefix of fill and
// filll: a shortest-first match would leave an l behind to be typeset as a letter. And a
// unit that is not there must give 0, so the fallback does not swallow the material after
// the glue — which it did, eating the b out of "\mskip3mu plus1fil b".
func TestGlueUnitLengthMatchesLongestFirst(t *testing.T) {
	for _, c := range []struct {
		text string
		want int
	}{
		{"filll", 5}, {"fill", 4}, {"fil", 3},
		{"mu", 2}, {"pt", 2}, {"sp", 2},
		{"fillx", 4}, // fill, then an x that is NOT part of the unit
		{"b", 0}, {"", 0}, {"xyz", 0},
	} {
		toks := tokenize(c.text)
		if got := glueUnitLen(toks, 0); got != c.want {
			t.Errorf("glueUnitLen(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}
