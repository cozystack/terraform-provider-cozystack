package client

// HarborResource identifies the Harbor application kind.
func HarborResource() Resource {
	return Resource{Resource: "harbors", Kind: "Harbor"}
}
