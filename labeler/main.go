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
		"handler": label,
	})
}

func label(payload []byte) ([]byte, error) {
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

		_, ok = m["labels"].(map[string]string)
		if !ok {
			m["labels"] = make(map[string]string)
		}

		l := m["labels"].(map[string]string)

		for k, v := range config {
			l[k] = v
		}

		result, err := json.Marshal(resource)
		if err != nil {
			return err
		}

		return os.WriteFile(path, result, fs.ModeType)
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
