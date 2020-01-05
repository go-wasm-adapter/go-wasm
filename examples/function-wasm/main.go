// +build js,wasm

package main

import (
	"crypto/rand"
	"errors"
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

func getBytes(this js.Value, args []js.Value) interface{} {
	r := make([]byte, 32)
	_, err := rand.Read(r)
	if err != nil {
		panic(err)
	}

	v := js.Global().Get("Uint8Array").New(len(r))
	js.CopyBytesToJS(v, r)
	return v
}

func getError(this js.Value, args []js.Value) interface{} {
	err := errors.New("test errors")
	return err.Error()
}

func receiveSendBytes(this js.Value, args []js.Value) interface{} {
	b := args[0]
	buf := make([]byte, b.Length(), b.Length())
	js.CopyBytesToGo(buf, b)
	v := js.Global().Get("Uint8Array").New(len(buf))
	js.CopyBytesToJS(v, buf)
	return v
}

func main() {
	ch := make(chan bool)

	// register functions
	js.Global().Set("addition", js.FuncOf(addition))
	js.Global().Set("multiplier", js.FuncOf(multiplier))
	js.Global().Set("getBytes", js.FuncOf(getBytes))
	js.Global().Set("getError", js.FuncOf(getError))
	js.Global().Set("bytes", js.FuncOf(receiveSendBytes))

	res := js.Global().Get("addProxy").Invoke(1, 2)
	log.Printf("1 + 2 = %d\n", res.Int())
	<-ch
}
