package main

import (
	"log"

	wasmgo "github.com/vedhavyas/wasm"
)

func main() {
	b := wasmgo.Bridge{}
	err := b.InitWASM("test", "./simple/prog/main.wasm", nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := b.Run(); err != nil {
		log.Fatal(err)
	}
}
