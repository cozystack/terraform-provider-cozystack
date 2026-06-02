package client

// KubernetesResource identifies the Kubernetes application kind.
func KubernetesResource() Resource {
	return Resource{Resource: "kuberneteses", Kind: "Kubernetes"}
}
