package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/fluxcd/pkg/ssa"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/open-component-model/ocm/pkg/common"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociblob"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/download/handlers/dirtree"
	ocmreg "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ocireg"
	"github.com/open-component-model/ocm/pkg/runtime"
	"github.com/tetratelabs/wazero"
	"github.com/wapc/wapc-go"
	wazeroEngine "github.com/wapc/wapc-go/engines/wazero"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/scheme"
)

func main() {
	ctx := context.Background()
	octx := ocm.ForContext(ctx)

	data, err := os.ReadFile("/home/piaras/.ocmconfig")
	if err != nil {
		log.Fatal(err)
	}

	cctx := octx.ConfigContext()

	_, err = cctx.ApplyData(data, runtime.DefaultYAMLEncoding, "ocmconfig")
	if err != nil {
		log.Fatal(err)
	}

	wasm, err := os.ReadFile(os.Args[1])
	if err != nil {
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

	filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode(data, nil, nil)
		b, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		return os.WriteFile(p, b, fs.ModeType)
	})

	engine := wazeroEngine.Engine()

	module, err := engine.New(ctx, makeHost(cv, dir), wasm, &wapc.ModuleConfig{
		Logger: wapc.PrintlnLogger,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer module.Close(ctx)

	module.(*wazeroEngine.Module).WithConfig(func(config wazero.ModuleConfig) wazero.ModuleConfig {
		conf := wazero.NewFSConfig().WithDirMount(dir, "/data")
		return config.WithFSConfig(conf).WithSysWalltime()
	})

	instance, err := module.Instantiate(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer instance.Close(ctx)

	// config := map[string]string{
	//     "test":         "this-is-a-label",
	//     "another-test": "this-is-a-label",
	// }

	config := map[string]string{
		"prefix": "ocm://",
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}

	_, err = instance.Invoke(ctx, "handler", configBytes)
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
			fmt.Fprintf(io.Discard, string(result))
			fmt.Println(string(result))
		}
		return nil
	})
}

func makeHost(cv ocm.ComponentVersionAccess, dir string) func(ctx context.Context, binding, namespace, operation string, payload []byte) ([]byte, error) {
	return func(ctx context.Context, binding, namespace, operation string, payload []byte) ([]byte, error) {
		if binding != "ocm.software" {
			return nil, errors.New("unrecognised binding")
		}
		switch namespace {
		case "get":
			switch operation {
			case "resource":
				res, err := cv.GetResource(ocmmetav1.NewIdentity(string(payload)))
				if err != nil {
					return nil, err
				}

				ref, err := getReference(cv.GetContext(), res)
				if err != nil {
					return nil, err
				}

				return []byte(ref), nil
			}
		}
		return nil, errors.New("unrecognised namespace")
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
