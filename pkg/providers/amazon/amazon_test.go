package amazon

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

func TestExtractRegionFromECRURL(t *testing.T) {
	tests := []struct {
		name           string
		registryURL    string
		expectedRegion string
		expectedOK     bool
	}{
		{
			name:           "valid ECR URL us-east-1",
			registryURL:    "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			expectedRegion: "us-east-1",
			expectedOK:     true,
		},
		{
			name:           "valid ECR URL eu-west-3",
			registryURL:    "123456789012.dkr.ecr.eu-west-3.amazonaws.com",
			expectedRegion: "eu-west-3",
			expectedOK:     true,
		},
		{
			name:           "valid ECR URL ap-southeast-1",
			registryURL:    "987654321098.dkr.ecr.ap-southeast-1.amazonaws.com",
			expectedRegion: "ap-southeast-1",
			expectedOK:     true,
		},
		{
			name:           "valid ECR URL with path",
			registryURL:    "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-repo",
			expectedRegion: "us-west-2",
			expectedOK:     true,
		},
		{
			name:           "invalid URL - not ECR",
			registryURL:    "docker.io/library/nginx",
			expectedRegion: "",
			expectedOK:     false,
		},
		{
			name:           "invalid URL - wrong format",
			registryURL:    "ecr.us-east-1.amazonaws.com",
			expectedRegion: "",
			expectedOK:     false,
		},
		{
			name:           "invalid URL - malformed account ID",
			registryURL:    "12345.dkr.ecr.us-east-1.amazonaws.com",
			expectedRegion: "",
			expectedOK:     false,
		},
		{
			name:           "empty string",
			registryURL:    "",
			expectedRegion: "",
			expectedOK:     false,
		},
		{
			name:           "valid ECR URL cn-north-1",
			registryURL:    "123456789012.dkr.ecr.cn-north-1.amazonaws.com.cn",
			expectedRegion: "",
			expectedOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			region, ok := extractRegionFromECRURL(tt.registryURL)
			if ok != tt.expectedOK {
				t.Errorf("extractRegionFromECRURL(%q) ok = %v, want %v", tt.registryURL, ok, tt.expectedOK)
			}
			if region != tt.expectedRegion {
				t.Errorf("extractRegionFromECRURL(%q) region = %q, want %q", tt.registryURL, region, tt.expectedRegion)
			}
		})
	}
}

func TestNewProvider(t *testing.T) {
	// This test only verifies that NewProvider creates a provider with initialized maps
	// It won't actually call AWS APIs in unit tests
	provider := &Provider{
		ecrClients:    make(map[string]*ecr.Client),
		authTokens:    make(map[string]*regionToken),
		clusterRegion: "us-east-1",
		name:          "amazon",
	}

	if provider.name != "amazon" {
		t.Errorf("Provider.name = %q, want %q", provider.name, "amazon")
	}

	if provider.ecrClients == nil {
		t.Error("Provider.ecrClients should be initialized")
	}

	if provider.authTokens == nil {
		t.Error("Provider.authTokens should be initialized")
	}

	if provider.clusterRegion == "" {
		t.Error("Provider.clusterRegion should not be empty")
	}
}