package client

// VMDiskResource identifies the VMDisk application kind.
func VMDiskResource() Resource {
	return Resource{Resource: "vmdisks", Kind: "VMDisk"}
}
