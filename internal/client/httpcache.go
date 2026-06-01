package client

// HTTPCacheResource identifies the HTTPCache application kind.
func HTTPCacheResource() Resource {
	return Resource{Resource: "httpcaches", Kind: "HTTPCache"}
}
