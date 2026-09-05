// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"strings"
	"testing"
)

// \triangleleft and \triangleright are BINARY operators at U+25C1 and U+25B7
// (unicode-math-table.tex:704/694, and fontmath.ltx:264-265 declares both
// \mathbin). They are not the ⊲ ⊳ of \vartriangleleft/right, which amssymb makes
// relations — the two pairs differ in glyph and in spacing.
func TestTriangleBinaryOperators(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{`x \triangleright y`, `x \triangleleft y`} {
		svg, err := r.RenderSVG(tex, 12)
		if err != nil {
			t.Fatalf("render(%q): %v", tex, err)
		}
		if n := strings.Count(svg, "<path"); n < 3 {
			t.Errorf("render(%q) drew %d paths, want the two operands and the triangle", tex, n)
		}
	}
}

// \genfrac{left}{right}{thickness}{style}{num}{den} (amsmath.sty:245-250): an empty
// thickness is \over — the font's rule — and any other is \above that thickness,
// 0pt being no rule at all, which is how \binom itself is defined
// (\genfrac()\z@{}, amsmath.sty:240).
func TestGenfrac(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct{ nom, tex string }{
		{"crochets sans filet", `\genfrac[]{0pt}{}{n}{k}`}, // le q-binôme des vrais papiers
		{"accolades sans filet", `\genfrac{\{}{\}}{0pt}{}{n}{k}`},
		{"sans délimiteur, filet", `\genfrac{}{}{}{}{a}{b}`},
		{"style affiché", `\genfrac{}{}{}0{a}{b}`},
		{"style script", `\genfrac()\z@{2}{a}{b}`},
	} {
		svg, err := r.RenderSVG(c.tex, 12)
		if err != nil {
			t.Fatalf("%s: render(%q): %v", c.nom, c.tex, err)
		}
		if n := strings.Count(svg, "<path"); n < 2 {
			t.Errorf("%s: %d tracés, want au moins le numérateur et le dénominateur", c.nom, n)
		}
	}
	// The rule is what separates \frac from \binom: an empty thickness draws one,
	// 0pt does not. The rule is a <rect>, the glyphs are <path>.
	withRule, err := r.RenderSVG(`\genfrac{}{}{}{}{n}{k}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	noRule, err := r.RenderSVG(`\genfrac{}{}{0pt}{}{n}{k}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := strings.Count(withRule, "<rect"), strings.Count(noRule, "<rect"); a <= b {
		t.Errorf("filet: %d rect avec, %d sans — l'épaisseur vide doit tracer un filet", a, b)
	}
}

// Every way a \genfrac can be malformed must come back as an error, not as a panic
// or a silently wrong fraction: the six arguments are read one by one and each read
// can fail on its own.
func TestGenfracMalformed(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct{ nom, tex string }{
		{"rien du tout", `\genfrac`},
		{"délimiteur gauche inconnu", `\genfrac{\foo}{}{}{}{a}{b}`},
		{"délimiteur droit inconnu", `\genfrac{}{\foo}{}{}{a}{b}`},
		{"accolade non fermée sur le délimiteur", `\genfrac{(`},
		{"délimiteur gauche seul", `\genfrac(`},
		{"numérateur manquant", `\genfrac{}{}{}{}`},
		{"dénominateur manquant", `\genfrac{}{}{}{}{a}`},
	} {
		if _, err := r.RenderSVG(c.tex, 12); err == nil {
			t.Errorf("%s: %q rendu sans erreur", c.nom, c.tex)
		}
	}
}

// The style argument is TeX's digit (amsmath passes \@mathstyle{#4}): 0 display,
// 1 text, 2 and 3 the two script sizes. A fraction set in text style is smaller
// than the same one set in display style, which is the whole point of \dfrac/\tfrac
// being \genfrac{}{}{}0 and \genfrac{}{}{}1 (amsmath.sty:238-239).
func TestGenfracStyles(t *testing.T) {
	r := newRenderer(t)
	var h [4]float64
	for i, sty := range []string{"0", "1", "2", "3"} {
		_, m, err := r.RenderSVGMetrics(`\genfrac{}{}{}`+sty+`{n}{k}`, 12)
		if err != nil {
			t.Fatalf("style %s: %v", sty, err)
		}
		h[i] = m.Height + m.Depth
	}
	if !(h[0] >= h[1] && h[1] > h[2]) {
		t.Errorf("hauteurs %v: display doit être au moins aussi haut que text, et text plus haut que script", h)
	}
	if h[3] > h[2] {
		t.Errorf("hauteurs %v: scriptscript ne doit pas dépasser script", h)
	}
}

// \z@ is amsmath's own spelling of zero and what \binom passes (\genfrac()\z@{},
// amsmath.sty:240). This parser has no catcodes, so it arrives as \z and @ — and
// read as \z alone it would draw a rule where the binomial wants none.
func TestGenfracZeroThicknessSpellings(t *testing.T) {
	r := newRenderer(t)
	ruled, err := r.RenderSVG(`\genfrac{}{}{}{}{n}{k}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	for _, zero := range []string{`{0pt}`, `{0}`, `{0.0pt}`, `\z@`} {
		svg, err := r.RenderSVG(`\genfrac{}{}`+zero+`{}{n}{k}`, 12)
		if err != nil {
			t.Fatalf("%s: %v", zero, err)
		}
		if a, b := strings.Count(svg, "<rect"), strings.Count(ruled, "<rect"); a >= b {
			t.Errorf("épaisseur %s: %d rect, autant que la version filetée (%d)", zero, a, b)
		}
	}
}

// readArgText reads a braced group's text, nested braces and all, and gives back
// what it has if the group is never closed.
func TestReadArgText(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`{0pt}`, "0pt"},
		{`{{0pt}}`, "0pt"},
		{`{0pt`, "0pt"},
		{`0`, "0"},
	} {
		got, _ := readArgText(tokenize(c.src))
		if got != c.want {
			t.Errorf("readArgText(%q) = %q, want %q", c.src, got, c.want)
		}
	}
	if got, rest := readArgText(nil); got != "" || rest != nil {
		t.Errorf("readArgText(nil) = %q, %v; want \"\", nil", got, rest)
	}
}

// A fraction one-style-down: TeX (Appendix G, rule 15b) sets a TEXT-style
// fraction's numerator and denominator in SCRIPT style, and only a display-style
// one keeps them at text size. Setting both at text size made an inline fraction
// nearly as tall as a displayed one — 16.3 pt against 18.3 at 11pt — which no
// longer fits inside a host engine's baseline distance: go-tex/engine then fell
// back to \lineskip and set every line carrying an inline fraction 17.3 pt apart
// instead of 13.6, where real LaTeX keeps 13.6.
func TestInlineFractionIsShorterThanDisplay(t *testing.T) {
	r := newRenderer(t)
	_, inl, err := r.RenderSVGMetrics(`\frac{a}{b}`, 11)
	if err != nil {
		t.Fatal(err)
	}
	_, dis, err := r.RenderDisplaySVGMetrics(`\frac{a}{b}`, 11)
	if err != nil {
		t.Fatal(err)
	}
	hi, hd := inl.Height+inl.Depth, dis.Height+dis.Depth
	if hi >= hd {
		t.Errorf("hauteurs %.3f (inline) et %.3f (display) : une fraction en ligne doit être PLUS COURTE", hi, hd)
	}
	// The point of the rule: it fits on a text line. A 11pt line is 13.6 pt of
	// baseline distance, so a taller box forces the host engine onto \lineskip.
	if hi > 13.6 {
		t.Errorf("hauteur en ligne %.3f pt : ne tient pas dans un interligne de 13,6 pt", hi)
	}
	// The parts shrink, so the fraction is narrower too — the width is what showed
	// the sizes were identical before.
	if inl.Width >= dis.Width {
		t.Errorf("largeurs %.3f (inline) et %.3f (display) : les parties doivent rétrécir", inl.Width, dis.Width)
	}
}
