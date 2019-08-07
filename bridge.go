package wasm

import (
	"encoding/binary"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

func init() {
	Bridge = new(bridge)
}

// Bridge connects go wasm builds
var Bridge *bridge

type bridge struct {
	instance wasmer.Instance
	vmExit   bool
	exitCode int
}

// GoBridge returns a new bridge.
func (b *bridge) InitWASM(file string, imports *wasmer.Imports) (err error) {
	bytes, err := wasmer.ReadBytes(file)
	if err != nil {
		return err
	}

	if imports == nil {
		imports = wasmer.NewImports()
	}

	err = b.addImports(imports)
	if err != nil {
		return err
	}

	inst, err := wasmer.NewInstanceWithImports(bytes, imports)
	if err != nil {
		return err
	}

	b.instance = inst
	return nil
}

// Run start the wasm instance.
func (b *bridge) Run() error {
	defer b.instance.Close()

	run := b.instance.Exports["run"]
	resume := b.instance.Exports["resume"]
	_, err := run(0, 0)
	if err != nil {
		return err
	}

	for !b.vmExit {
		_, err = resume()
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *bridge) mem() []byte {
	return b.instance.Memory.Data()
}

func (b bridge) setInt64(offset int32, v int64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], uint64(v))
}

func (b bridge) loadSlice(addr int32) []byte {
	mem := b.mem()
	array := binary.LittleEndian.Uint64(mem[addr+0:])
	length := binary.LittleEndian.Uint64(mem[addr+8:])
	return mem[array : array+length]
}

func (b bridge) loadString(addr int32) string {
	d := b.loadSlice(addr)
	return string(d)
}
