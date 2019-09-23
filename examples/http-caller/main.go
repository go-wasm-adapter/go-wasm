package main

import (
	"context"
	"log"

	"github.com/vedhavyas/go-wasm"
)

func main() {
	b, err := wasm.BridgeFromFile("test", "./examples/http/main.wasm", nil)
	if err != nil {
		panic(err)
	}

	ctx, canc := context.WithCancel(context.Background())
	defer canc()
	init := make(chan error)
	go b.Run(ctx, init)
	if err := <-init; err != nil {
		panic(err)
	}

	res, err := b.CallFunc("call", []interface{}{"https://google.com"})
	if err != nil {
		panic(err)
	}

	str, err := wasm.String(res)
	if err != nil {
		panic(err)
	}

	log.Println("Result:", str)
}
