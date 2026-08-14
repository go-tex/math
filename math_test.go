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
		t.Error("New(garbage) should fail")
	}
	nomath, err := os.ReadFile("testdata/nomath.otf")
	if err != nil {
		t.Fatalf("read nomath: %v", err)
	}
	if _, err := New(nomath); err == nil {
		t.Error("New(no MATH) should fail")
	}
	if newRenderer(t).font == nil {
		t.Error("nil font")
	}
}

// renderOK renders (both styles) and asserts a well-formed svg with no NaN/Inf.
func renderOK(t *testing.T, r *Renderer, tex string) string {
	t.Helper()
	for _, disp := range []bool{false, true} {
		var svg string
		var err error
		if disp {
			svg, err = r.RenderDisplaySVG(tex, 32)
		} else {
			svg, err = r.RenderSVG(tex, 32)
		}
		if err != nil {
			t.Fatalf("render(%q, display=%v): %v", tex, disp, err)
		}
		if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") {
			t.Fatalf("render(%q): malformed: %.40s", tex, svg)
		}
		if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
			t.Fatalf("render(%q): NaN/Inf", tex)
		}
	}
	svg, _ := r.RenderSVG(tex, 32)
	return svg
}

func TestFeatures(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`a + b = c`,
		`x^2 - 2x + 1 \leq 0`,
		`a_i + b^j - c_k^l`,
		`\frac{a}{b}`, `\dfrac{a}{b}`, `\tfrac{a}{b}`,
		`\sqrt{x}`, `\sqrt[3]{x+y}`,
		`\left( a \right)`, `\left[ \frac{a}{b} \right]`, `\left\{ x \right\}`,
		`\left. a \right|`, `\left\langle x \right\rangle`,
		`\hat{x} \bar{y} \vec{v} \tilde{n} \dot{a} \ddot{b} \check{c} \breve{d} \acute{e} \grave{f}`,
		`\overline{a+b}`, `\underline{xyz}`,
		`\mathbb{RCHNPQZ} \mathbb{R}`, `\mathcal{ABEFHILMR}`, `\mathfrak{CHIRZg}`,
		`\mathbf{x} \mathsf{y} \mathtt{z} \mathit{w} \mathrm{d} \text{if}`,
		`f'(x) g''(x) h'''`,
		`\sum_{i=1}^{n} i`, `\prod_k a_k`, `\int_0^1 f`, `\lim_{x\to 0} f`,
		`\begin{matrix} a & b \\ c & d \end{matrix}`,
		`\begin{pmatrix} a & b \\ c & d \end{pmatrix}`,
		`\begin{bmatrix} 1 \\ 2 \\ 3 \end{bmatrix}`,
		`\begin{vmatrix} a & b \\ c & d \end{vmatrix}`,
		`\begin{Vmatrix} a \end{Vmatrix}`, `\begin{Bmatrix} a \end{Bmatrix}`,
		`\begin{cases} 1 & x>0 \\ 0 & x\leq 0 \end{cases}`,
		`a \, b \: c \; d \! e \quad f \qquad g`,
		`\alpha\beta\gamma\Delta\Omega \infty \partial \nabla \forall \exists`,
		`x \in A \cup B, \ y \notin C \cap D`,
		`{a}`, `{}`, `\displaystyle \frac{a}{b}`, `\textstyle x`, `\scriptstyle y`,
		`\mathbb{R}^n \to \mathbb{R}^m`,
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
}

// The amsmath paired delimiters, colon-relations and extra relation/ordinary
// symbols added for real-paper coverage all resolve to a glyph and render.
func TestAddedSymbols(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\lvert x \rvert`, `\lVert v \rVert`, `\left\lvert a \right\rvert`,
		`f \colon A \to B`, `x \coloneqq y`, `a \eqqcolon b`,
		`p \nmid q`, `A \subsetneq B`, `x \leqslant y \geqslant z`,
		`u \lesssim v \gtrsim w`, `\Diamond \square \blacksquare`,
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
}

// \binom, the \big delimiter family, and \overset/\underset/\stackrel render.
func TestBinomBigOverset(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\binom{n}{k}`, `\dbinom{n}{2}`, `\tbinom{a}{b}`,
		`\bigl( x \bigr)`, `\Big[ y \Big]`, `\bigg\{ z \bigg\}`, `\Bigg| w \Bigg|`,
		`\big| a \big|`, `\bigm| b`,
		`\overset{a}{b}`, `\underset{c}{d}`, `x \stackrel{f}{=} y`,
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
	// \binom draws no rule (unlike \frac).
	if strings.Contains(mustRender(t, r, `\binom{n}{k}`), "<rect") {
		t.Error("\\binom must not draw a fraction rule")
	}
	// \bigl( is taller than a plain (.
	_, big, _ := r.RenderSVGMetrics(`\bigl(`, 40)
	_, plain, _ := r.RenderSVGMetrics(`(`, 40)
	if big.Height+big.Depth <= plain.Height+plain.Depth {
		t.Errorf("\\bigl( (%.1f) should be taller than ( (%.1f)", big.Height+big.Depth, plain.Height+plain.Depth)
	}
}

// \not overlays a negation slash on the following atom; \substack stacks its
// \\-separated lines at script size.
func TestNotAndSubstack(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`a \not= b`, `A \not\subset B`, `x \not\in S`,
		`\substack{i<n \\ i \ne j}`, `\sum_{\substack{a \\ b \\ c}} x`,
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
	// \not adds ink (the slash) over the atom without changing its advance width.
	_, plain, _ := r.RenderSVGMetrics(`=`, 40)
	_, neg, _ := r.RenderSVGMetrics(`\not=`, 40)
	if pn, nn := strings.Count(mustRender(t, r, `=`), "<path"), strings.Count(mustRender(t, r, `\not=`), "<path"); nn <= pn {
		t.Errorf("\\not= has %d paths, = has %d — the slash was not drawn", nn, pn)
	}
	if neg.Width < plain.Width*0.5 {
		t.Errorf("\\not= width %.1f collapsed vs = width %.1f", neg.Width, plain.Width)
	}
}

// \ifmmode is always true in this always-math renderer: its true branch is kept
// and the \else branch dropped. Dual text/math macros (\ensuremath) reach the math
// source this way; before, the unknown \ifmmode dropped the whole equation.
func TestIfmmodeResolves(t *testing.T) {
	r := newRenderer(t)
	// True branch is taken: the \else branch's undefined command is never seen, so
	// this renders instead of erroring.
	if _, _, err := r.RenderSVGMetrics(`\ifmmode a+b\else\nosuchcommand\fi`, 40); err != nil {
		t.Fatalf("true branch not taken (saw the \\else branch): %v", err)
	}
	// Nesting and a leading construct around the conditional both work.
	for _, tex := range []string{
		`x = \ifmmode\mathbb{R}\else R\fi`,
		`\sum_{\ifmmode i=1\else i\fi}^{n} a_i`,
		`\ifmmode\ifmmode y\fi\else z\fi`,
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
	// A missing branch: an \else branch that is the whole tail is dropped.
	if _, _, err := r.RenderSVGMetrics(`k\ifmmode+1\fi`, 40); err != nil {
		t.Fatalf("\\ifmmode…\\fi (no else) failed: %v", err)
	}
}

// The census batch: \ensuremath, \mathscr, atom-class wrappers, \hbox/\mbox,
// \boxed, \overbrace/\underbrace, \phantom, \limits, and added symbol aliases.
func TestCensusBatch(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\ensuremath{x+1}`, `\mathscr{ABH}`, `\mathbin{\star}`, `\mathrel{R}`,
		`\mathord{!}`, `\mathopen{[}x\mathclose{]}`,
		`\hbox{note}`, `\mbox{if } x>0`, `\boxed{E=mc^2}`,
		`\underbrace{a+b}_{s}`, `\overbrace{x+y}^{t}`,
		`\phantom{xy}z`, `\hphantom{x}y`, `\vphantom{X}y`,
		`A\vDash B`, `p\land q\lor r`, `\llbracket x\rrbracket`, `a\rtimes b`,
		`x\hdots y`, `\sum\limits_{i}a_i`, `\therefore x`, `a\boxplus b`, `\bigstar`,
	} {
		t.Run(tex, func(t *testing.T) { renderOK(t, r, tex) })
	}
	// \boxed draws a frame (rules); \phantom draws no ink but reserves width.
	if !strings.Contains(mustRender(t, r, `\boxed{x}`), "<rect") {
		t.Error("\\boxed drew no frame")
	}
	if strings.Contains(mustRender(t, r, `\phantom{x}`), "<path") {
		t.Error("\\phantom drew ink")
	}
	_, ph, _ := r.RenderSVGMetrics(`\phantom{MMM}`, 40)
	if ph.Width <= 0 {
		t.Error("\\phantom reserved no width")
	}
}

func TestStructure(t *testing.T) {
	r := newRenderer(t)
	// fraction and radical draw a rule.
	if !strings.Contains(mustRender(t, r, `\frac{a}{b}`), "<rect") {
		t.Error("fraction has no rule")
	}
	if !strings.Contains(mustRender(t, r, `\sqrt{x}`), "<rect") {
		t.Error("radical has no rule")
	}
	if !strings.Contains(mustRender(t, r, `\overline{x}`), "<rect") {
		t.Error("overline has no rule")
	}
	if !strings.Contains(mustRender(t, r, `\underline{x}`), "<rect") {
		t.Error("underline has no rule")
	}
	// pmatrix stretches delimiters → multiple paths.
	if n := strings.Count(mustRender(t, r, `\begin{pmatrix} a & b \\ c & d \end{pmatrix}`), "<path"); n < 6 {
		t.Errorf("pmatrix paths = %d", n)
	}
}

func mustRender(t *testing.T, r *Renderer, tex string) string {
	t.Helper()
	svg, err := r.RenderSVG(tex, 32)
	if err != nil {
		t.Fatalf("render(%q): %v", tex, err)
	}
	return svg
}

func TestErrors(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{
		`\nope`,                          // unknown command
		`a}`,                             // unexpected close brace
		`{a`,                             // missing close brace
		`&`,                              // an alignment tab outside a matrix/array is misplaced
		`\frac{a}`,                       // missing frac arg
		`x^`,                             // missing script atom
		`\sqrt[3]{`,                      // missing radical body
		`\sqrt[3`,                        // missing ]
		`\left(`,                         // \left without \right
		`\left`,                          // \left without delimiter
		`\begin`,                         // \begin without {env}
		`\begin{nope} a \end{nope}`,      // unknown env
		`\begin{matrix} a`,               // env without \end
		`\begin{matrix`,                  // unterminated \begin{
		`\begin{matrix} a \end{bmatrix}`, // mismatched \end
		`\begin{matrix} a \end`,          // \end without {env}
		`\begin{matrix} a \end{matrix`,   // unterminated \end{
		`\hat`,                           // accent without arg
		`\mathbb`,                        // alphabet without arg
		// error propagation from nested constructs
		`\frac{\nope}{b}`, `\frac{a}{\nope}`, `\sqrt{\nope}`, `\sqrt[\nope]{x}`,
		`\hat{\nope}`, `\overline{\nope}`, `\underline{\nope}`, `\mathbb{\nope}`,
		`\left(\nope\right)`, `\left(a\nope`, `\begin{matrix}\nope\end{matrix}`,
		`x^\nope`, `x_\nope`, `{\nope}`, `\left\zzz a \right)`,
		`\left( x \right\zzz`, // bad closing delimiter
	} {
		t.Run(tex, func(t *testing.T) {
			if _, err := r.RenderSVG(tex, 32); err == nil {
				t.Errorf("render(%q) should error", tex)
			}
		})
	}
}

// A script or prime with no preceding nucleus is valid TeX — it attaches to an
// empty box. This is exactly how a footnote/citation mark like $^1$ is set, which
// real document classes (amsart's \maketitle) emit; rejecting it aborted or
// derailed the whole compile.
func TestEmptyBaseScripts(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{`^1`, `_2`, `^{1}`, `_{ab}`, `'`, `^a_b`, `x + ^1`} {
		t.Run(tex, func(t *testing.T) {
			svg, err := r.RenderSVG(tex, 32)
			if err != nil {
				t.Fatalf("render(%q) errored: %v", tex, err)
			}
			if !strings.HasPrefix(svg, "<svg") {
				t.Errorf("render(%q): malformed svg", tex)
			}
		})
	}
}

func TestClampsAndDelimiters(t *testing.T) {
	r := newRenderer(t)
	// wide scripts wider than the nucleus exercise the width-max branches.
	renderOK(t, r, `x^{aaaaaa}`)
	renderOK(t, r, `x_{bbbbbb}`)
	renderOK(t, r, `I^{WWWW}_{MMMM}`)
	// display limits wider than the operator, and narrower.
	renderOK(t, r, `\sum_{kkkkkk}^{j} a`)
	renderOK(t, r, `\prod^{mmmmmm} b`)
	// control-symbol delimiters and null delimiter.
	renderOK(t, r, `\left\{ x \right\}`)
	renderOK(t, r, `\left. x \right.`)
	renderOK(t, r, `\left\lfloor x \right\rfloor`)
	// tiny size drives the scriptSize floor (s<1).
	if _, err := r.RenderSVG(`x^2_3`, 1); err != nil {
		t.Fatalf("tiny size: %v", err)
	}
	// single-item and ragged matrix rows.
	renderOK(t, r, `\begin{matrix} a \\ b & c \end{matrix}`)
	// primes combined with an explicit superscript.
	renderOK(t, r, `x'^{2}`)
	renderOK(t, r, `f''_{n}`)
	// a big operator with only a subscript / only a superscript in display.
	renderOK(t, r, `\bigcup_{i} A_i`)
	// a binary operator at the start of a list is re-classed to ordinary.
	renderOK(t, r, `-x`)
	renderOK(t, r, `= + a`)
}

func TestDefaultSizeAndEmpty(t *testing.T) {
	r := newRenderer(t)
	if svg, err := r.RenderSVG("x", 0); err != nil || !strings.HasPrefix(svg, "<svg") {
		t.Fatalf("default size: %v", err)
	}
	// empty group renders nothing.
	if svg := mustRender(t, r, `{}`); strings.Contains(svg, "<path") {
		t.Error("empty group emitted a path")
	}
	// absent glyph → empty box.
	e := &engine{font: r.font, upem: float64(r.font.UnitsPerEm())}
	if _, ok := e.glyphBox(0x10FFFF, 32, clsOrd); ok {
		t.Error("absent glyph reported ok")
	}
	if b := e.mustGlyph(0x10FFFF, 32, clsOrd); b.w != 0 || b.String() != "" {
		t.Errorf("mustGlyph(absent) = %+v", b)
	}
}

// TestRenderMetrics checks the baseline-aware API: the returned Metrics are
// positive and self-consistent with the tight SVG (baseline at y=Height, viewBox
// = Width×(Height+Depth), no padding), for both inline and display style.
func TestRenderMetrics(t *testing.T) {
	r := newRenderer(t)
	for _, disp := range []bool{false, true} {
		var svg string
		var m Metrics
		var err error
		if disp {
			svg, m, err = r.RenderDisplaySVGMetrics("x+1", 40)
		} else {
			svg, m, err = r.RenderSVGMetrics("x+1", 40)
		}
		if err != nil {
			t.Fatalf("metrics(display=%v): %v", disp, err)
		}
		if m.Width <= 0 || m.Height <= 0 || m.Depth < 0 {
			t.Fatalf("metrics(display=%v) = %+v, want positive width/height, non-neg depth", disp, m)
		}
		// The tight SVG's viewBox height is exactly Height+Depth, its width Width;
		// there is no padding (unlike RenderSVG).
		wantVB := `viewBox="0 0 ` + ftoa(m.Width) + ` ` + ftoa(m.Height+m.Depth) + `"`
		if !strings.Contains(svg, wantVB) {
			t.Errorf("display=%v: viewBox not tight to metrics; want %s in %.90s", disp, wantVB, svg)
		}
		// The content group is translated so the baseline sits at y=Height.
		wantBaseline := `transform="translate(` + ftoa(0) + `,` + ftoa(m.Height) + `)"`
		if !strings.Contains(svg, wantBaseline) {
			t.Errorf("display=%v: baseline not at y=Height; want %s", disp, wantBaseline)
		}
	}
	// A descender (y) must report positive depth; an ascender-only token (l) must
	// report more height than depth — proving the split is real, not h/2.
	_, my, _ := r.RenderSVGMetrics("y", 40)
	if my.Depth <= 0 {
		t.Errorf("'y' depth = %v, want > 0 (it has a descender)", my.Depth)
	}
	_, ml, _ := r.RenderSVGMetrics("l", 40)
	if ml.Height <= ml.Depth {
		t.Errorf("'l' height %v not greater than depth %v", ml.Height, ml.Depth)
	}
	// An error still surfaces through the metrics API.
	if _, _, err := r.RenderSVGMetrics(`\nonesuch`, 40); err == nil {
		t.Error("metrics API swallowed an unknown-command error")
	}
}

func TestAlphabets(t *testing.T) {
	// hole-mapped letters resolve to the reserved code points.
	cases := []struct {
		name string
		in   rune
		want rune
	}{
		{"mathbb", 'R', 0x211D}, {"mathbb", 'A', 0x1D538}, {"mathbb", '0', 0x1D7D8},
		{"mathcal", 'L', 0x2112}, {"mathcal", 'A', 0x1D49C},
		{"mathfrak", 'H', 0x210C}, {"mathfrak", 'a', 0x1D51E},
		{"mathbf", 'a', 0x1D41A}, {"mathsf", 'A', 0x1D5A0}, {"mathtt", 'a', 0x1D68A},
		{"mathit", 'h', 0x210E}, {"text", 'x', 'x'}, {"mathrm", 'y', 'y'},
		{"boldsymbol", 'a', 0x1D482}, {"mathbf", '5', 0x1D7D3}, {"mathbf", '+', '+'},
	}
	for _, c := range cases {
		if got := alphabetFor(c.name)(c.in); got != c.want {
			t.Errorf("%s(%q) = %U, want %U", c.name, c.in, got, c.want)
		}
	}
	if alphabetFor("bogus") != nil {
		t.Error("unknown alphabet should be nil")
	}
	// digits/punctuation pass through a block mapper unchanged when no digit base.
	if got := alphabetFor("mathcal")('7'); got != '7' {
		t.Errorf("mathcal digit = %q", got)
	}
}

func TestCharClassAndMathItalic(t *testing.T) {
	pairs := map[rune]atomClass{'+': clsBin, '=': clsRel, '(': clsOpen, ')': clsClose, ',': clsPunct, 'a': clsOrd}
	for r, c := range pairs {
		if got := charClass(r); got != c {
			t.Errorf("charClass(%q) = %d, want %d", r, got, c)
		}
	}
	if mathItalic('a') != 0x1D44E || mathItalic('A') != 0x1D434 || mathItalic('h') != 0x210E || mathItalic('1') != '1' {
		t.Error("mathItalic mapping wrong")
	}
}

func TestFtoa(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 12.0: "12", 1.25: "1.25", -0.5: "-0.5", 3.14159: "3.142"} {
		if got := ftoa(in); got != want {
			t.Errorf("ftoa(%v) = %q, want %q", in, got, want)
		}
	}
}
