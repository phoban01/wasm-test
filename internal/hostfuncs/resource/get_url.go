package resource

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/phoban01/test/internal/hostfuncs/registry"
	"github.com/phoban01/test/internal/hostfuncs/types"
	"github.com/tetratelabs/wazero/api"
)

func init() {
	registry.Register("get_resource_url", getResourceURL)
}

func getResourceURL(cv ocm.ComponentVersionAccess) types.HostFunc {
	return func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
		mem := m.Memory()
		data, _ := mem.Read(offset, size)

		malloc := m.ExportedFunction("malloc")
		free := m.ExportedFunction("free")

		res, err := cv.GetResource(ocmmetav1.NewIdentity(string(data)))
		if err != nil {
			log.Fatal(err)
		}

		ref, err := getReference(cv.GetContext(), res)
		if err != nil {
			log.Fatal(err)
		}

		refSize := uint64(len([]byte(ref)))

		results, err := malloc.Call(ctx, refSize)
		if err != nil {
			log.Fatal(err)
		}

		refPtr := results[0]
		defer free.Call(ctx, refPtr)

		if !mem.Write(uint32(refPtr), []byte(ref)) {
			log.Fatal("could not write ref")
		}

		return refPtr<<32 | refSize
	}
}

func getReference(octx ocm.Context, res ocm.ResourceAccess) (string, error) {
	accSpec, err := res.Access()
	if err != nil {
		return "", err
	}

	var (
		ref    string
		refErr error
	)

	for ref == "" && refErr == nil {
		switch x := accSpec.(type) {
		case *ociartifact.AccessSpec:
			ref = x.ImageReference
		case *ociblob.AccessSpec:
			ref = fmt.Sprintf("%s@%s", x.Reference, x.Digest)
		case *localblob.AccessSpec:
			if x.GlobalAccess == nil {
				refErr = errors.New("cannot determine image digest")
			} else {
				accSpec, refErr = octx.AccessSpecForSpec(x.GlobalAccess)
			}
		default:
			refErr = errors.New("cannot determine access spec type")
		}
	}

	return ref, nil
}
