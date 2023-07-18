package resource

import (
	"context"
	"log"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/phoban01/test/internal/hostfuncs/registry"
	"github.com/phoban01/test/internal/hostfuncs/types"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	registry.Register("get_resource_bytes", getResourceBytes)
}

func getResourceBytes(cv ocm.ComponentVersionAccess) types.HostFunc {
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

		meth, err := res.AccessMethod()
		if err != nil {
			log.Fatal(err)
		}
		defer meth.Close()

		resourceData, err := meth.Get()
		if err != nil {
			log.Fatal(err)
		}

		resourceSize := uint64(len(resourceData))

		results, err := malloc.Call(ctx, resourceSize)
		if err != nil {
			log.Fatal(err)
		}

		resourcePointer := results[0]
		defer free.Call(ctx, resourcePointer)

		if !mem.Write(uint32(resourcePointer), resourceData) {
			log.Fatal("could not write ref")
		}

		return resourcePointer<<32 | resourceSize
	}
}
