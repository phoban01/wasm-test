package main

// #include <stdlib.h>
import "C"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unsafe"

	"gopkg.in/yaml.v2"
)

func main() {}

func resolve(resource []byte) []byte {
	ptr, size := stringToPtr(string(resource))
	result := _resolve(ptr, size)
	resultPtr := uint32(result >> 32)
	resultSize := uint32(result & 0xffffffff)
	return []byte(ptrToString(resultPtr, resultSize))
}

//go:wasmimport ocm.software resolve
func _resolve(ptr, size uint32) uint64

//export handler
func handler(configPtr, configSize uint32) uint32 {
	payload := []byte(ptrToString(configPtr, configSize))

	var config map[string]string
	if err := yaml.Unmarshal(payload, &config); err != nil {
		return 1
	}

	if _, ok := config["prefix"]; !ok {
		return 1
	}

	if err := filepath.WalkDir("/data", func(path string, d fs.DirEntry, e error) error {
		if d.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		data, err = subst([]byte(config["prefix"]), data)
		if err != nil {
			return err
		}

		return os.WriteFile(path, data, fs.ModeType)
	}); err != nil {
		return 2
	}

	return 0
}

type tag struct {
	resource []byte
	original string
}

func parse(prefix, data []byte) *tag {
	r := bytes.Trim(bytes.Split(data, []byte(`":"`))[1], `"`)[len(prefix):]
	return &tag{
		resource: r,
		original: string(prefix) + string(r),
	}
}

func subst(prefix []byte, data []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Split(commaSplitter)
	replacements := make(map[string][]byte)
	for scanner.Scan() {
		word := scanner.Bytes()
		// need to modify this to work with yaml
		// rather than json
		if bytes.Contains(word, prefix) {
			artifact := parse(prefix, word)
			fmt.Fprintln(os.Stdout, artifact.original, string(artifact.resource))
			resolved := resolve(artifact.resource)
			if resolved == nil {
				return nil, errors.New("could not resolve resource")
			}
			replacements[artifact.original] = resolved
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for k, v := range replacements {
		data = bytes.ReplaceAll(data, []byte(k), v)
	}
	return data, nil
}

func commaSplitter(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, ','); i >= 0 {
		// Found a comma, return the token and advance to the next character
		return i + 1, data[0:i], nil
	}
	if atEOF {
		// No more commas, return the remaining data as a token
		return len(data), data, nil
	}
	// Need more data, request another read
	return 0, nil, nil
}

// ptrToString returns a string from WebAssembly compatible numeric types
// representing its pointer and length.
func ptrToString(ptr uint32, size uint32) string {
	return unsafe.String((*byte)(unsafe.Pointer(uintptr(ptr))), size)
}

// stringToPtr returns a pointer and size pair for the given string in a way
// compatible with WebAssembly numeric types.
// The returned pointer aliases the string hence the string must be kept alive
// until ptr is no longer needed.
func stringToPtr(s string) (uint32, uint32) {
	ptr := unsafe.Pointer(unsafe.StringData(s))
	return uint32(uintptr(ptr)), uint32(len(s))
}

// stringToLeakedPtr returns a pointer and size pair for the given string in a way
// compatible with WebAssembly numeric types.
// The pointer is not automatically managed by TinyGo hence it must be freed by the host.
func stringToLeakedPtr(s string) (uint32, uint32) {
	size := C.ulong(len(s))
	ptr := unsafe.Pointer(C.malloc(size))
	copy(unsafe.Slice((*byte)(ptr), size), s)
	return uint32(uintptr(ptr)), uint32(size)
}
