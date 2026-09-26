// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

// exAt is amsmath's \ex@ at font size p, in points: the unit smallmatrix states its
// leading in (amsmath.sty:1047, \baselineskip6\ex@ \lineskip1.5\ex@).
//
// It is NOT a fixed 1pt and NOT proportional to the size. amsgen.sty's
// \compute@ex@ walks \vfuzz down by a factor of .97 once per point of distance
// from 10pt (doubled), and adds or subtracts the shortfall from 1pt depending on
// the sign — so it is an exponential-ish interpolation that saturates at 1.5pt
// above 20pt. Transcribed rather than approximated, and in TeX's own scaled-point
// arithmetic, because the factor .97 is a 16-bit fraction and a float version
// drifts: see TestExAtMatchesTheReference, which pins all fifteen sizes measured
// off tectonic.
func exAt(p float64) float64 {
	const pt = 1 << 16 // scaled points per point
	sp := func(v float64) int64 {
		if v >= 0 {
			return int64(v*pt + 0.5)
		}
		return -int64(-v*pt + 0.5)
	}
	d := -sp(p)
	if d < -20*pt {
		return 1.5
	}
	d += 10 * pt
	d *= 2
	// \edef\@tempa{\ifdim\dimen@>\z@ -\fi}: the sign is captured BEFORE the absolute
	// value is taken, and it is what makes a size below 10pt subtract where a size
	// above adds.
	neg := d > 0
	if d < 0 {
		d = -d
	}
	d -= 10000 // "fudge factor", \@m sp
	vfuzz := int64(pt)
	for d > 0 {
		vfuzz = texScale(vfuzz, 63570) // \vfuzz=.97\vfuzz
		d -= pt
	}
	d = pt - vfuzz
	ex := int64(pt)
	if neg {
		ex -= d
	} else {
		ex += d
	}
	return float64(ex) / pt
}

// texScale is <factor><dimen> as scan_dimen computes it: the dimen times the
// decimal factor's 16-bit numerator over 2^16, truncating.
//
// .97 is 63570/65536. TeX's scan_decimal_fraction reaches that by its own halving
// algorithm over the digits "97", and round(.97*65536) reaches the same number, so
// nothing here rests on which of the two was used — the value is pinned either way
// by TestExAtMatchesTheReference.
func texScale(x, num int64) int64 {
	return x * num / (1 << 16)
}
