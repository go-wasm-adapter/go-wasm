// +build js,wasm

package main

import (
	"fmt"
	"net/http"
	"syscall/js"
)

func call(this js.Value, args []js.Value) interface{} {
	res, err := http.Get(args[0].String())
	if err != nil {
		panic(err)
	}

	f := fmt.Sprintln(res.Status, res.StatusCode, res.ContentLength)
	err = res.Body.Close()
	if err != nil {
		panic(err)
	}

	return f
}

func main() {
	js.Global().Set("call", js.FuncOf(call))
	select {}
}
