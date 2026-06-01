package client

// KafkaResource identifies the Kafka application kind.
func KafkaResource() Resource {
	return Resource{Resource: "kafkas", Kind: "Kafka"}
}
