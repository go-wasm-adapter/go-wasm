// +build js,wasm

package main

import (
	"fmt"
	"syscall/js"
)

func printWasm(v []js.Value) {
	fmt.Println("Hello from WASM", v)
}

func main() {
	js.Global().Set("printWasm", js.NewCallback(printWasm))
	fmt.Println("Done...")
	<-make(chan struct{})
}
