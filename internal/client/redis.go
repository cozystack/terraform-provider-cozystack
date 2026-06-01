package client

// RedisResource describes the apps.cozystack.io Redis resource.
func RedisResource() Resource {
	return Resource{Resource: "redises", Kind: "Redis"}
}
