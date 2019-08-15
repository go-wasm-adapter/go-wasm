package wasm

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

var undefined = &struct{}{}
var bridges = map[string]*Bridge{}
var mu sync.RWMutex // to protect bridges
type context struct{ n string }

// TODO ensure it  wont override the another context with same name.
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
	values   []interface{}
	refs     map[interface{}]int
}

func BridgeFromBytes(name string, bytes []byte, imports *wasmer.Imports) (*Bridge, error) {
	b := new(Bridge)
	if imports == nil {
		imports = wasmer.NewImports()
	}

	b.name = name
	err := b.addImports(imports)
	if err != nil {
		return nil, err
	}

	inst, err := wasmer.NewInstanceWithImports(bytes, imports)
	if err != nil {
		return nil, err
	}

	b.instance = inst
	inst.SetContextData(setBridge(b))
	b.addValues()
	b.refs = make(map[interface{}]int)
	return b, nil
}

func BridgeFromFile(name, file string, imports *wasmer.Imports) (*Bridge, error) {
	bytes, err := wasmer.ReadBytes(file)
	if err != nil {
		return nil, err
	}

	return BridgeFromBytes(name, bytes, imports)
}

func (b *Bridge) addValues() {
	b.values = []interface{}{
		math.NaN(),
		float64(0),
		nil,
		true,
		false,
		&object{
			props: map[string]interface{}{
				"Object":       &object{name: "Object", props: map[string]interface{}{}},
				"Array":        &object{name: "Array", props: map[string]interface{}{}},
				"Int8Array":    typedArray("Int8Array"),
				"Int16Array":   typedArray("Int16Array"),
				"Int32Array":   typedArray("Int32Array"),
				"Uint8Array":   typedArray("Uint8Array"),
				"Uint16Array":  typedArray("Uint16Array"),
				"Uint32Array":  typedArray("Uint32Array"),
				"Float32Array": typedArray("Float32Array"),
				"Float64Array": typedArray("Float64Array"),
				"process":      &object{name: "process", props: map[string]interface{}{}},
				"fs": &object{name: "fs", props: map[string]interface{}{
					"constants": &object{name: "constants", props: map[string]interface{}{
						"O_WRONLY": syscall.O_WRONLY,
						"O_RDWR":   syscall.O_RDWR,
						"O_CREAT":  syscall.O_CREAT,
						"O_TRUNC":  syscall.O_TRUNC,
						"O_APPEND": syscall.O_APPEND,
						"O_EXCL":   syscall.O_EXCL,
					}},
				}},
			},
		}, //global
		&object{
			name: "mem",
			props: map[string]interface{}{
				"buffer": &arrayBuffer{data: b.mem()},
			},
		},
		&object{name: "jsGo", props: map[string]interface{}{}}, // jsGo
	}
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

func (b *Bridge) getSP() int32 {
	spFunc := b.instance.Exports["getsp"]
	val, err := spFunc()
	if err != nil {
		log.Fatal("failed to get sp", err)
	}

	return val.ToI32()
}

func (b *Bridge) setInt64(offset int32, v int64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], uint64(v))
}

func (b *Bridge) getInt64(offset int32) int64 {
	mem := b.mem()
	return int64(binary.LittleEndian.Uint64(mem[offset:]))
}

func (b *Bridge) setUint32(offset int32, v uint32) {
	mem := b.mem()
	binary.LittleEndian.PutUint32(mem[offset:], v)
}

func (b *Bridge) setUint64(offset int32, v uint64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], v)
}

func (b *Bridge) getUnit64(offset int32) uint64 {
	mem := b.mem()
	return binary.LittleEndian.Uint64(mem[offset+0:])
}

func (b *Bridge) setFloat64(offset int32, v float64) {
	uf := math.Float64bits(v)
	b.setUint64(offset, uf)
}

func (b *Bridge) getFloat64(offset int32) float64 {
	uf := b.getUnit64(offset)
	return math.Float64frombits(uf)
}

func (b *Bridge) getUint32(offset int32) uint32 {
	return binary.LittleEndian.Uint32(b.mem()[offset+0:])
}

func (b *Bridge) loadSlice(addr int32) []byte {
	mem := b.mem()
	array := binary.LittleEndian.Uint64(mem[addr+0:])
	length := binary.LittleEndian.Uint64(mem[addr+8:])
	return mem[array : array+length]
}

func (b *Bridge) loadString(addr int32) string {
	d := b.loadSlice(addr)
	return string(d)
}

func (b *Bridge) loadSliceOfValues(addr int32) []interface{} {
	arr := int(b.getInt64(addr + 0))
	arrLen := int(b.getInt64(addr + 8))
	vals := make([]interface{}, arrLen, arrLen)
	for i := 0; i < int(arrLen); i++ {
		_, vals[i] = b.loadValue(int32(arr + i*8))
	}

	return vals
}

// TODO remove id once debugging is done
func (b *Bridge) loadValue(addr int32) (uint32, interface{}) {
	f := b.getFloat64(addr)
	if f == 0 {
		return 0, undefined
	}

	if !math.IsNaN(f) {
		return 0, f
	}

	id := b.getUint32(addr)
	return id, b.values[id]
}

func (b *Bridge) storeValue(addr int32, v interface{}) {
	const nanHead = 0x7FF80000

	if i, ok := v.(int); ok {
		v = float64(i)
	}

	if i, ok := v.(uint); ok {
		v = float64(i)
	}

	if v, ok := v.(float64); ok {
		if math.IsNaN(v) {
			b.setUint32(addr+4, nanHead)
			b.setUint32(addr, 0)
			return
		}

		if v == 0 {
			b.setUint32(addr+4, nanHead)
			b.setUint32(addr, 1)
			return
		}

		b.setFloat64(addr, v)
		return
	}

	switch v {
	case undefined:
		b.setFloat64(addr, 0)
		return
	case nil:
		b.setUint32(addr+4, nanHead)
		b.setUint32(addr, 2)
		return
	case true:
		b.setUint32(addr+4, nanHead)
		b.setUint32(addr, 3)
		return
	case false:
		b.setUint32(addr+4, nanHead)
		b.setUint32(addr, 4)
		return
	}

	ref, ok := b.refs[v]
	if !ok {
		ref = len(b.values)
		b.values = append(b.values, v)
		b.refs[v] = ref
	}

	typeFlag := 0
	switch v.(type) {
	case string:
		typeFlag = 1
	case *object, *arrayBuffer: //TODO symbol maybe?
		typeFlag = 2
	default:
		log.Fatalf("unknown type: %T", v)
		// TODO function
	}
	b.setUint32(addr+4, uint32(nanHead|typeFlag))
	b.setUint32(addr, uint32(ref))
}

type object struct {
	name  string // TODO for debugging
	props map[string]interface{}
}

func typedArray(name string) *object {
	return &object{
		name:  name,
		props: map[string]interface{}{},
	}
}

type arrayBuffer struct {
	data []byte
}
