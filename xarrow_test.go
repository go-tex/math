// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"strings"
	"testing"
)

// TestXArrow_Renders checks that every extensible-arrow command renders without
// dropping and produces a well-formed SVG in both inline and display style.
func TestXArrow_Renders(t *testing.T) {
	r := newRenderer(t)
	cases := []string{
		`\xrightarrow{f}`,
		`\xrightarrow[g]{f}`,
		`\xleftarrow{n\to\infty}`,
		`\xleftarrow[a]{b}`,
		`\xleftrightarrow{x}`,
		`\xhookrightarrow{i}`,
		`\xhookleftarrow{j}`,
		`\xmapsto{\phi}`,
		`\xRightarrow{p}`,
		`\xLeftarrow{q}`,
		`\xrightarrow{}`, // empty label must still render a bare arrow
		`A \xrightarrow{f} B \xleftarrow[u]{v} C`,
	}
	for _, c := range cases {
		svg := renderOK(t, r, c)
		// Every arrow draws at least one filled arrowhead <path …Z> and a shaft <rect.
		if !strings.Contains(svg, "<path") || !strings.Contains(svg, "<rect") {
			t.Errorf("render(%q): missing arrowhead/shaft: %.80s", c, svg)
		}
	}
}

// TestXArrow_StretchesToLabel checks the shaft grows with the label width.
func TestXArrow_StretchesToLabel(t *testing.T) {
	r := newRenderer(t)
	_, mShort, err := r.RenderSVGMetrics(`\xrightarrow{a}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	_, mLong, err := r.RenderSVGMetrics(`\xrightarrow{aaaaaaaa}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !(mLong.Width > mShort.Width) {
		t.Errorf("arrow did not stretch: short=%.2f long=%.2f", mShort.Width, mLong.Width)
	}
	// The under-label also stretches the shaft.
	_, mUnder, err := r.RenderSVGMetrics(`\xrightarrow[wwwwwwww]{a}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !(mUnder.Width > mShort.Width) {
		t.Errorf("under-label did not stretch: %.2f vs %.2f", mUnder.Width, mShort.Width)
	}
}

// TestXArrow_LabelsPlaced checks the superscript raises the box and the optional
// subscript deepens it.
func TestXArrow_LabelsPlaced(t *testing.T) {
	r := newRenderer(t)
	_, bare, err := r.RenderSVGMetrics(`\xrightarrow{}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	_, over, err := r.RenderSVGMetrics(`\xrightarrow{X}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !(over.Height > bare.Height) {
		t.Errorf("over-label did not raise height: %.2f vs %.2f", over.Height, bare.Height)
	}
	_, under, err := r.RenderSVGMetrics(`\xrightarrow[Y]{X}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !(under.Depth > over.Depth) {
		t.Errorf("under-label did not deepen box: %.2f vs %.2f", under.Depth, over.Depth)
	}
}

// TestXArrow_Decorations checks mapsto adds a tail bar and the hook variants add
// a stroked (fill:none) hook path that the plain arrows do not.
func TestXArrow_Decorations(t *testing.T) {
	r := newRenderer(t)
	plain, err := r.RenderSVG(`\xrightarrow{f}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, `fill="none"`) {
		t.Errorf("plain arrow should not contain a stroked hook path")
	}
	hooked, err := r.RenderSVG(`\xhookrightarrow{f}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hooked, `fill="none"`) {
		t.Errorf("hooked arrow missing stroked hook path")
	}
	// mapsto has more rects (extra tail bar) than the equivalent plain arrow.
	mapsto, err := r.RenderSVG(`\xmapsto{f}`, 40)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(mapsto, "<rect") <= strings.Count(plain, "<rect") {
		t.Errorf("xmapsto should add a tail bar rect: mapsto=%d plain=%d",
			strings.Count(mapsto, "<rect"), strings.Count(plain, "<rect"))
	}
}

// TestXArrowSpec covers the command-name classifier, including the non-arrow case.
func TestXArrowSpec(t *testing.T) {
	l, r2, h, m, ok := xArrowSpec("xrightarrow")
	if !ok || l || !r2 || h || m {
		t.Errorf("xrightarrow spec wrong: %v %v %v %v %v", l, r2, h, m, ok)
	}
	l, r2, h, m, ok = xArrowSpec("xleftarrow")
	if !ok || !l || r2 || h || m {
		t.Errorf("xleftarrow spec wrong: %v %v %v %v %v", l, r2, h, m, ok)
	}
	l, r2, h, _, ok = xArrowSpec("xleftrightarrow")
	if !ok || !l || !r2 || h {
		t.Errorf("xleftrightarrow spec wrong")
	}
	_, _, h, _, ok = xArrowSpec("xhookrightarrow")
	if !ok || !h {
		t.Errorf("xhookrightarrow should be hooked")
	}
	_, _, _, m, ok = xArrowSpec("xmapsto")
	if !ok || !m {
		t.Errorf("xmapsto should be a mapsto")
	}
	if _, _, _, _, ok := xArrowSpec("frac"); ok {
		t.Errorf("frac is not an extensible arrow")
	}
}
