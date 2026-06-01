package client

// QdrantResource describes the apps.cozystack.io Qdrant resource.
func QdrantResource() Resource {
	return Resource{Resource: "qdrants", Kind: "Qdrant"}
}
