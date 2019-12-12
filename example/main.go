package main

import (
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/glassechidna/awsctx/service/ssmctx"
	"github.com/glassechidna/awsctx/service/stsctx"
	"github.com/glassechidna/step-everywhere/stepeverywhere"
	"github.com/pkg/errors"
)

func main() {
	handler := stepeverywhere.Wrap(handler)
	lambda.StartHandler(handler)
}

func handler(ctx context.Context, filters []*ssm.ParameterStringFilter) (interface{}, error) {
	sess := stepeverywhere.SessionFromContext(ctx)
	ssmApi := ssmctx.New(ssm.New(sess), nil)
	stsApi := stsctx.New(sts.New(sess), nil)

	resp, err := ssmApi.DescribeParametersWithContext(ctx, &ssm.DescribeParametersInput{
		ParameterFilters: filters,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	resp2, err := stsApi.GetCallerIdentityWithContext(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return map[string]interface{}{
		"Params":   resp,
		"Identity": resp2,
	}, nil
}
