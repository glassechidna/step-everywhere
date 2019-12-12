package stepeverywhere

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/glassechidna/awsctx/service/kmsctx"
	"github.com/glassechidna/awsctx/service/lambdactx"
	"github.com/pkg/errors"
	"strings"
)

type Granter struct {
	kms    kmsctx.KMS
	lambda lambdactx.Lambda
	keyId  string
}

func NewGranter(kms kmsctx.KMS, lambda lambdactx.Lambda, keyId string) *Granter {
	return &Granter{kms: kms, lambda: lambda, keyId: keyId}
}

type GrantInput struct {
	ExecutionId string
	Function    string
}

type GrantOutput struct {
	GrantId    string
	GrantToken string
	Grantee    string
}

type RevokeInput struct {
	GrantId string
}

func (g *Granter) Grant(ctx context.Context, input *GrantInput) (*GrantOutput, error) {
	execIdParts := strings.SplitN(input.ExecutionId, ":", 8)
	name := "StepEverywhere-" + execIdParts[7]

	lam, err := g.lambda.GetFunctionWithContext(ctx, &lambda.GetFunctionInput{FunctionName: &input.Function})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	grant, err := g.kms.CreateGrantWithContext(ctx, &kms.CreateGrantInput{
		GranteePrincipal: lam.Configuration.Role,
		KeyId:            &g.keyId,
		Name:             &name,
		Operations:       []*string{aws.String("Decrypt")},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &GrantOutput{
		GrantId:    *grant.GrantId,
		GrantToken: *grant.GrantToken,
		Grantee:    *lam.Configuration.Role,
	}, nil
}

func (g *Granter) Revoke(ctx context.Context, input *RevokeInput) error {
	_, err := g.kms.RevokeGrantWithContext(ctx, &kms.RevokeGrantInput{
		GrantId: &input.GrantId,
		KeyId:   &g.keyId,
	})
	return errors.WithStack(err)
}
