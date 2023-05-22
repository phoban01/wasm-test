package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	wapc "github.com/wapc/wapc-guest-tinygo"
)

func main() {
	wapc.RegisterFunctions(wapc.Functions{
		"handler": validator,
	})
}

func validator(payload []byte) ([]byte, error) {
	var config map[string]string
	if err := json.Unmarshal(payload, &config); err != nil {
		return nil, err
	}
	err := filepath.WalkDir("/data", func(path string, d fs.DirEntry, e error) error {
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var resource map[string]any
		if err := json.Unmarshal(data, &resource); err != nil {
			return err
		}

		m, ok := resource["metadata"].(map[string]interface{})
		if !ok {
			return errors.New("couldn't parse metadata")
		}

		labels, ok := m["labels"].(map[string]string)
		if !ok {
			return errors.New("not valid")
		}

		for k := range config {


			if _, ok := labels[k]; !ok {
				return errors.New("not valid")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
