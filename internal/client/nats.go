package client

// NATSResource describes the apps.cozystack.io NATS resource.
func NATSResource() Resource {
	return Resource{Resource: "natses", Kind: "NATS"}
}
