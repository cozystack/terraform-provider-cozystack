package client

// VMInstanceResource identifies the VMInstance application kind.
func VMInstanceResource() Resource {
	return Resource{Resource: "vminstances", Kind: "VMInstance"}
}
