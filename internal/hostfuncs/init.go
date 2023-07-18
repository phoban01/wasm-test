package hostfuncs

import (
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/phoban01/test/internal/hostfuncs/registry"
	"github.com/tetratelabs/wazero"

	_ "github.com/phoban01/test/internal/hostfuncs/resource"
)

// ForBuilder adds all registered hostfuncs to the builder.
func ForBuilder(b wazero.HostModuleBuilder, cv ocm.ComponentVersionAccess) wazero.HostModuleBuilder {
	for name, f := range registry.GetHostFuncs() {
		b = b.NewFunctionBuilder().
			WithFunc(f(cv)).
			Export(name)
	}
	return b
}
