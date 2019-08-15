package wasm

/*
#include <stdlib.h>

extern void debug(void *context, int32_t a);
extern void wexit(void *context, int32_t a);
extern void wwrite(void *context, int32_t a);
extern void nanotime(void *context, int32_t a);
extern void walltime(void *context, int32_t a);
extern void scheduleCallback(void *context, int32_t a);
extern void clearScheduledCallback(void *context, int32_t a);
extern void getRandomData(void *context, int32_t a);
extern void stringVal(void *context, int32_t a);
extern void valueGet(void *context, int32_t a);
extern void valueSet(void *context, int32_t a);
extern void valueIndex(void *context, int32_t a);
extern void valueSetIndex(void *context, int32_t a);
extern void valueCall(void *context, int32_t a);
extern void valueInvoke(void *context, int32_t a);
extern void valueNew(void *context, int32_t a);
extern void valueLength(void *context, int32_t a);
extern void valuePrepareString(void *context, int32_t a);
extern void valueLoadString(void *context, int32_t a);
extern void scheduleTimeoutEvent(void *context, int32_t a);
extern void clearTimeoutEvent(void *context, int32_t a);
*/
import "C"
import (
	"crypto/rand"
	"fmt"
	"log"
	"time"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

//export debug
func debug(ctx unsafe.Pointer, sp int32) {
	log.Println(sp)
}

//export wexit
func wexit(ctx unsafe.Pointer, sp int32) {
	b := getBridge(ctx)
	b.vmExit = true
	b.exitCode = int(b.getUint32(sp + 8))
}

//export wwrite
func wwrite(ctx unsafe.Pointer, sp int32) {
	log.Fatal("wasm write", sp)
}

//export nanotime
func nanotime(ctx unsafe.Pointer, sp int32) {
	n := time.Now().UnixNano()
	getBridge(ctx).setInt64(sp+8, n)
}

//export walltime
func walltime(ctx unsafe.Pointer, sp int32) {
	log.Fatal("wall time")
}

//export scheduleCallback
func scheduleCallback(ctx unsafe.Pointer, sp int32) {
	log.Fatal("schedule callback")
}

//export clearScheduledCallback
func clearScheduledCallback(ctx unsafe.Pointer, sp int32) {
	log.Fatal("clear scheduled callback")
}

//export getRandomData
func getRandomData(ctx unsafe.Pointer, sp int32) {
	s := getBridge(ctx).loadSlice(sp + 8)
	_, err := rand.Read(s)
	// TODO how to pass error?
	if err != nil {
		log.Fatal("failed: getRandomData", err)
	}
}

//export stringVal
func stringVal(ctx unsafe.Pointer, sp int32) {
	log.Fatal("stringVal")
}

//export valueGet
func valueGet(ctx unsafe.Pointer, sp int32) {
	b := getBridge(ctx)
	str := b.loadString(sp + 16)
	id, val := b.loadValue(sp + 8)
	sp = b.getSP()
	obj, ok := val.(*object)
	if !ok {
		fmt.Println("valueGet", str, id)
		b.storeValue(sp+32, val)
		return
	}

	res, ok := obj.props[str]
	if !ok {
		// TODO
		log.Fatal("missing property", val, str)
	}
	fmt.Println("valueGet", str, id, obj.name)
	b.storeValue(sp+32, res)
}

//export valueSet
func valueSet(ctx unsafe.Pointer, sp int32) {
	str := getBridge(ctx).loadString(sp + 16)
	log.Fatal("valueSet", str)
}

//export valueIndex
func valueIndex(ctx unsafe.Pointer, sp int32) {
	log.Fatal("valueIndex")
}

//export valueSetIndex
func valueSetIndex(ctx unsafe.Pointer, sp int32) {
	log.Fatal("valueSetIndex")
}

//export valueCall
func valueCall(ctx unsafe.Pointer, sp int32) {
	str := getBridge(ctx).loadString(sp + 16)
	log.Fatal("valueCall", str)
}

//export valueInvoke
func valueInvoke(ctx unsafe.Pointer, sp int32) {
	log.Fatal("valueInvoke")
}

//export valueNew
func valueNew(ctx unsafe.Pointer, sp int32) {
	b := getBridge(ctx)
	id, val := b.loadValue(sp + 8)
	args := b.loadSliceOfValues(sp + 16)
	log.Fatal("valueNew ", id, val, args)
}

//export valueLength
func valueLength(ctx unsafe.Pointer, sp int32) {
	log.Fatal("valueLength")
}

//export valuePrepareString
func valuePrepareString(ctx unsafe.Pointer, sp int32) {
	log.Fatal("valuePrepareString")
}

//export valueLoadString
func valueLoadString(ctx unsafe.Pointer, sp int32) {
	log.Fatal("valueLoadString")
}

//export scheduleTimeoutEvent
func scheduleTimeoutEvent(ctx unsafe.Pointer, sp int32) {
	log.Fatal("scheduleTimeoutEvent")
}

//export clearTimeoutEvent
func clearTimeoutEvent(ctx unsafe.Pointer, sp int32) {
	log.Fatal("clearTimeoutEvent")
}

// addImports adds go Bridge imports in "go" namespace.
func (b *Bridge) addImports(imps *wasmer.Imports) error {
	imps = imps.Namespace("go")
	var is = []struct {
		name string
		imp  interface{}
		cgo  unsafe.Pointer
	}{
		{"debug", debug, C.debug},
		{"runtime.wasmExit", wexit, C.wexit},
		{"runtime.wasmWrite", wwrite, C.wwrite},
		{"runtime.nanotime", nanotime, C.nanotime},
		{"runtime.walltime", walltime, C.walltime},
		{"runtime.scheduleCallback", scheduleCallback, C.scheduleCallback},
		{"runtime.clearScheduledCallback", clearScheduledCallback, C.clearScheduledCallback},
		{"runtime.getRandomData", getRandomData, C.getRandomData},
		{"runtime.scheduleTimeoutEvent", scheduleTimeoutEvent, C.scheduleTimeoutEvent},
		{"runtime.clearTimeoutEvent", clearTimeoutEvent, C.clearTimeoutEvent},
		{"syscall/js.stringVal", stringVal, C.stringVal},
		{"syscall/js.valueGet", valueGet, C.valueGet},
		{"syscall/js.valueSet", valueSet, C.valueSet},
		{"syscall/js.valueIndex", valueIndex, C.valueIndex},
		{"syscall/js.valueSetIndex", valueSetIndex, C.valueSetIndex},
		{"syscall/js.valueCall", valueCall, C.valueCall},
		{"syscall/js.valueInvoke", valueInvoke, C.valueInvoke},
		{"syscall/js.valueNew", valueNew, C.valueNew},
		{"syscall/js.valueLength", valueLength, C.valueLength},
		{"syscall/js.valuePrepareString", valuePrepareString, C.valuePrepareString},
		{"syscall/js.valueLoadString", valueLoadString, C.valueLoadString},
	}

	var err error
	for _, imp := range is {
		imps, err = imps.Append(imp.name, imp.imp, imp.cgo)
		if err != nil {
			return err
		}
	}

	return nil
}
