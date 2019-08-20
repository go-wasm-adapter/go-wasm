// +build js,wasm

package main

import (
	"fmt"
	"syscall/js"
)

// TODO: log seems to cause an issue
func printWasm(this js.Value, v []js.Value) interface{} {
	fmt.Println("Hello from WASM", v)
	return nil
}

func main() {
	ch := make(chan bool)
	//fmt.Println("WASM Go Initialized")

	// register functions
	fun := js.FuncOf(printWasm)
	js.Global().Set("printWasm", fun)
	//fmt.Println("Done...")
	<-ch
}
