package client

// TCPBalancerResource identifies the TCPBalancer application kind.
func TCPBalancerResource() Resource {
	return Resource{Resource: "tcpbalancers", Kind: "TCPBalancer"}
}
