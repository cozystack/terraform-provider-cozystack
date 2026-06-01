package client

// ClickHouseResource describes the apps.cozystack.io ClickHouse resource.
func ClickHouseResource() Resource {
	return Resource{Resource: "clickhouses", Kind: "ClickHouse"}
}
