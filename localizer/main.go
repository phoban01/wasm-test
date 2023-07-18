package main

// #include <stdlib.h>
import "C"

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	// metav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/phoban01/test/pkg/ocm"
	"github.com/phoban01/test/pkg/wasmconfig"
)

type label struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func main() {}

//export handler
func handler() uint32 {
	resource := wasmconfig.Name()

	var labels []label
	labelData := ocm.GetResourceLabels(resource)
	if err := json.Unmarshal(labelData, &labels); err != nil {
		return 1
	}

	resourceMap := make(map[string]string)
	for _, l := range labels {
		if l.Name != "ocm.software/localize" {
			continue
		}

		var resLabels []label
		if err := json.Unmarshal(ocm.GetResourceLabels(l.Value), &resLabels); err != nil {
			return 1
		}

		var origin string
		for _, rl := range resLabels {
			if rl.Name != "ocm.software/origin" {
				continue
			}
			origin = rl.Value
		}

		if origin == "" {
			continue
		}

		resourceMap[origin] = ocm.GetResourceURL(l.Value)
	}

	if err := filepath.WalkDir("/data", func(path string, d fs.DirEntry, e error) error {
		if d.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for k, v := range resourceMap {
			data = subst(string(data), k, v)
		}

		return os.WriteFile(path, data, fs.ModeType)
	}); err != nil {
		return 2
	}

	return 0
}
func subst(data, k, v string) []byte {
	data = strings.ReplaceAll(data, k, v)
	return []byte(data)
}
