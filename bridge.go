package wasm

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

var undefined = &struct{}{}
var bridges = map[string]*Bridge{}
var mu sync.RWMutex // to protect bridges

type context struct {
	n string
}

func setBridge(b *Bridge) unsafe.Pointer {
	mu.Lock()
	defer mu.Unlock()
	bridges[b.name] = b
	return unsafe.Pointer(&context{n: b.name})
}

func getBridge(ctx unsafe.Pointer) *Bridge {
	ictx := wasmer.IntoInstanceContext(ctx)
	c := (*context)(ictx.Data())
	mu.RLock()
	defer mu.RUnlock()
	return bridges[c.n]
}

type Bridge struct {
	name     string
	instance wasmer.Instance
	vmExit   bool
	exitCode int
}

func (b *Bridge) InitWASMBytes(name string, bytes []byte, imports *wasmer.Imports) error {
	if imports == nil {
		imports = wasmer.NewImports()
	}

	b.name = name
	err := b.addImports(imports)
	if err != nil {
		return err
	}

	inst, err := wasmer.NewInstanceWithImports(bytes, imports)
	if err != nil {
		return err
	}

	b.instance = inst
	inst.SetContextData(setBridge(b))
	return nil
}

func (b *Bridge) InitWASM(name, file string, imports *wasmer.Imports) (err error) {
	bytes, err := wasmer.ReadBytes(file)
	if err != nil {
		return err
	}

	return b.InitWASMBytes(name, bytes, imports)
}

// Run start the wasm instance.
func (b *Bridge) Run() error {
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

	fmt.Printf("WASM exited with code: %v\n", b.exitCode)
	return nil
}

func (b *Bridge) mem() []byte {
	return b.instance.Memory.Data()
}

func (b Bridge) setInt64(offset int32, v int64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], uint64(v))
}

func (b Bridge) setUint64(offset int32, v uint64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], v)
}

func (b Bridge) getUnit64(offset int32) uint64 {
	mem := b.mem()
	return binary.LittleEndian.Uint64(mem[offset+0:])
}

func (b Bridge) setFloat64(offset int32, v float64) {
	uf := math.Float64bits(v)
	b.setUint64(offset, uf)
}

func (b Bridge) getFloat64(offset int32) float64 {
	uf := b.getUnit64(offset)
	return math.Float64frombits(uf)
}

func (b Bridge) getUint32(offset int32) uint32 {
	return binary.LittleEndian.Uint32(b.mem()[offset+0:])
}

func (b Bridge) loadSlice(addr int32) []byte {
	mem := b.mem()
	array := binary.LittleEndian.Uint64(mem[addr+0:])
	length := binary.LittleEndian.Uint64(mem[addr+8:])
	return mem[array : array+length]
}

func (b Bridge) loadString(addr int32) string {
	d := b.loadSlice(addr)
	return string(d)
}

func (b Bridge) loadValue(addr int32) interface{} {
	f := b.getFloat64(addr)
	if f == 0 {
		return undefined
	}

	if !math.IsNaN(f) {
		return f
	}

	// return value instead of uint32
	return b.getUint32(addr)
}
