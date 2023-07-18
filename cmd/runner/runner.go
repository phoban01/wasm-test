package main

import (
	"context"
	"fmt"
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
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/download/handlers/dirtree"
	ocmreg "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ocireg"
	"github.com/phoban01/test/internal/hostfuncs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"gopkg.in/yaml.v2"
)

var resourceName = "manifests"

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

	res, err := cv.GetResource(ocmmetav1.NewIdentity(resourceName))
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
		WithArgs(resourceName, string(configBytes)).
		WithStdout(os.Stdout).
		WithFSConfig(fsConfig)

	builder := r.NewHostModuleBuilder("ocm.software")
	builder = hostfuncs.ForBuilder(builder, cv)
	if _, err := builder.Instantiate(ctx); err != nil {
		log.Fatal(err)
	}

	mod, err := r.InstantiateWithConfig(ctx, wasm, modConfig)
	if err != nil {
		log.Fatal(err)
	}

	handler := mod.ExportedFunction("handler")
	_, err = handler.Call(ctx)
	if err != nil {
		log.Fatal(err)
	}

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
			// fmt.Fprintf(io.Discard, string(result))
			fmt.Println(string(result))
		}
		return nil
	})
}
