// +build js,wasm

package converts

import "syscall/js"

func ToBytes(v js.Value) []byte {
	buf := make([]byte, v.Length(), v.Length())
	for i := 0; i < v.Length(); i++ {
		buf[i] = byte(v.Index(i).Int())
	}

	return buf
}
