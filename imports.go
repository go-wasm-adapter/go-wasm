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
*/
import "C"
import (
	"fmt"
	"log"
	"unsafe"

	"github.com/wasmerio/go-ext-wasm/wasmer"
)

//export debug
func debug(ctx unsafe.Pointer, a int32) {
	log.Println(a)
}

//export wexit
func wexit(ctx unsafe.Pointer, a int32) {
	fmt.Println("wasm exit")
}

//export wwrite
func wwrite(ctx unsafe.Pointer, a int32) {
	fmt.Println("wasm write")
}

//export nanotime
func nanotime(ctx unsafe.Pointer, a int32) {
	fmt.Println("nano time")
}

//export walltime
func walltime(ctx unsafe.Pointer, a int32) {
	fmt.Println("wall time")
}

//export scheduleCallback
func scheduleCallback(ctx unsafe.Pointer, a int32) {
	fmt.Println("schedule callback")
}

//export clearScheduledCallback
func clearScheduledCallback(ctx unsafe.Pointer, a int32) {
	fmt.Println("clear scheduled callback")
}

//export getRandomData
func getRandomData(ctx unsafe.Pointer, a int32) {
	fmt.Println("getRandomData")
}

//export stringVal
func stringVal(ctx unsafe.Pointer, a int32) {
	fmt.Println("stringVal")
}

//export valueGet
func valueGet(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueGet")
}

//export valueSet
func valueSet(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueSet")
}

//export valueIndex
func valueIndex(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueIndex")
}

//export valueSetIndex
func valueSetIndex(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueSetIndex")
}

//export valueCall
func valueCall(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueCall")
}

//export valueInvoke
func valueInvoke(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueInvoke")
}

//export valueNew
func valueNew(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueNew")
}

//export valueLength
func valueLength(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueLength")
}

//export valuePrepareString
func valuePrepareString(ctx unsafe.Pointer, a int32) {
	fmt.Println("valuePrepareString")
}

//export valueLoadString
func valueLoadString(ctx unsafe.Pointer, a int32) {
	fmt.Println("valueLoadString")
}

// Imports returns wasm go specific imports
func Imports() (*wasmer.Imports, error) {
	imps := wasmer.NewImports().Namespace("go")
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
			return nil, err
		}
	}

	return imps, nil
}
