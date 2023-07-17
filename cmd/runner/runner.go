package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/fluxcd/pkg/ssa"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/common/accessio"
	"github.com/open-component-model/ocm/pkg/contexts/credentials/repositories/dockerconfig"
	"github.com/open-component-model/ocm/pkg/contexts/oci/attrs/cacheattr"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/download/handlers/dirtree"
	ocmreg "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ocireg"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"gopkg.in/yaml.v2"
)

func main() {
	ctx := context.Background()
	octx := ocm.ForContext(ctx)

	cache, err := accessio.NewStaticBlobCache("/home/piaras/.ocm/cache")
	if err != nil {
		log.Fatal(err)
	}
	cacheattr.Set(octx, cache)

	wasm, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	spec := dockerconfig.NewRepositorySpec("~/.docker/config.json", true)

	if _, err := octx.CredentialsContext().RepositoryForSpec(spec); err != nil {
		log.Fatal(err)
	}

	repo, err := octx.RepositoryForSpec(ocmreg.NewRepositorySpec("ghcr.io/phoban01", nil))
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	cv, err := repo.LookupComponentVersion("phoban.io/test/podinfo", "1.0.0")
	if err != nil {
		log.Fatal(err)
	}
	defer cv.Close()

	res, err := cv.GetResource(ocmmetav1.NewIdentity("manifests"))
	if err != nil {
		log.Fatal(err)
	}

	dir, err := os.MkdirTemp("", "wasm-tmp-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	tmpfs, err := projectionfs.New(osfs.New(), dir)
	if err != nil {
		os.Remove(dir)
	}

	_, _, err = dirtree.New().Download(common.NewPrinter(os.Stdout), res, "", tmpfs)
	if err != nil {
		log.Fatal(err)
	}

	configBytes, err := yaml.Marshal(map[string]string{
		"prefix": "ocm://",
	})
	if err != nil {
		log.Fatal(err)
	}

	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	fsConfig := wazero.NewFSConfig().WithDirMount(dir, "/data")
	modConfig := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithFSConfig(fsConfig)

	builder := r.NewHostModuleBuilder("ocm.software").
		NewFunctionBuilder().WithFunc(
		func(ctx context.Context, m api.Module, offset, size uint32) uint64 {
			mem := m.Memory()
			data, _ := mem.Read(offset, size)

			res, err := cv.GetResource(ocmmetav1.NewIdentity(string(data)))
			if err != nil {
				log.Fatal(err)
			}

			ref, err := getReference(cv.GetContext(), res)
			if err != nil {
				log.Fatal(err)
			}

			refSize := uint64(len([]byte(ref)))
			if !mem.Write(offset+size, []byte(ref)) {
				log.Fatal("could not write ref")
			}

			return uint64(offset+size)<<32 | refSize
		}).Export("resolve")

	if _, err := builder.Instantiate(ctx); err != nil {
		log.Fatal(err)
	}

	mod, err := r.InstantiateWithConfig(ctx, wasm, modConfig)
	if err != nil {
		log.Fatal(err)
	}

	handler := mod.ExportedFunction("handler")
	malloc := mod.ExportedFunction("malloc")
	free := mod.ExportedFunction("free")

	configBytesSize := uint64(len(configBytes))
	results, err := malloc.Call(ctx, configBytesSize)
	if err != nil {
		log.Fatal(err)
	}
	configPtr := results[0]
	defer free.Call(ctx, configPtr)

	if !mod.Memory().Write(uint32(configPtr), configBytes) {
		log.Fatalf("Memory.Write(%d, %d) out of range of memory size %d",
			configPtr, configBytesSize, mod.Memory().Size())
	}

	result, err := handler.Call(ctx, configPtr, configBytesSize)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)

	filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if d.IsDir() {
			return nil
		}
		data, err := os.Open(p)
		if err != nil {
			return err
		}
		objects, err := ssa.ReadObjects(data)
		if err != nil {
			return err
		}
		for _, o := range objects {
			result, err := yaml.Marshal(o.Object)
			if err != nil {
				return err
			}
			fmt.Fprintf(io.Discard, string(result))
			// fmt.Println(string(result))
		}
		return nil
	})
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
