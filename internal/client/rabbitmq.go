package client

// RabbitMQResource describes the apps.cozystack.io RabbitMQ resource.
func RabbitMQResource() Resource {
	return Resource{Resource: "rabbitmqs", Kind: "RabbitMQ"}
}
