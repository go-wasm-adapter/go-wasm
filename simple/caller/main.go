package main

import (
	"fmt"
	"log"

	wasmgo "github.com/vedhavyas/wasm"
)

func main() {
	b, err := wasmgo.BridgeFromFile("test", "./simple/prog/main.wasm", nil)
	if err != nil {
		log.Fatal(err)
	}

	init, done := make(chan error), make(chan bool)
	go b.Run(init, done)
	err = <-init
	if err != nil {
		panic(err)
	}

	res, err := b.CallFunc("printWasm", &[]interface{}{"Hello from Go"})
	fmt.Println(res, err)
	<-done
	fmt.Println("wasm exited", err)
}
