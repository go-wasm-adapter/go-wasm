package wasm

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"reflect"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

var (
	undefined = &struct{}{}
	bridges   = map[string]*Bridge{}
	mu        sync.RWMutex // to protect bridges
)

type bctx struct{ n string }

func getCtxData(b *Bridge) (*bctx, error) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := bridges[b.name]; ok {
		return nil, fmt.Errorf("bridge with name %s already exists", b.name)
	}

	bridges[b.name] = b
	return &bctx{n: b.name}, nil
}

func getBridge(ctx unsafe.Pointer) *Bridge {
	ictx := wasmer.IntoInstanceContext(ctx)
	c := (ictx.Data()).(*bctx)
	mu.RLock()
	defer mu.RUnlock()
	return bridges[c.n]
}

type Bridge struct {
	name     string
	instance wasmer.Instance
	exitCode int
	valueIDX int
	valueMap map[int]interface{}
	refs     map[interface{}]int
	valuesMu sync.RWMutex
	memory   []byte
	exited   bool
	cancF    context.CancelFunc
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

	ctx, err := getCtxData(b)
	if err != nil {
		return nil, err
	}

	b.instance = inst
	inst.SetContextData(ctx)
	b.addValues()
	b.refs = make(map[interface{}]int)
	b.valueIDX = 8
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
	var goObj *object
	goObj = propObject("jsGo", map[string]interface{}{
		"_makeFuncWrapper": Func(func(args []interface{}) (interface{}, error) {
			return &funcWrapper{id: args[0]}, nil
		}),
		"_pendingEvent": nil,
	})
	b.valueMap = map[int]interface{}{
		0: math.NaN(),
		1: float64(0),
		2: nil,
		3: true,
		4: false,
		5: &object{
			props: map[string]interface{}{
				"Object": &object{name: "Object", new: func(args []interface{}) interface{} {
					return &object{name: "ObjectInner", props: map[string]interface{}{}}
				}},
				"Array":      arrayObject("Array"),
				"Uint8Array": arrayObject("Uint8Array"),
				"process":    propObject("process", nil),
				"Date": &object{name: "Date", new: func(args []interface{}) interface{} {
					t := time.Now()
					return &object{name: "DateInner", props: map[string]interface{}{
						"time": t,
						"getTimezoneOffset": Func(func(args []interface{}) (interface{}, error) {
							_, offset := t.Zone()

							// make it negative and return in minutes
							offset = (offset / 60) * -1
							return offset, nil
						}),
					}}
				}},
				"crypto": propObject("crypto", map[string]interface{}{
					"getRandomValues": Func(func(args []interface{}) (interface{}, error) {
						arr := args[0].(*array)
						return rand.Read(arr.buf)
					}),
				}),
				"AbortController": &object{name: "AbortController", new: func(args []interface{}) interface{} {
					return &object{name: "AbortControllerInner", props: map[string]interface{}{
						"signal": propObject("signal", map[string]interface{}{}),
					}}
				}},
				"Headers": &object{name: "Headers", new: func(args []interface{}) interface{} {
					headers := http.Header{}
					obj := &object{name: "HeadersInner", props: map[string]interface{}{
						"headers": headers,
						"append": Func(func(args []interface{}) (interface{}, error) {
							headers.Add(args[0].(string), args[1].(string))
							return nil, nil
						}),
					}}

					return obj
				}},
				"fetch": Func(func(args []interface{}) (interface{}, error) {
					// Fixme(ved): implement fetch
					log.Fatalln(args)
					return nil, nil
				}),
				"fs": propObject("fs", map[string]interface{}{
					"constants": propObject("constants", map[string]interface{}{
						"O_WRONLY": syscall.O_WRONLY,
						"O_RDWR":   syscall.O_RDWR,
						"O_CREAT":  syscall.O_CREAT,
						"O_TRUNC":  syscall.O_TRUNC,
						"O_APPEND": syscall.O_APPEND,
						"O_EXCL":   syscall.O_EXCL,
					}),

					"write": Func(func(args []interface{}) (interface{}, error) {
						fd := int(args[0].(float64))
						offset := int(args[2].(float64))
						length := int(args[3].(float64))
						buf := args[1].(*array).buf[offset : offset+length]
						pos := args[4]
						callback := args[5].(*funcWrapper)
						var err error
						var n int
						if pos != nil {
							position := int64(pos.(float64))
							n, err = syscall.Pwrite(fd, buf, position)
						} else {
							n, err = syscall.Write(fd, buf)
						}

						if err != nil {
							return nil, err
						}

						return b.makeFuncWrapper(callback.id, goObj, &[]interface{}{nil, n})
					}),
				}),
			},
		}, //global
		6: goObj, // jsGo
	}
}

func (b *Bridge) check() {
	if b.exited {
		panic("WASM instance already exited")
	}
}

// Run start the wasm instance.
func (b *Bridge) Run(ctx context.Context, init chan error) {
	b.check()
	defer b.instance.Close()

	run := b.instance.Exports["run"]
	_, err := run(0, 0)
	if err != nil {
		init <- err
		return
	}

	ctx, cancF := context.WithCancel(ctx)
	b.cancF = cancF
	init <- nil
	select {
	case <-ctx.Done():
		log.Printf("stopping WASM[%s] instance...\n", b.name)
		b.exited = true
		return
	}
}

func (b *Bridge) mem() []byte {
	if b.memory == nil {
		b.memory = b.instance.Memory.Data()
	}

	return b.memory
}

func (b *Bridge) getSP() int32 {
	spFunc := b.instance.Exports["getsp"]
	val, err := spFunc()
	if err != nil {
		panic("failed to get sp")
	}

	return val.ToI32()
}

func (b *Bridge) setUint8(offset int32, v uint8) {
	mem := b.mem()
	mem[offset] = byte(v)
}

func (b *Bridge) setInt64(offset int32, v int64) {
	mem := b.mem()
	binary.LittleEndian.PutUint64(mem[offset:], uint64(v))
}

func (b *Bridge) setInt32(offset int32, v int32) {
	mem := b.mem()
	binary.LittleEndian.PutUint32(mem[offset:], uint32(v))
}

func (b *Bridge) getInt64(offset int32) int64 {
	mem := b.mem()
	return int64(binary.LittleEndian.Uint64(mem[offset:]))
}

func (b *Bridge) getInt32(offset int32) int32 {
	mem := b.mem()
	return int32(binary.LittleEndian.Uint32(mem[offset:]))
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
		vals[i] = b.loadValue(int32(arr + i*8))
	}

	return vals
}

func (b *Bridge) loadValue(addr int32) interface{} {
	f := b.getFloat64(addr)
	if f == 0 {
		return undefined
	}

	if !math.IsNaN(f) {
		return f
	}

	b.valuesMu.RLock()
	defer b.valuesMu.RUnlock()

	return b.valueMap[int(b.getUint32(addr))]
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

	rt := reflect.TypeOf(v)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	rv := v
	if !rt.Comparable() {
		// since some types like Func cant be set as key, we will use their reflect value
		// as key to insert for refs[key] so that we can avoid any duplicates
		rv = reflect.ValueOf(v)
	}

	ref, ok := b.refs[rv]
	if !ok {
		b.valuesMu.RLock()
		b.valueMap[b.valueIDX] = v
		ref = b.valueIDX
		b.refs[rv] = ref
		b.valueIDX++
		b.valuesMu.RUnlock()
	}

	typeFlag := 0
	switch rt.Kind() {
	case reflect.String:
		typeFlag = 1
	case reflect.Func:
		typeFlag = 3
	}
	b.setUint32(addr+4, uint32(nanHead|typeFlag))
	b.setUint32(addr, uint32(ref))
}

type object struct {
	name  string // for debugging
	props map[string]interface{}
	new   func(args []interface{}) interface{}
}

func propObject(name string, prop map[string]interface{}) *object {
	return &object{name: name, props: prop}
}

type array struct {
	buf []byte
}

func arrayObject(name string) *object {
	return &object{
		name: name,
		new: func(args []interface{}) interface{} {
			l := int(args[0].(float64))
			return &array{
				buf: make([]byte, l, l),
			}
		},
	}
}

// TODO make this a wrapper that takes an inner `this` js object
type Func func(args []interface{}) (interface{}, error)

func (b *Bridge) resume() error {
	res := b.instance.Exports["resume"]
	_, err := res()
	return err
}

type funcWrapper struct {
	id interface{}
}

func (b *Bridge) makeFuncWrapper(id, this interface{}, args *[]interface{}) (interface{}, error) {
	goObj := this.(*object)
	event := propObject("_pendingEvent", map[string]interface{}{
		"id":   id,
		"this": goObj,
		"args": args,
	})

	goObj.props["_pendingEvent"] = event
	err := b.resume()
	if err != nil {
		return nil, err
	}

	return event.props["result"], nil
}

func (b *Bridge) CallFunc(fn string, args []interface{}) (interface{}, error) {
	b.check()
	b.valuesMu.RLock()
	fw, ok := b.valueMap[5].(*object).props[fn]
	if !ok {
		return nil, fmt.Errorf("missing function: %v", fn)
	}
	this := b.valueMap[6]
	b.valuesMu.RUnlock()
	return b.makeFuncWrapper(fw.(*funcWrapper).id, this, &args)
}

func (b *Bridge) SetFunc(fname string, fn Func) error {
	b.valuesMu.RLock()
	defer b.valuesMu.RUnlock()
	b.valueMap[5].(*object).props[fname] = &fn
	return nil
}

func Bytes(v interface{}) ([]byte, error) {
	arr, ok := v.(*array)
	if !ok {
		return nil, fmt.Errorf("got %T instead of bytes", v)
	}

	return arr.buf, nil
}

func String(v interface{}) (string, error) {
	str, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("got %t instead of string", v)
	}

	return str, nil
}

func Error(v interface{}) (errVal, err error) {
	str, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("got %T instead of error", v)
	}

	return errors.New(str), nil
}

func FromBytes(v []byte) interface{} {
	buf := make([]byte, len(v), len(v))
	copy(buf, v)
	return &array{buf: buf}
}
