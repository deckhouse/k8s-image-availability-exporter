package amazon

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-node-termination-handler/pkg/ec2metadata"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sirupsen/logrus"
)

var (
	// ecrURLRegex extracts region from ECR registry URL
	// Format: <account-id>.dkr.ecr.<region>.amazonaws.com
	ecrURLRegex = regexp.MustCompile(`^(\d{12})\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com`)
)

type regionToken struct {
	authConfig authn.AuthConfig
	expiry     time.Time
}

type Provider struct {
	ecrClients    map[string]*ecr.Client
	authTokens    map[string]*regionToken
	clientsLock   sync.RWMutex
	tokensLock    sync.RWMutex
	clusterRegion string
	name          string
}

func NewProvider() *Provider {
	clusterRegion := requestEC2Region()
	return &Provider{
		ecrClients:    make(map[string]*ecr.Client),
		authTokens:    make(map[string]*regionToken),
		clusterRegion: clusterRegion,
		name:          "amazon",
	}
}

// extractRegionFromECRURL extracts AWS region from ECR registry URL.
// Returns region and true if successfully extracted, otherwise returns empty string and false.
// Example: "123456789012.dkr.ecr.us-west-2.amazonaws.com" -> "us-west-2", true
// Note: AWS China regions (.amazonaws.com.cn) are not supported.
func extractRegionFromECRURL(registryURL string) (string, bool) {
	// Exclude AWS China regions (.amazonaws.com.cn)
	if strings.Contains(registryURL, ".amazonaws.com.cn") {
		return "", false
	}

	matches := ecrURLRegex.FindStringSubmatch(registryURL)
	if len(matches) >= 3 {
		return matches[2], true
	}
	return "", false
}

// getOrCreateECRClient returns cached ECR client for the region or creates a new one.
func (p *Provider) getOrCreateECRClient(region string) (*ecr.Client, error) {
	p.clientsLock.RLock()
	client, exists := p.ecrClients[region]
	p.clientsLock.RUnlock()

	if exists {
		return client, nil
	}

	p.clientsLock.Lock()
	defer p.clientsLock.Unlock()

	// Double-check after acquiring write lock
	if client, exists := p.ecrClients[region]; exists {
		return client, nil
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w", region, err)
	}

	client = ecr.NewFromConfig(cfg)
	p.ecrClients[region] = client

	logrus.Debugf("Created ECR client for region: %s", region)
	return client, nil
}

func (p *Provider) GetAuthKeychain(registry string) (authn.Keychain, error) {
	const bufferPeriod = time.Hour

	// Extract region from ECR URL, fallback to cluster region
	region, ok := extractRegionFromECRURL(registry)
	if !ok {
		region = p.clusterRegion
		logrus.Debugf("Could not extract region from registry URL %s, using cluster region: %s", registry, region)
	}

	// Check cached token for this region
	p.tokensLock.RLock()
	cachedToken, exists := p.authTokens[region]
	p.tokensLock.RUnlock()

	if exists && cachedToken.authConfig.Username != "" &&
		time.Now().Before(cachedToken.expiry.Add(-bufferPeriod)) {
		return &customKeychain{authenticator: authn.FromConfig(cachedToken.authConfig)}, nil
	}

	// Get or create ECR client for the region
	ecrClient, err := p.getOrCreateECRClient(region)
	if err != nil {
		return nil, err
	}

	// Get authorization token
	authTokenOutput, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ECR authorization token for region %s: %w", region, err)
	}

	if len(authTokenOutput.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data received from ECR for region %s", region)
	}

	authData := authTokenOutput.AuthorizationData[0]

	if authData.AuthorizationToken == nil || *authData.AuthorizationToken == "" {
		return nil, fmt.Errorf("authorization token is missing or empty for region %s", region)
	}

	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode authorization token for region %s: %w", region, err)
	}

	credentials := strings.SplitN(string(decodedToken), ":", 2)
	if len(credentials) != 2 {
		return nil, fmt.Errorf("invalid authorization token format for region %s", region)
	}

	authConfig := authn.AuthConfig{
		Username: credentials[0],
		Password: credentials[1],
	}

	// Cache the token for this region
	p.tokensLock.Lock()
	p.authTokens[region] = &regionToken{
		authConfig: authConfig,
		expiry:     *authData.ExpiresAt,
	}
	p.tokensLock.Unlock()

	logrus.Debugf("Successfully obtained ECR token for region: %s", region)
	return &customKeychain{authenticator: authn.FromConfig(authConfig)}, nil
}

func requestEC2Region() string {
	ec2metadataClient := ec2metadata.New("http://169.254.169.254", 1)
	metadata := ec2metadataClient.GetNodeMetadata()

	return metadata.Region
}

type customKeychain struct {
	authenticator authn.Authenticator
}

func (kc *customKeychain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return kc.authenticator, nil
}

func (p Provider) GetName() string {
	return p.name
}
