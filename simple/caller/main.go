package main

import "C"
import (
	"fmt"
	"log"

	wasmgo "github.com/vedhavyas/wasm"
	wasm "github.com/wasmerio/go-ext-wasm/wasmer"
)

func main() {
	// Reads the WebAssembly module as bytes.
	bytes, err := wasm.ReadBytes("./simple/prog/main.wasm")
	if err != nil {
		log.Fatal(err)
	}

	imports, err := wasmgo.Imports()
	if err != nil {
		log.Fatal(err)
	}

	// Instantiates the WebAssembly module.
	instance, err := wasm.NewInstanceWithImports(bytes, imports)
	if err != nil {
		log.Fatal(err)
	}
	defer instance.Close()

	fmt.Println(instance.Exports)
}
