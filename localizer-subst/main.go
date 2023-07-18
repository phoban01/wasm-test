package main

// #include <stdlib.h>
import "C"

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/phoban01/test/pkg/ocm"
	"github.com/phoban01/test/pkg/wasmconfig"
)

func main() {}

//export handler
func handler() uint32 {
	config := wasmconfig.Get()
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

		data, err = subst(config["prefix"], string(data))
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
	resource string
	original string
}

func parse(prefix, data string) *tag {
	r := strings.Split(strings.Trim(data, " "), " ")
	if len(r) != 2 {
		return nil
	}
	return &tag{
		resource: strings.TrimLeft(r[1], string(prefix)),
		original: r[1],
	}
}

func subst(prefix, data string) ([]byte, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	replacements := make(map[string]string)
	for scanner.Scan() {
		word := scanner.Text()
		if strings.Contains(word, prefix) {
			artifact := parse(prefix, word)
			url := ocm.GetResourceURL(artifact.resource)
			if url == "" {
				return nil, errors.New("could not resolve resource")
			}
			replacements[artifact.original] = url
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	for k, v := range replacements {
		data = strings.ReplaceAll(data, k, v)
	}
	return []byte(data), nil
}
