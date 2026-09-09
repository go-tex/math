// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"testing"

	"github.com/go-opentype/opentype"
)

// A large operator grows in display style. TeX does it by walking one step up the
// font's charlist:
//
//	if (cur_style<text_style)and(char_tag(cur_i)=list_tag) then {make it larger}
//	  begin c:=rem_byte(cur_i); …; character(nucleus(q)):=c; end   tex.web:14685-14691
//
// An OpenType MATH font says the same with the glyph's vertical MathVariants and
// MathConstants' DisplayOperatorMinHeight. RenderDisplaySVG's own doc comment
// already promised "larger operators"; the code did not do it, so a display \int
// was set at its inline size — 28px tall at 200dpi where the reference sets 62px.
func TestDisplayGrowsALargeOperator(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{`\int`, `\sum`, `\prod`, `\oint`, `\bigcup`} {
		_, inline, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("inline %s: %v", tex, err)
		}
		_, display, err := r.RenderDisplaySVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("display %s: %v", tex, err)
		}
		in := inline.Height + inline.Depth
		dp := display.Height + display.Depth
		if dp <= in {
			t.Errorf("%s: display %.1f is not taller than inline %.1f", tex, dp, in)
		}
	}
}

// An ordinary atom is NOT grown: only large operators take the display variant, or
// every letter beside the operator would grow with it.
func TestDisplayLeavesOrdinaryAtomsAlone(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{`x`, `\alpha`, `+`, `\leq`} {
		_, inline, err := r.RenderSVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("inline %s: %v", tex, err)
		}
		_, display, err := r.RenderDisplaySVGMetrics(tex, 32)
		if err != nil {
			t.Fatalf("display %s: %v", tex, err)
		}
		if got, want := display.Height+display.Depth, inline.Height+inline.Depth; got != want {
			t.Errorf("%s: display %.1f != inline %.1f — an ordinary atom must not grow", tex, got, want)
		}
	}
}

// An operator that already reaches DisplayOperatorMinHeight is returned as it
// stands. No glyph of the default font does — a display \int at 32px measures
// 30.5 against a target of 58 — so the contract is exercised here directly, with
// a box tall enough to satisfy it. It is what keeps a MATH table that omits the
// constant (MathConstant reports 0) from swapping the base for the first variant.
func TestDisplayOperatorKeepsAnOperatorAlreadyTallEnough(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm()), gc: r.gc}
	tall := newBox(clsOp)
	tall.h = e.mc(opentype.DisplayOperatorMinHeight, 32) + 1
	if got := e.displayOperator('∫', 32, clsOp, tall); got != tall {
		t.Errorf("a %.1f-tall operator was replaced although the target is %.1f",
			tall.h, e.mc(opentype.DisplayOperatorMinHeight, 32))
	}
}
