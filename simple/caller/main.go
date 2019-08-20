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

	init, done := make(chan bool), make(chan error)
	go b.Run(init, done)
	<-init
	res, err := b.CallFunc("printWasm", &[]interface{}{"success call"})
	fmt.Println(res, err)
	err = <-done
	fmt.Println("wasm exited", err)
}
