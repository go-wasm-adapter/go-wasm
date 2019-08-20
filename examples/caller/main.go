package main

import (
	"log"

	"github.com/vedhavyas/go-wasm"
)

func proxy(b *wasm.Bridge) wasm.Func {
	return func(args []interface{}) (i interface{}, e error) {
		log.Println("In Go", args)
		return b.CallFunc("addition", args)
	}
}

func main() {
	b, err := wasm.BridgeFromFile("test", "./examples/wasm/main.wasm", nil)
	if err != nil {
		log.Fatal(err)
	}

	err = b.SetFunc("proxy", proxy(b))
	if err != nil {
		panic(err)
	}

	init, done := make(chan error), make(chan bool)
	go b.Run(init, done)
	err = <-init
	if err != nil {
		panic(err)
	}

	<-done
	log.Println("wasm exited", err)
}
