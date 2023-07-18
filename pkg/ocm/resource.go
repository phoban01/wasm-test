package ocm

// GetResourceURL takes a resource name and returns the access location for the resource
func GetResourceURL(resource string) string {
	ptr, size := stringToPointer(resource)
	result := _getResourceURL(ptr, size)
	resultPtr := uint32(result >> 32)
	resultSize := uint32(result & 0xffffffff)
	return pointerToString(resultPtr, resultSize)
}

//go:wasmimport ocm.software get_resource_url
func _getResourceURL(ptr, size uint32) uint64

// GetResourceBytes takes a resource name and returns the access location for the resource
func GetResourceBytes(resource string) []byte {
	ptr, size := stringToPointer(resource)
	result := _getResourceBytes(ptr, size)
	resultPtr := uint32(result >> 32)
	resultSize := uint32(result & 0xffffffff)
	return []byte(pointerToString(resultPtr, resultSize))
}

//go:wasmimport ocm.software get_resource_bytes
func _getResourceBytes(ptr, size uint32) uint64

// GetResourceLabels takes a resource name and returns the access location for the resource
func GetResourceLabels(resource string) []byte {
	ptr, size := stringToPointer(resource)
	result := _getResourcelabels(ptr, size)
	resultPtr := uint32(result >> 32)
	resultSize := uint32(result & 0xffffffff)
	return []byte(pointerToString(resultPtr, resultSize))
}

//go:wasmimport ocm.software get_resource_labels
func _getResourcelabels(ptr, size uint32) uint64
