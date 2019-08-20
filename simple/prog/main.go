// +build js,wasm

package main

import (
	"fmt"
	"syscall/js"
)

// TODO: log seems to cause an issue
func printWasm(this js.Value, v []js.Value) interface{} {
	fmt.Println(v[0].String())
	return "Hello from WASM"
}

func main() {
	ch := make(chan bool)
	fmt.Println("WASM-Go Initialized")

	// register functions
	fun := js.FuncOf(printWasm)
	js.Global().Set("printWasm", fun)
	<-ch
}
