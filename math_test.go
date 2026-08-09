// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"os"
	"strings"
	"testing"
)

func newRenderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := New(DefaultFont())
	if err != nil {
		t.Fatalf("New(DefaultFont): %v", err)
	}
	return r
}

func TestNew_Errors(t *testing.T) {
	if _, err := New([]byte("not a font")); err == nil {
		t.Error("New(garbage) should fail to parse")
	}
	nomath, err := os.ReadFile("testdata/nomath.otf")
	if err != nil {
		t.Fatalf("read nomath: %v", err)
	}
	if _, err := New(nomath); err == nil {
		t.Error("New(font without MATH) should fail")
	}
	if r := newRenderer(t); r.font == nil {
		t.Error("valid font gave a nil renderer font")
	}
}

func TestRenderSVG_Structure(t *testing.T) {
	r := newRenderer(t)
	cases := []struct {
		tex         string
		wantPathMin int
		wantRect    bool
	}{
		{`x`, 1, false},
		{`x^2 + 1`, 4, false},
		{`a_i`, 2, false},
		{`E = mc^2`, 5, false},
		{`\frac{a}{b}`, 2, true},
		{`\alpha \leq \beta`, 3, false},
		{`\frac{x^2}{\alpha-\beta}`, 4, true},
	}
	for _, c := range cases {
		t.Run(c.tex, func(t *testing.T) {
			svg, err := r.RenderSVG(c.tex, 40)
			if err != nil {
				t.Fatalf("RenderSVG(%q): %v", c.tex, err)
			}
			if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
				t.Errorf("not a complete svg document: %.40s", svg)
			}
			if n := strings.Count(svg, "<path"); n < c.wantPathMin {
				t.Errorf("path count = %d, want >= %d", n, c.wantPathMin)
			}
			if hasRect := strings.Contains(svg, "<rect"); hasRect != c.wantRect {
				t.Errorf("rect present = %v, want %v", hasRect, c.wantRect)
			}
			if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
				t.Errorf("svg contains NaN/Inf: %q", c.tex)
			}
		})
	}
}

func TestRenderSVG_DefaultSizeAndAbsentGlyph(t *testing.T) {
	r := newRenderer(t)
	// sizePx <= 0 falls back to the default.
	if svg, err := r.RenderSVG("x", 0); err != nil || !strings.HasPrefix(svg, "<svg") {
		t.Fatalf("default size render: err=%v", err)
	}
	// A rune the font lacks degrades to an empty box (no crash, no path).
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm())}
	if _, ok := e.glyphBox(0x10FFFF, 40); ok {
		t.Error("glyphBox for an absent rune should report !ok")
	}
	if b := e.mustGlyph(0x10FFFF, 40); b.w != 0 || b.String() != "" {
		t.Errorf("mustGlyph(absent) = %+v, want empty box", b)
	}
}

func TestRenderSVG_ParseErrors(t *testing.T) {
	r := newRenderer(t)
	bad := []string{
		`\nope`,    // unknown command
		`a}`,       // unexpected closing brace
		`{a`,       // missing closing brace
		`^2`,       // superscript with no nucleus
		`_i`,       // subscript with no nucleus
		`\frac{a}`, // frac missing its second argument
		`x^`,       // script missing its atom
		`x^}`,      // script atom is a closing brace
		`\frac}`,   // frac argument is a closing brace
	}
	for _, s := range bad {
		if _, err := r.RenderSVG(s, 40); err == nil {
			t.Errorf("RenderSVG(%q) should error", s)
		}
	}
	// An empty group is valid and renders nothing.
	if svg, err := r.RenderSVG(`{}`, 40); err != nil || strings.Contains(svg, "<path") {
		t.Errorf("empty group: err=%v svg=%q", err, svg)
	}
}

func TestTokenize(t *testing.T) {
	toks := tokenize(`a^2 \alpha \{ x_i {b}`)
	var kinds []tokenKind
	for _, tk := range toks {
		kinds = append(kinds, tk.kind)
	}
	want := []tokenKind{tChar, tSup, tChar, tCtrl, tCtrl, tChar, tSub, tChar, tLBrace, tChar, tRBrace}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("token %d kind = %d, want %d", i, kinds[i], want[i])
		}
	}
	if toks[4].kind != tCtrl || toks[4].text != "{" {
		t.Errorf("control symbol = %+v, want \\{", toks[4])
	}
}

func TestMathItalic(t *testing.T) {
	cases := map[rune]rune{
		'a': 0x1D44E,
		'z': 0x1D467,
		'A': 0x1D434,
		'Z': 0x1D44D,
		'h': 0x210E, // reserved hole → PLANCK CONSTANT
		'1': '1',    // digits unchanged
		'+': '+',
	}
	for in, want := range cases {
		if got := mathItalic(in); got != want {
			t.Errorf("mathItalic(%q) = %U, want %U", in, got, want)
		}
	}
}

func TestScriptSize(t *testing.T) {
	r := newRenderer(t)
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm())}
	if s := e.scriptSize(100); s <= 0 || s >= 100 {
		t.Errorf("scriptSize(100) = %d, want in (0,100)", s)
	}
	if s := e.scriptSize(1); s < 1 {
		t.Errorf("scriptSize(1) = %d, want >= 1", s)
	}
}

func TestFtoa(t *testing.T) {
	cases := map[float64]string{0: "0", 12.0: "12", 1.25: "1.25", -0.5: "-0.5", 3.14159: "3.142"}
	for in, want := range cases {
		if got := ftoa(in); got != want {
			t.Errorf("ftoa(%v) = %q, want %q", in, got, want)
		}
	}
}
