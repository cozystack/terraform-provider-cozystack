package client

// MongoDBResource describes the apps.cozystack.io MongoDB resource.
func MongoDBResource() Resource {
	return Resource{Resource: "mongodbs", Kind: "MongoDB"}
}
