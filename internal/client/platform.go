package client

// Cluster-scoped resources in the cozystack.io platform group. Unlike the
// namespaced apps.cozystack.io kinds, these describe the platform itself.

// PackageResource identifies the Package kind.
func PackageResource() Resource {
	return Resource{
		Group: "cozystack.io", Resource: "packages", Kind: "Package", ClusterScoped: true,
	}
}

// PackageSourceResource identifies the PackageSource kind.
func PackageSourceResource() Resource {
	return Resource{
		Group: "cozystack.io", Resource: "packagesources", Kind: "PackageSource", ClusterScoped: true,
	}
}

// ApplicationDefinitionResource identifies the ApplicationDefinition kind.
func ApplicationDefinitionResource() Resource {
	return Resource{
		Group: "cozystack.io", Resource: "applicationdefinitions", Kind: "ApplicationDefinition", ClusterScoped: true,
	}
}

// SchedulingClassResource identifies the SchedulingClass kind.
func SchedulingClassResource() Resource {
	return Resource{
		Group: "cozystack.io", Resource: "schedulingclasses", Kind: "SchedulingClass", ClusterScoped: true,
	}
}
