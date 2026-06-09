package graphql

// GraphQLInput represents a GraphQL query or mutation request
type GraphQLInput struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// GPUTypeInfo represents GPU information from the gpuTypes query
type GPUTypeInfo struct {
	ID            string `json:"id"`
	DisplayName   string `json:"displayName"`
	MemoryInGb    int    `json:"memoryInGb"`
	SecureCloud   bool   `json:"secureCloud"`
	CommunityCloud bool  `json:"communityCloud"`
}

// GPUPriceInfo represents GPU pricing information
type GPUPriceInfo struct {
	GPUName              string  `json:"gpuName"`
	GPUTypeID            string  `json:"gpuTypeId"`
	MinimumBidPrice      float64 `json:"minimumBidPrice"`
	UninterruptablePrice float64 `json:"uninterruptablePrice"`
	MinMemory            int     `json:"minMemory"`
	MinVCPU              int     `json:"minVcpu"`
}

// GPULowestPriceInput represents filter input for lowest price query
type GPULowestPriceInput struct {
	GpuCount      int   `json:"gpuCount"`
	MinMemoryInGb int   `json:"minMemoryInGb,omitempty"`
	MinVcpuCount  int   `json:"minVcpuCount,omitempty"`
	SecureCloud   *bool `json:"secureCloud,omitempty"`
}

// PodEnvVar represents an environment variable for pod creation
type PodEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PodFindAndDeployInput represents the input for pod creation mutation
type PodFindAndDeployInput struct {
	CloudType          string      `json:"cloudType,omitempty"`
	GpuTypeID          string      `json:"gpuTypeId"`
	ImageName          string      `json:"imageName"`
	Name               string      `json:"name"`
	Env                []PodEnvVar `json:"env,omitempty"`
	ContainerDiskInGb  int         `json:"containerDiskInGb"`
	VolumeInGb         int         `json:"volumeInGb,omitempty"`
	VolumeMountPath    string      `json:"volumeMountPath,omitempty"`
	Ports              string      `json:"ports,omitempty"`
	StartSsh           bool        `json:"startSsh"`
	GpuCount           int         `json:"gpuCount"`
	MinCudaVersion     string      `json:"minCudaVersion,omitempty"`
	TerminateAfter     string      `json:"terminateAfter,omitempty"`
}

// PodInfo represents pod information returned from creation
type PodInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	DesiredStatus    string `json:"desiredStatus"`
	CostPerHr        float64 `json:"costPerHr"`
	ImageName        string `json:"imageName"`
	ContainerDiskInGb int    `json:"containerDiskInGb"`
}
