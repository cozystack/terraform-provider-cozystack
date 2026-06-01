package client

// BucketResource describes the apps.cozystack.io Bucket resource.
func BucketResource() Resource {
	return Resource{Resource: "buckets", Kind: "Bucket"}
}
