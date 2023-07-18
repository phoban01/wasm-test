package resource

import (
	"context"
	"encoding/json"
	"log"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/phoban01/test/internal/hostfuncs/registry"
	"github.com/phoban01/test/internal/hostfuncs/types"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	registry.Register("get_resource_labels", getResourceLabels)
}

func getResourceLabels(cv ocm.ComponentVersionAccess) types.HostFunc {
	return func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()
		data, ok := mem.Read(offset, size)
		if !ok {
			log.Fatal("could not read input")
		}

		malloc := m.ExportedFunction("malloc")
		free := m.ExportedFunction("free")

		res, err := cv.GetResource(ocmmetav1.NewIdentity(string(data)))
		if err != nil {
			log.Fatal(err)
		}

		labels := res.Meta().Labels

		labelData, err := json.Marshal(labels)
		if err != nil {
			log.Fatal(err)
		}

		labelSize := uint64(len(labelData))

		results, err := malloc.Call(ctx, labelSize)
		if err != nil {
			log.Fatal(err)
		}

		labelPointer := results[0]
		defer free.Call(ctx, labelPointer)

		if !mem.Write(uint32(labelPointer), labelData) {
			log.Fatal("could not write ref")
		}

		return labelPointer<<32 | labelSize
	}
}
