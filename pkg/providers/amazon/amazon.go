package amazon

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/aws/aws-node-termination-handler/pkg/ec2metadata"
)
type Provider struct {
	ecrClient      *ecr.Client
	authToken      *authn.AuthConfig
	authTokenExpiry time.Time
}

func NewProvider() *Provider {
	cfg, _ := config.LoadDefaultConfig(context.TODO(), config.WithRegion(requestEC2Region()))
	ecrClient := ecr.NewFromConfig(cfg)
	return &Provider{ecrClient: ecrClient}
}

func (p *Provider) GetAuthKeychain(_ string) (authn.Keychain, error) {
	const bufferPeriod = time.Hour

	if p.authToken != nil && time.Now().Before(p.authTokenExpiry.Add(-bufferPeriod)) {
		return &customKeychain{authenticator: authn.FromConfig(*p.authToken)}, nil
	}

	authTokenOutput, err := p.ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, err
	}

	if len(authTokenOutput.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data received from ECR")
	}

	authData := authTokenOutput.AuthorizationData[0]
	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return nil, err
	}

	credentials := strings.SplitN(string(decodedToken), ":", 2)
	if len(credentials) != 2 {
		return nil, fmt.Errorf("invalid authorization token format")
	}

	p.authToken = &authn.AuthConfig{
		Username: credentials[0],
		Password: credentials[1],
	}
	p.authTokenExpiry = *authData.ExpiresAt

	return &customKeychain{authenticator: authn.FromConfig(*p.authToken)}, nil
}

 
func requestEC2Region() string {
	ec2metadataClient := ec2metadata.New("http://169.254.169.254", 1)
	metadata := ec2metadataClient.GetNodeMetadata()

	return metadata.Region
}

type customKeychain struct {
	authenticator authn.Authenticator
}

func (c *customKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	return c.authenticator, nil
}
