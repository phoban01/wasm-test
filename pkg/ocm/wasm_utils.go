package ocm

import (
	"unsafe"
)

// pointerToString returns a string from WebAssembly compatible numeric types
// representing its pointer and length.
func pointerToString(ptr uint32, size uint32) string {
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), size)
}

// stringToPointer returns a pointer and size pair for the given string in a way
// compatible with WebAssembly numeric types.
// The returned pointer aliases the string hence the string must be kept alive
// until ptr is no longer needed.
func stringToPointer(s string) (uint32, uint32) {
	ptr := unsafe.Pointer(unsafe.StringData(s))
	return uint32(uintptr(ptr)), uint32(len(s))
}
