package amazon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func payloadHash(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}

func signingRegion(cfg RuntimeConfig) string {
	r := strings.TrimSpace(cfg.Region)
	if r != "" {
		return r
	}
	u, err := url.Parse(cfg.SPAPIBaseURL)
	if err == nil && u.Host != "" {
		return InferSigV4Region(u.Host)
	}
	return "us-east-1"
}

func awsCredentialsForSPAPI(ctx context.Context, cfg RuntimeConfig) (aws.Credentials, error) {
	region := signingRegion(cfg)
	awscfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("amazon: load aws config: %w", err)
	}
	if arn := strings.TrimSpace(cfg.RoleARN); arn != "" {
		stsClient := sts.NewFromConfig(awscfg)
		provider := stscreds.NewAssumeRoleProvider(stsClient, arn)
		awscfg.Credentials = aws.NewCredentialsCache(provider)
	}
	creds, err := awscfg.Credentials.Retrieve(ctx)
	if err != nil {
		return aws.Credentials{}, fmt.Errorf("amazon: AWS credentials unavailable for SP-API SigV4: %w", err)
	}
	if strings.TrimSpace(creds.AccessKeyID) == "" {
		return aws.Credentials{}, fmt.Errorf("amazon: AWS credentials unavailable for SP-API SigV4 (configure env/instance/task role, or role_arn + base credentials)")
	}
	return creds, nil
}

func signSPAPIRequest(ctx context.Context, req *http.Request, cfg RuntimeConfig, body []byte) error {
	creds, err := awsCredentialsForSPAPI(ctx, cfg)
	if err != nil {
		return err
	}
	hash := payloadHash(body)
	signer := v4.NewSigner()
	if err := signer.SignHTTP(ctx, creds, req, hash, "execute-api", signingRegion(cfg), time.Now()); err != nil {
		return fmt.Errorf("amazon: SigV4 sign: %w", err)
	}
	return nil
}
