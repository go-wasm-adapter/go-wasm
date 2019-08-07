package main

import (
	"log"

	wasmgo "github.com/vedhavyas/wasm"
)

func main() {
	err := wasmgo.Bridge.InitWASM("./simple/prog/main.wasm", nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := wasmgo.Bridge.Run(); err != nil {
		log.Fatal(err)
	}
}
