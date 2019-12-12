package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	lambdasvc "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/glassechidna/awsctx"
	"github.com/glassechidna/awsctx/service/kmsctx"
	"github.com/glassechidna/awsctx/service/lambdactx"
	"github.com/glassechidna/awsctx/service/organizationsctx"
	"github.com/glassechidna/awsctx/service/stsctx"
	"github.com/glassechidna/step-everywhere/stepeverywhere"
	"os"
	"strings"
)

func main() {
	sess := session.Must(session.NewSession())

	var handler interface{}
	var ctxer awsctx.Contexter

	orgsApi := organizationsctx.New(organizations.New(sess), ctxer)
	lambdaApi := lambdactx.New(lambdasvc.New(sess), ctxer)
	kmsApi := kmsctx.New(kms.New(sess), ctxer)
	stsApi := stsctx.New(sts.New(sess), ctxer)
	s3api := s3.New(sess)

	keyId := os.Getenv("KmsKeyId")

	granter := stepeverywhere.NewGranter(kmsApi, lambdaApi, keyId)
	collector := stepeverywhere.NewCollector(orgsApi, strings.Split(os.Getenv("DefaultRegionsCsv"), ","))
	assumer := stepeverywhere.NewRoleAssumer(
		s3api,
		stsApi,
		kmsApi,
		keyId,
		os.Getenv("OutputBucket"),
		os.Getenv("OutputKeyPattern"),
		os.Getenv("RoleSessionPattern"),
	)

	switch os.Getenv("Mode") {
	case "Collector":
		handler = collector.CollectAccountIds
	case "AssumeRole":
		handler = assumer.AssumeRole
	case "GrantCreator":
		handler = granter.Grant
	case "GrantRevoker":
		handler = granter.Revoke
	default:
		panic("unknown mode")
	}

	lambda.Start(handler)
}
