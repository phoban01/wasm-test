package main

import (
	"bufio"
	"bytes"
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

		// if _, ok := resource["metadata"].(map[string]interface{})["labels"]; !ok {
		//     resource["metadata"].(map[string]interface{})["labels"] = make(map[string]string)
		// }
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

type tag struct {
	resource []byte
}

func parse(prefix, data []byte) *tag {
	return &tag{
		resource: data[len(prefix):],
	}
}

func subst(prefix []byte, data []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Split(bufio.ScanWords)
	replacements := make(map[string][]byte)
	for scanner.Scan() {
		word := scanner.Bytes()
		if bytes.Contains(word, prefix) {
			artifact := parse(prefix, word)
			resolved, err := wapc.HostCall("ocm.software", "get", "resource", artifact.resource)
			if err != nil {
				return nil, err
			}
			replacements[string(word)] = resolved
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
