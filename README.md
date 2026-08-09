# math — go-tex

[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests)

**A pure-Go (no cgo) TeX math-mode typesetter → SVG.** It parses a subset of TeX
math syntax and lays it out to a self-contained `<svg>` using the OpenType
**MATH** table — via [go-opentype](https://github.com/go-opentype/opentype) for
MATH metrics and vector glyph outlines — with **no TeX engine, no server, and no
cgo**. It compiles to `GOOS=js/GOARCH=wasm`, so it renders math **client-side and
offline** in a browser Web Worker.

This is the math-mode component of the [go-tex](https://github.com/go-tex)
pure-Go TeX effort; the full engine will live in `go-tex/tex`.

## What it does

Typesets, using real OpenType MATH metrics (axis height, script shifts,
fraction rule thickness, script scale-down, …):

- **variables** as Unicode math-italic; **atom-class spacing** (TeX Appendix G:
  thin/medium/thick around operators, relations, punctuation);
- **superscripts** (`^`), **subscripts** (`_`), and **primes** (`'`), with
  big-operator **limits** set above/below in display style;
- **fractions** (`\frac`, `\dfrac`, `\tfrac`) with the rule on the math axis;
- **radicals** — `\sqrt{…}` and `\sqrt[n]{…}` with a stretchy radical sign;
- **stretchy delimiters** — `\left( … \right)`, `[ ]`, `\{ \}`, `\langle`,
  `\lfloor`, `|`, and null `.`, grown to the content via MATH size variants;
- **matrices** — `matrix`, `pmatrix`, `bmatrix`, `Bmatrix`, `vmatrix`,
  `Vmatrix`, and `cases`;
- **accents** — `\hat \bar \vec \tilde \dot \ddot \check \breve \acute \grave`
  — plus `\overline`/`\underline`;
- **math alphabets** — `\mathbb \mathcal \mathfrak \mathbf \mathsf \mathtt
  \mathit \mathrm` (via Unicode math alphanumerics, hole-corrected) and `\text`;
- **~200 named symbols** — Greek, big operators, binary operators, relations,
  arrows, set/logic; **spacing** (`\, \: \; \! \quad \qquad`); and explicit
  **`\displaystyle`/`\textstyle`**.

```
\sum_{i=1}^{n} i^2      \sqrt[3]{\frac{a}{b}}      \left(\frac{x+1}{2}\right)^n
\begin{pmatrix} a & b \\ c & d \end{pmatrix}      \mathbb{R} \subset \mathbb{C}
```

Each renders to a crisp, resolution-independent SVG of positioned glyph paths
(with `<rect>` rules for fractions, radicals and bars). Use `RenderDisplaySVG`
for display style (larger operators, limits above/below).

## Install

```sh
go get github.com/go-tex/math
```

## Usage

```go
package main

import (
	"fmt"

	texmath "github.com/go-tex/math"
)

func main() {
	// DefaultFont returns an embedded MATH font (STIX Two Math, OFL).
	r, err := texmath.New(texmath.DefaultFont())
	if err != nil {
		panic(err)
	}
	svg, err := r.RenderSVG(`\frac{x^2+1}{\alpha-\beta}`, 40) // 40px base size
	if err != nil {
		panic(err)
	}
	fmt.Println(svg) // <svg …>…</svg>
}
```

`New` accepts any OpenType font that carries a MATH table (STIX Two Math, Latin
Modern Math, XITS Math, …); `DefaultFont` embeds STIX Two Math so the zero-config
path — and the wasm worker — is self-contained.

## WebAssembly

Being pure Go (CGO=0), it compiles to `GOOS=js GOARCH=wasm`. `cmd/wasm` is a
worker that exposes `globalThis.renderMathSVG(tex)`:

```sh
GOOS=js GOARCH=wasm go build -o texmath.wasm ./cmd/wasm
```

Measured (STIX Two Math embedded): ~1.6 MB gzip worker (most of it the font,
which is subsettable), ~0.1–0.3 ms per formula — fast enough to re-render on
every keystroke.

## Scope — what this is, and is *not*

This is a **math-mode typesetter for preview**, not a TeX engine and **not a
replacement for a TeX distribution (TeXLive)**. It renders TeX *math* to SVG; it
does not process LaTeX *documents* — there is no macro expander, no page/line
breaking, no packages, no fonts-beyond-the-one-you-pass, and no PDF/DVI output.
Full LaTeX is the separate, staged [`go-tex/tex`](https://github.com/go-tex)
engine effort.

Within math mode it is broad (see the feature list above). Known limitations:
stretchy glyphs use MATH *size variants* only (not yet the *assembly* recipe, so
delimiters taller than the largest variant stop growing); accent positioning is
approximate; and there is no `\substack`, alignment (`aligned`/`&`-columns
beyond matrices), or user-defined macros. The symbol table is easy to extend.

## Tests

Statement coverage is held at **100%** (parser, layout, and error paths), `go
vet` clean, and green across the six 64-bit Go targets plus `js/wasm` and
`wasip1/wasm`.

```sh
go test ./...
```

## Fonts

`DefaultFont` embeds **STIX Two Math**, licensed under the SIL Open Font License
1.1 — see [STIXTwoMath-OFL.txt](STIXTwoMath-OFL.txt). The test-only
`testdata/nomath.otf` is Source Serif 4 (also OFL).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-tex/math authors.
(The embedded font is under its own OFL license, above.)
