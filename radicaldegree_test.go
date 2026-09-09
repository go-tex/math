// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"testing"

	"github.com/go-opentype/opentype"
)

// A radical's degree sits INSIDE the radical sign's own vertical extent, so
// \sqrt[3]{x} is exactly as tall as \sqrt{x}. Two sources say it:
//
//	\setbox\rootbox\hbox{$\m@th\scriptscriptstyle{#1}$}%   latex.ltx:11187
//	\mkern5mu\raise.6\dimen@\copy\rootbox                  latex.ltx:11192
//
// — the degree raised by .6 of the radical box's (height − depth) — and the MATH
// table's RadicalDegreeBottomRaisePercent, "height of the bottom of the radical
// degree, if such be present, in proportion to the ascender of the radical sign".
//
// Placed with its BASELINE at the rule instead, the degree floated entirely above
// the sign, and since the box height did not count it, above the box as well:
// measured against tectonic at 600dpi, \sqrt[3]{x} was 136px tall where both the
// reference's \sqrt[3]{x} and its \sqrt{x} are 83.
func TestRadicalDegreeDoesNotRaiseTheBox(t *testing.T) {
	r := newRenderer(t)
	_, plain, err := r.RenderDisplaySVGMetrics(`\sqrt{x}`, 20)
	if err != nil {
		t.Fatalf(`\sqrt{x}: %v`, err)
	}
	for _, tex := range []string{`\sqrt[3]{x}`, `\sqrt[10]{x}`, `\sqrt[n+1]{x}`} {
		_, rooted, err := r.RenderDisplaySVGMetrics(tex, 20)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		if rooted.Height != plain.Height || rooted.Depth != plain.Depth {
			t.Errorf("%s is %.3f+%.3f tall where \\sqrt{x} is %.3f+%.3f — the degree sits inside the sign",
				tex, rooted.Height, rooted.Depth, plain.Height, plain.Depth)
		}
	}
}

// A wide degree does widen the box, though: the reference goes 111px to 121px
// between \sqrt[3] and \sqrt[10] while staying 83px tall.
func TestAWideRadicalDegreeWidensTheBox(t *testing.T) {
	r := newRenderer(t)
	w := func(tex string) float64 {
		_, m, err := r.RenderDisplaySVGMetrics(tex, 20)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		return m.Width
	}
	narrow, wide := w(`\sqrt[3]{x}`), w(`\sqrt[10]{x}`)
	if wide <= narrow {
		t.Errorf(`\sqrt[10]{x} is %.3f wide and \sqrt[3]{x} %.3f — a wider degree must push the sign right`, wide, narrow)
	}
}

// And it is set in scriptSCRIPT style, the size latex.ltx names. TeX has three
// sizes rather than a ladder — \scriptscriptfont is its own font — so the check is
// against the font's second constant and not the script size scaled twice.
func TestRadicalDegreeIsSetInScriptScriptStyle(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	const px = 20
	ss, s := e.scriptScriptSize(px), e.scriptSize(px)
	if ss >= s {
		t.Fatalf("scriptscript size %d is not smaller than script size %d", ss, s)
	}
	if want := px * e.face(px).MathConstant(opentype.ScriptScriptPercentScaleDown) / 100; ss != want {
		t.Errorf("scriptScriptSize = %d, want %d (ScriptScriptPercentScaleDown)", ss, want)
	}
	// End to end: a digit ADDED to the degree widens the box by that digit's width
	// at the degree's own size. The difference of two degrees is the instrument,
	// because the width a degree adds on its own is clamped at zero — the negative
	// RadicalKernAfterDegree tucks a short degree over the sign's left arm, which
	// is what the reference does too (\sqrt[3]{x} and \sqrt{x} are both 111px wide
	// there, and \sqrt[10]{x} is 121px).
	w := func(tex string) float64 {
		_, m, err := r.RenderDisplaySVGMetrics(tex, px)
		if err != nil {
			t.Fatalf("%s: %v", tex, err)
		}
		return m.Width
	}
	added := w(`\sqrt[888]{x}`) - w(`\sqrt[88]{x}`)
	atSS := e.mustGlyph('8', ss, clsOrd).w
	atS := e.mustGlyph('8', s, clsOrd).w
	if abs(added-atSS) >= abs(added-atS) {
		t.Errorf("a digit of degree adds %.3f: scriptscript predicts %.3f, script %.3f — it is set one size too large",
			added, atSS, atS)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
