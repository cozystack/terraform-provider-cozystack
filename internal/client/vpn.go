package client

// VPNResource describes the apps.cozystack.io VPN resource.
func VPNResource() Resource {
	return Resource{Resource: "vpns", Kind: "VPN"}
}
