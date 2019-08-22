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
	return js.TypedArrayOf(r)
}

func getError(this js.Value, args []js.Value) interface{} {
	err := errors.New("test errors")
	return err.Error()
}

func main() {
	ch := make(chan bool)

	// register functions
	js.Global().Set("addition", js.FuncOf(addition))
	js.Global().Set("multiplier", js.FuncOf(multiplier))
	js.Global().Set("getBytes", js.FuncOf(getBytes))
	js.Global().Set("getError", js.FuncOf(getError))

	res := js.Global().Get("addProxy").Invoke(1, 2)
	log.Printf("1 + 2 = %d\n", res.Int())
	<-ch
}
