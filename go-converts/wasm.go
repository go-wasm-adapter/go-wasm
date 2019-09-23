// +build js,wasm

package converts

import "syscall/js"

func ToBytes(v js.Value) []byte {
	buf := make([]byte, v.Length(), v.Length())
	for i := 0; i < v.Length(); i++ {
		sv := v.Index(i)
		buf[i] = byte(sv.Int())
	}

	return buf
}

// Free frees the value for GC
func free(v js.Value) {
	v.Call("_release_")
}
