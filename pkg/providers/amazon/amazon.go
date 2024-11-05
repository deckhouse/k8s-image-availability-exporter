package amazon

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sirupsen/logrus"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p Provider) GetAuthKeychain(registryStr string) authn.Keychain {
	ecrClient, err := awsRegionalClient(context.TODO(), parseECRDetails(registryStr))
	if err != nil {
		logrus.Panic(err)
	}

	authTokenOutput, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		logrus.Panic(err)
	}

	if len(authTokenOutput.AuthorizationData) == 0 {
		logrus.Panic("no authorization data received from ECR")
		return nil
	}

	authData := authTokenOutput.AuthorizationData[0]
	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		logrus.Panic("error decoding authorization token: %w", err)
		return nil
	}

	credentials := strings.SplitN(string(decodedToken), ":", 2)
	if len(credentials) != 2 {
		logrus.Panic("invalid authorization token format")
		return nil
	}
	authConfig := authn.AuthConfig{
		Username: credentials[0],
		Password: credentials[1],
	}
	auth := authn.FromConfig(authConfig)
	return &customKeychain{authenticator: auth}
}

func parseECRDetails(registryStr string) string {
	parts := strings.SplitN(registryStr, ".", 5)
	if len(parts) < 3 {
		return ""
	}
	return parts[3]
}

func awsRegionalClient(ctx context.Context, region string) (*ecr.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := ecr.NewFromConfig(cfg)
	return client, nil
}

type customKeychain struct {
	authenticator authn.Authenticator
}

func (kc *customKeychain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return kc.authenticator, nil
}
