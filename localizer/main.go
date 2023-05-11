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
		"handler": localize,
	})
}

func localize(payload []byte) ([]byte, error) {
	var config map[string]string
	if err := json.Unmarshal(payload, &config); err != nil {
		return nil, err
	}
	if _, ok := config["prefix"]; !ok {
		return nil, errors.New("prefix is required")
	}
	err := filepath.WalkDir("/data", func(path string, d fs.DirEntry, e error) error {
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
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
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
		if bytes.Contains(word, prefix) {
			artifact := parse(prefix, word)
			resolved, err := wapc.HostCall("ocm.software", "get", "resource", artifact.resource)
			if err != nil {
				return nil, err
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
