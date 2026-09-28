// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "fmt"

// \raisebox{lift}[height][depth]{content}, from latex.ltx:11913-11935. Every clause
// there is load-bearing and the order matters:
//
//	11913  \DeclareRobustCommand\raisebox[1]{\leavevmode
//	11915    \@ifnextchar[{\@rsbox{#1}}{\@irsbox{#1}[]}}
//	11918  \long\def\@irsbox#1[#2]#3{\@begin@tempboxa\hbox{#3}%
//	11922    \setbox\@tempboxa\hbox{\raise\@tempdima\box\@tempboxa}%   RAISE first
//	11923    \ifx\\#2\\\else\ht\@tempboxa\@tempdimb\fi                 then OVERRIDE
//	11926  \long\def\@iirsbox#1[#2][#3]#4{…
//	11932    \ht\@tempboxa\@tempdimb
//	11933    \dp\@tempboxa\dimen@                                      both, two-option
//
// So: the content is boxed and shifted UP by lift (negative lowers); the width is
// never touched; and a declared height or depth REPLACES the shifted box's own rather
// than adding to it. With one optional argument only the HEIGHT is replaced — the
// depth keeps whatever the raise left it — and only the two-option form sets both.
// That asymmetry is in the source, not a simplification here.
//
// 366 equations over 15 papers of a 999-paper census (go-tex/engine#466), and 12 of
// the 15 write it in their own .tex rather than inherit it from a class, which is the
// profile that predicted \allowbreak recovering 85% of its count.
func (e *engine) raiseBox(b *box, lift float64, h, d float64, hasH, hasD bool) *box {
	out := newBox(clsOrd) // an \hbox is an Ord atom in a formula
	// The box coordinate system has Y DOWNWARD (see box), so raising by lift is a
	// NEGATIVE dy. Getting this sign wrong lowers every \raisebox in the corpus and
	// no extent test would notice, which is why a test reads the emitted transform.
	place(out, b, 0, -lift)
	out.w = b.w
	// \hbox{\raise l \box} has height h+l and depth d−l. A large lift therefore makes
	// the depth NEGATIVE — the box's bottom edge sits above the baseline — which TeX
	// permits and which is geometrically what the shift means. It is not clamped.
	out.h, out.d = b.h+lift, b.d-lift
	if hasH {
		out.h = h
	}
	if hasD {
		out.d = d
	}
	return out
}

// parseRaisebox reads \raisebox{lift}[height][depth]{content}.
//
// The lift is REQUIRED: latex.ltx declares \raisebox[1], so a missing brace group is
// an error rather than a zero shift. Reading it as zero would silently set the content
// on the baseline, which is exactly what the fallback this replaces already did.
func (e *engine) parseRaisebox(toks []token, sty style) (*box, atomClass, bool, []token, error) {
	ls, rest, err := readOpName(toks)
	if err != nil {
		return nil, 0, false, nil, fmt.Errorf(`texmath: \raisebox needs {lift}`)
	}
	lift := e.dimen(ls, sty)

	var h, d float64
	var hasH, hasD bool
	for i := 0; i < 2; i++ {
		if len(rest) == 0 || rest[0].kind != tChar || rest[0].r != '[' {
			break
		}
		s, r2, ok := readToBracket(rest[1:])
		if !ok {
			// An unclosed [ is not an optional argument: it is the character it is,
			// and the content follows it. Treating it as one would swallow the rest
			// of the formula looking for a ].
			break
		}
		// LaTeX distinguishes an EMPTY bracket from an absent one: \@irsbox is called
		// with [] when no bracket was written, and \ifx\\#2\\ then leaves the extent
		// alone. So [] must not set the extent to zero.
		if s != "" {
			if i == 0 {
				h, hasH = e.dimen(s, sty), true
			} else {
				d, hasD = e.dimen(s, sty), true
			}
		}
		rest = r2
	}

	b, r3, err := e.parseGroupArg(rest, sty)
	if err != nil {
		return nil, 0, false, nil, err
	}
	return e.raiseBox(b, lift, h, d, hasH, hasD), clsOrd, false, r3, nil
}
