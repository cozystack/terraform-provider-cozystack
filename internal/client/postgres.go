package client

// PostgresResource describes the apps.cozystack.io Postgres resource.
func PostgresResource() Resource {
	return Resource{Resource: "postgreses", Kind: "Postgres"}
}
