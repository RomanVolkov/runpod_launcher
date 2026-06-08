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
