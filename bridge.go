package wasm

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

var undefined = &struct{}{}
var bridges = map[string]*Bridge{}
var mu sync.RWMutex // to protect bridges
type context struct{ n string }

func getCtxData(b *Bridge) (unsafe.Pointer, error) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := bridges[b.name]; ok {
		return nil, fmt.Errorf("bridge with name %s already exists", b.name)
	}

	bridges[b.name] = b
	return unsafe.Pointer(&context{n: b.name}), nil
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
	done     chan bool
	exitCode int
	values   []interface{}
	refs     map[interface{}]int
	memory   []byte
	exited   bool
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
	b.values = []interface{}{
		math.NaN(),
		float64(0),
		nil,
		true,
		false,
		&object{
			props: map[string]interface{}{
				"Object":       propObject("Object", nil),
				"Array":        propObject("Array", nil),
				"Int8Array":    typedArray("Int8Array"),
				"Int16Array":   typedArray("Int16Array"),
				"Int32Array":   typedArray("Int32Array"),
				"Uint8Array":   typedArray("Uint8Array"),
				"Uint16Array":  typedArray("Uint16Array"),
				"Uint32Array":  typedArray("Uint32Array"),
				"Float32Array": typedArray("Float32Array"),
				"Float64Array": typedArray("Float64Array"),
				"process":      propObject("process", nil),
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
						buf := args[1].(*array).data()[offset : offset+length]
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
		propObject("mem", map[string]interface{}{
			"buffer": &buffer{data: b.mem()}},
		),
		goObj, // jsGo
	}
}

func (b *Bridge) check() {
	if b.exited {
		panic("WASM instance already exited")
	}
}

// Run start the wasm instance.
func (b *Bridge) Run(init chan error, done chan bool) {
	b.check()
	defer b.instance.Close()

	b.done = done
	run := b.instance.Exports["run"]
	_, err := run(0, 0)
	if err != nil {
		init <- err
		return
	}

	init <- nil
	<-b.done
	fmt.Printf("WASM exited with code: %v\n", b.exitCode)
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

	return b.values[b.getUint32(addr)]
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

	rv := reflect.TypeOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	ref, ok := b.refs[v]
	if !ok {
		ref = len(b.values)
		b.values = append(b.values, v)
		b.refs[v] = ref
	}

	typeFlag := 0
	switch rv.Kind() {
	case reflect.String:
		typeFlag = 1
	case reflect.Struct, reflect.Slice:
		typeFlag = 2
	case reflect.Func:
		typeFlag = 3
	default:
		panic(fmt.Sprintf("unknown type: %T", v))
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
	buf    *buffer
	offset int
	length int
}

func (a *array) data() []byte {
	return a.buf.data[a.offset : a.offset+a.length]
}
func typedArray(name string) *object {
	return &object{
		name: name,
		new: func(args []interface{}) interface{} {
			return &array{
				buf:    args[0].(*buffer),
				offset: int(args[1].(float64)),
				length: int(args[2].(float64)),
			}
		},
	}
}

type buffer struct {
	data []byte
}

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
	goObj := b.values[7].(*object)
	event := propObject("_pendingEvent", map[string]interface{}{
		"id":   id,
		"this": nil,
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
	fw, ok := b.values[5].(*object).props[fn]
	if !ok {
		return nil, fmt.Errorf("missing function: %v", fn)
	}

	return b.makeFuncWrapper(fw.(*funcWrapper).id, b.values[7], &args)
}

func (b *Bridge) SetFunc(fname string, fn Func) error {
	b.values[5].(*object).props[fname] = &fn
	return nil
}
