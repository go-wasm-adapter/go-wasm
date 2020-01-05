package main

import (
	"context"
	"log"

	"github.com/vedhavyas/go-wasm"
)

func addProxy(b *wasm.Bridge) wasm.Func {
	return func(args []interface{}) (i interface{}, e error) {
		log.Println("In Go", args)
		return b.CallFunc("addition", args)
	}
}

func multiply(b *wasm.Bridge, a int) (int, error) {
	m, err := b.CallFunc("multiplier", nil)
	if err != nil {
		return 0, err
	}

	return a * int(m.(float64)), nil
}

func main() {
	b, err := wasm.BridgeFromFile("test", "./examples/function-wasm/main.wasm", nil)
	if err != nil {
		panic(err)
	}

	err = b.SetFunc("addProxy", addProxy(b))
	if err != nil {
		panic(err)
	}

	init := make(chan error)
	ctx, cancF := context.WithCancel(context.Background())
	defer cancF()
	go b.Run(ctx, init)
	err = <-init
	if err != nil {
		panic(err)
	}

	mul, err := multiply(b, 10)
	if err != nil {
		panic(err)
	}
	log.Printf("Multiplier: %v\n", mul)

	res, err := b.CallFunc("getBytes", nil)
	if err != nil {
		panic(err)
	}

	bytes, err := wasm.Bytes(res)
	if err != nil {
		panic(err)
	}
	log.Println(bytes)

	res, err = b.CallFunc("bytes", []interface{}{wasm.FromBytes(bytes)})
	if err != nil {
		panic(err)
	}

	nb, err := wasm.Bytes(res)
	if err != nil {
		panic(err)
	}

	log.Println(nb)
}
