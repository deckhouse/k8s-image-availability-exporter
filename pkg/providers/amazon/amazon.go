package amazon

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-node-termination-handler/pkg/ec2metadata"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sirupsen/logrus"
)

type Provider struct {
	ecrClient       *ecr.Client
	authToken       authn.AuthConfig
	authTokenExpiry time.Time
	name            string
}

func NewProvider() *Provider {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(requestEC2Region()))
	if err != nil {
		logrus.Warn("error while loading config for new aws provider ", err)
	}
	ecrClient := ecr.NewFromConfig(cfg)
	return &Provider{ecrClient: ecrClient, name: "amazon"}
}

func (p *Provider) GetAuthKeychain(_ string) (authn.Keychain, error) {
	const bufferPeriod = time.Hour

	if p.authToken.Username != "" && time.Now().Before(p.authTokenExpiry.Add(-bufferPeriod)) {
		return &customKeychain{authenticator: authn.FromConfig(p.authToken)}, nil
	}

	authTokenOutput, err := p.ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, err
	}

	if len(authTokenOutput.AuthorizationData) == 0 {
		return nil, fmt.Errorf("no authorization data received from ECR")
	}

	authData := authTokenOutput.AuthorizationData[0]

	if authData.AuthorizationToken == nil || *authData.AuthorizationToken == "" {
		return nil, fmt.Errorf("authorization token is missing or empty")
	}

	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return nil, err
	}

	credentials := strings.SplitN(string(decodedToken), ":", 2)
	if len(credentials) != 2 {
		return nil, fmt.Errorf("invalid authorization token format")
	}

	p.authToken = authn.AuthConfig{
		Username: credentials[0],
		Password: credentials[1],
	}
	p.authTokenExpiry = *authData.ExpiresAt

	return &customKeychain{authenticator: authn.FromConfig(p.authToken)}, nil
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
