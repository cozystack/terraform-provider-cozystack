package client

// KubernetesResource identifies the Kubernetes application kind.
func KubernetesResource() Resource {
	return Resource{Resource: "kuberneteses", Kind: "Kubernetes"}
}

// KubernetesNodesResource identifies the KubernetesNodes application kind: a
// worker node pool attached by name to a Kubernetes cluster in the same
// namespace.
func KubernetesNodesResource() Resource {
	return Resource{Resource: "kubernetesnodeses", Kind: "KubernetesNodes"}
}
