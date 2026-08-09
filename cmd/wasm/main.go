//go:build js && wasm

package main

import (
	texmath "github.com/go-tex/math"
	"syscall/js"
)

func main() {
	r, err := texmath.New(texmath.DefaultFont())
	if err != nil {
		panic(err)
	}
	js.Global().Set("renderMathSVG", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return ""
		}
		svg, err := r.RenderSVG(args[0].String(), 40)
		if err != nil {
			return "ERR: " + err.Error()
		}
		return svg
	}))
	select {}
}
