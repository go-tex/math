package math

import _ "embed"

//go:embed stixtwomath.otf
var stixFont []byte

// DefaultFont returns an embedded MATH font (STIX Two Math, OFL).
func DefaultFont() []byte { return stixFont }
