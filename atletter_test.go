// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package math

import "testing"

// TeX reads a control word as a run of category-11 letters, and LaTeX makes @ a
// letter in package and class code (\makeatletter). So \the@inst and \@ifundefined
// are ONE control sequence each — and a caller that resolves macros by name
// (go-tex/engine) can only find them if they arrive whole. Split, \the@inst was
// reported as an unknown \the, which no macro table has.
func TestControlWordsKeepTheirAt(t *testing.T) {
	for _, c := range []struct {
		src  string
		want []string
	}{
		{`\the@inst`, []string{"the@inst"}},
		{`\@ifundefined`, []string{"@ifundefined"}},
		{`\@@affmark`, []string{"@@affmark"}},
		{`\@ x`, []string{"@"}}, // écrit avec son espace: le nom ne déborde pas
		{`\alpha`, []string{"alpha"}},
		{`\{`, []string{"{"}}, // symbole de contrôle: inchangé
	} {
		var got []string
		for _, tk := range tokenize(c.src) {
			if tk.kind == tCtrl {
				got = append(got, tk.text)
			}
		}
		if len(got) != len(c.want) || (len(got) > 0 && got[0] != c.want[0]) {
			t.Errorf("tokenize(%q) donne %q, want %q", c.src, got, c.want)
		}
	}
}

// The renderer must still say which command it could not find, by its whole name:
// that string is what the caller looks up.
func TestUnknownControlWordReportsItsWholeName(t *testing.T) {
	r := newRenderer(t)
	_, err := r.RenderSVG(`x^{\the@inst }`, 12)
	if err == nil {
		t.Fatal("aucune erreur pour une commande inconnue")
	}
	if got := err.Error(); got != `texmath: unknown command \the@inst` {
		t.Errorf("erreur %q, want la commande entière", got)
	}
}
