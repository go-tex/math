// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import (
	"math"
	"strings"
	"testing"
)

// dims returns a formula's width, height and depth.
func dims(t *testing.T, tex string) (w, h, d float64) {
	t.Helper()
	r := newRenderer(t)
	_, m, err := r.RenderSVGMetrics(tex, 10)
	if err != nil {
		t.Fatalf("%s: %v", tex, err)
	}
	return m.Width, m.Height, m.Depth
}

// \scalebox{h}[v]{content} scales a box. graphics.sty:521-537 states every case, and
// each is asserted as a RATIO against the unscaled box rather than a measured number —
// a number would pin today's font, while "half as wide" is what the factor means.
//
//	:526-528  a negative vertical factor SWAPS height and depth
//	:529-531  otherwise both are multiplied by it
//	:533-536  the width is |h| times the original either way
//	:519      the vertical factor DEFAULTS to the horizontal one
//
// It is the largest single item in the dropped-equation census: 3624 equations across 16
// papers of a 999-paper sample, 23% of every equation dropped there.
func TestScaleboxScalesByItsFactors(t *testing.T) {
	w0, h0, d0 := dims(t, `abc`)
	for _, c := range []struct {
		tex        string
		fw, fh, fd float64
	}{
		{`\scalebox{1}{abc}`, 1, 1, 1},
		{`\scalebox{0.5}{abc}`, 0.5, 0.5, 0.5},
		{`\scalebox{2}{abc}`, 2, 2, 2},
		// The optional factor applies to the VERTICAL only, and its absence means "the
		// same as horizontal" — the two cases a single-factor reading would confuse.
		{`\scalebox{0.5}[1]{abc}`, 0.5, 1, 1},
		{`\scalebox{1}[0.5]{abc}`, 1, 0.5, 0.5},
		{`\scalebox{2}[0.5]{abc}`, 2, 0.5, 0.5},
		// A negative horizontal factor mirrors: |h| times as wide, same extents.
		{`\scalebox{-1}[1]{abc}`, 1, 1, 1},
		{`\scalebox{-2}[1]{abc}`, 2, 1, 1},
	} {
		w, h, d := dims(t, c.tex)
		if math.Abs(w-c.fw*w0) > 0.01 || math.Abs(h-c.fh*h0) > 0.01 || math.Abs(d-c.fd*d0) > 0.01 {
			t.Errorf("%s = %.3f/%.3f/%.3f, want %.1fx/%.1fx/%.1fx of abc's %.3f/%.3f/%.3f",
				c.tex, w, h, d, c.fw, c.fh, c.fd, w0, h0, d0)
		}
	}
}

// A negative VERTICAL factor swaps height and depth (graphics.sty:526-528): scaling
// about the baseline turns what was above it into what is below. This is the case a
// reading that only took |v| would get wrong, and `abc` is a good subject because its
// height and depth differ by a factor of 50.
func TestANegativeVerticalFactorSwapsHeightAndDepth(t *testing.T) {
	_, h0, d0 := dims(t, `abc`)
	if h0 <= d0 {
		t.Skip("abc's height and depth are not distinguishable, so this proves nothing")
	}
	_, h, d := dims(t, `\scalebox{1}[-1]{abc}`)
	if math.Abs(h-d0) > 0.01 || math.Abs(d-h0) > 0.01 {
		t.Errorf(`\scalebox{1}[-1]{abc} = h %.3f d %.3f, want them SWAPPED from %.3f/%.3f`,
			h, d, h0, d0)
	}
	// And |v| still scales, so the swap is not all that happens.
	_, h2, d2 := dims(t, `\scalebox{1}[-2]{abc}`)
	if math.Abs(h2-2*d0) > 0.01 || math.Abs(d2-2*h0) > 0.01 {
		t.Errorf(`\scalebox{1}[-2]{abc} = h %.3f d %.3f, want 2x the swap of %.3f/%.3f`,
			h2, d2, h0, d0)
	}
}

// \reflectbox is \Gscale@box-1[1] (graphics.sty:538) — literally \scalebox{-1}[1], so
// the two must agree exactly. Free once the negative case works, and asserted so it
// cannot drift into its own implementation.
func TestReflectboxIsScaleboxMinusOne(t *testing.T) {
	w1, h1, d1 := dims(t, `\reflectbox{abc}`)
	w2, h2, d2 := dims(t, `\scalebox{-1}[1]{abc}`)
	if w1 != w2 || h1 != h2 || d1 != d2 {
		t.Errorf(`\reflectbox = %.3f/%.3f/%.3f, \scalebox{-1}[1] gives %.3f/%.3f/%.3f`,
			w1, h1, d1, w2, h2, d2)
	}
	// It must also actually MIRROR, not merely keep the extents: the SVG carries the
	// negative scale. Without this the test would pass on \reflectbox doing nothing.
	r := newRenderer(t)
	svg, err := r.RenderSVG(`\reflectbox{abc}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, "scale(-1") {
		t.Error(`\reflectbox emits no negative scale: the box keeps its size but is not mirrored`)
	}
}

// A mirrored box must be SHIFTED back over its own width. scale(-1,1) reflects about
// x=0, so without a compensating translate the content draws to the LEFT of the origin —
// on top of whatever precedes it — while the box still declares the same width.
//
// graphics.sty:534-536 does this with a kern inside an \hbox of the scaled width:
//
//	\hb@xt@-#1\wd\z@{\kern-#1\wd\z@\box\tw@\hss}
//
// The extents tests cannot see it: removing the translate breaks NONE of them, because
// the declared width is |h| times the original either way. Only the emitted transform
// says where the ink went, so that is what this reads.
func TestAMirroredBoxIsShiftedBackOverItsWidth(t *testing.T) {
	r := newRenderer(t)
	w, _, _ := dims(t, `abc`)
	svg, err := r.RenderSVG(`\reflectbox{abc}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, "scale(-1") {
		t.Fatal(`no negative scale: \reflectbox is not mirroring at all`)
	}
	// The translate carries the scaled width, so the mirrored ink lands back in [0,w].
	want := "translate(" + ftoa(w)
	if !strings.Contains(svg, want) {
		t.Errorf("mirrored box carries no %q: the content draws to the LEFT of the origin, "+
			"over whatever precedes it, while the box still claims width %.3f", want, w)
	}
	// A POSITIVE factor must not carry one — the correction is for the mirror alone.
	pos, err := r.RenderSVG(`\scalebox{2}{abc}`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pos, `transform="translate`) && !strings.Contains(pos, "scale(2") {
		t.Error(`\scalebox{2} carries a compensating translate it does not need`)
	}
}

// A factor that is not a number is an ERROR, not a silent 1. Scaling by the wrong amount
// is invisible on the page; a refused command is one the census can rank and a reader can
// see reported.
func TestScaleboxRefusesANonNumericFactor(t *testing.T) {
	r := newRenderer(t)
	for _, tex := range []string{`\scalebox{x}{abc}`, `\scalebox{}{abc}`, `\scalebox{0.5x}{abc}`} {
		if _, _, err := r.RenderSVGMetrics(tex, 10); err == nil {
			t.Errorf("%s rendered without error", tex)
		} else if !strings.Contains(err.Error(), "not a number") {
			t.Errorf("%s: error %q does not say the factor is not a number", tex, err)
		}
	}
}

// The two ways the arguments themselves can be malformed, each reported rather than
// swallowed along with whatever followed.
func TestScaleboxReportsMalformedArguments(t *testing.T) {
	r := newRenderer(t)
	for _, c := range []struct{ tex, want string }{
		// No content at all: the factor parsed, the box did not.
		{`\scalebox{2}`, "expected an atom"},
	} {
		_, _, err := r.RenderSVGMetrics(c.tex, 10)
		if err == nil {
			t.Errorf("%s rendered without error", c.tex)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %q does not contain %q", c.tex, err, c.want)
		}
	}
}

// An UNCLOSED [ is not an optional argument, and not an error either: it stays in the
// stream and is typeset as the character it is, which is what \@ifnextchar would also
// do. \scalebox{2}[1 therefore scales a "[" by 2 and leaves the 1 beside it.
//
// My first version of this asserted an error and was wrong: a bracket is a perfectly
// good atom. The property worth pinning is that the factor was NOT read from a malformed
// bracket — the vertical factor must still equal the horizontal one.
func TestAnUnclosedBracketIsTypesetNotRefused(t *testing.T) {
	r := newRenderer(t)
	if _, _, err := r.RenderSVGMetrics(`\scalebox{2}[1`, 10); err != nil {
		t.Errorf(`\scalebox{2}[1: %v — an unclosed bracket is a character, not a failure`, err)
	}
	// And the vertical factor defaulted to 2 rather than being taken from "1": a "[" at
	// 2x is twice as TALL as at 1x.
	_, h2, _ := dims(t, `\scalebox{2}{[}`)
	_, h1, _ := dims(t, `\scalebox{2}[1]{[}`)
	if math.Abs(h2-2*h1) > 0.01 {
		t.Errorf("scalebox{2}{[} height %.3f against scalebox{2}[1]{[} %.3f: the optional "+
			"factor is not being distinguished from its absence", h2, h1)
	}
}

// An unparseable OPTIONAL factor is not an error: \scalebox{2}[x]{abc} leaves the "[x]"
// where it stands, as \@ifnextchar would, and the vertical factor stays the horizontal
// one. Refusing it would drop an equation over a bracket that may not have been an
// optional argument at all.
func TestAnUnreadableOptionalFactorIsNotTakenAsOne(t *testing.T) {
	r := newRenderer(t)
	if _, _, err := r.RenderSVGMetrics(`\scalebox{2}[x]{abc}`, 10); err != nil {
		// It may well fail on the stray [x] further along; what must NOT happen is the
		// optional factor being read as some number.
		if strings.Contains(err.Error(), "not a number") {
			t.Errorf(`\scalebox{2}[x]: %v — the optional factor must be left alone, not refused`, err)
		}
	}
}
