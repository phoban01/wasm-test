package registry

import (
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/phoban01/test/internal/hostfuncs/types"
)

var registeredHostFuncs = make(map[string]func(ocm.ComponentVersionAccess) types.HostFunc)

// Register is called by handlers to register themselves.
func Register(name string, f func(ocm.ComponentVersionAccess) types.HostFunc) {
	registeredHostFuncs[name] = f
}

// GetHostFuncs returns the registered handlers.
func GetHostFuncs() map[string]func(ocm.ComponentVersionAccess) types.HostFunc {
	return registeredHostFuncs
}
