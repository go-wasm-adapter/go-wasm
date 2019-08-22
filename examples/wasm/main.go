// +build js,wasm

package main

import (
	"log"
	"syscall/js"
)

func addition(this js.Value, args []js.Value) interface{} {
	log.Println("In WASM", args)
	a, b := args[0].Int(), args[1].Int()
	return a + b
}

func multiplier(this js.Value, args []js.Value) interface{} {
	return 10
}

func main() {
	ch := make(chan bool)

	// register functions
	js.Global().Set("addition", js.FuncOf(addition))
	js.Global().Set("multiplier", js.FuncOf(multiplier))

	res := js.Global().Get("addProxy").Invoke(1, 2)
	log.Printf("1 + 2 = %d\n", res.Int())
	<-ch
}
