package client

// MariaDBResource describes the apps.cozystack.io MariaDB resource.
func MariaDBResource() Resource {
	return Resource{Resource: "mariadbs", Kind: "MariaDB"}
}
