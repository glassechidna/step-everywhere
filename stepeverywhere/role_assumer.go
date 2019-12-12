package stepeverywhere

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/glassechidna/awsctx/service/kmsctx"
	"github.com/glassechidna/awsctx/service/stsctx"
	"github.com/pkg/errors"
	"strings"
	"text/template"
	"time"
)

type RoleAssumer struct {
	s3             s3iface.S3API
	sts            stsctx.STS
	kms            kmsctx.KMS
	keyId          string
	bucket         string
	keyPattern     string
	sessionPattern string
}

func NewRoleAssumer(s3 s3iface.S3API, sts stsctx.STS, kms kmsctx.KMS, keyId, bucket, keyPattern, sessionPattern string) *RoleAssumer {
	return &RoleAssumer{s3: s3, sts: sts, kms: kms, keyId: keyId, bucket: bucket, keyPattern: keyPattern, sessionPattern: sessionPattern}
}

type AssumeRoleInput struct {
	Context     Context
	ExecutionId string
	Grant       string
	Grantee     string
	Function    string
}

type AssumeRoleOutput struct {
	Context     Context
	Credentials Credentials
}

func (r *RoleAssumer) AssumeRole(ctx context.Context, input *AssumeRoleInput) (*AssumeRoleOutput, error) {
	execIdParts := strings.SplitN(input.ExecutionId, ":", 8)
	stateMachine := execIdParts[6]
	executionId := execIdParts[7]

	functionParts := strings.SplitN(input.Function, ":", 8)
	functionName := functionParts[6]
	qualifier := "$LATEST"
	if len(functionParts) == 8 {
		qualifier = functionParts[7]
	}

	tmpl, err := template.New("").Parse(r.keyPattern)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tmplMap := map[string]string{
		"StateMachine":      stateMachine,
		"ExecutionId":       executionId,
		"Function":          functionName,
		"FunctionQualifier": qualifier,
		"AccountId":         input.Context.AccountId,
		"Region":            input.Context.Region,
	}

	keyb := &strings.Builder{}
	err = tmpl.Execute(keyb, tmplMap)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", input.Context.AccountId, input.Context.RoleName)

	tmpl, err = template.New("").Parse(r.sessionPattern)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sessionb := &strings.Builder{}
	err = tmpl.Execute(sessionb, tmplMap)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	sessionName := sessionb.String()

	resp, err := r.sts.AssumeRoleWithContext(ctx, &sts.AssumeRoleInput{
		RoleArn:         &roleArn,
		RoleSessionName: aws.String(sessionName),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c := resp.Credentials

	req, _ := r.s3.PutObjectRequest(&s3.PutObjectInput{
		Bucket: &r.bucket,
		Key:    aws.String(keyb.String()),
	})

	url, err := req.Presign(20 * time.Minute)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	secret := SecretPayload{SecretAccessKey: *c.SecretAccessKey, OutputUrl: url}
	plaintext, _ := json.Marshal(secret)

	kmsResp, err := r.kms.EncryptWithContext(ctx, &kms.EncryptInput{
		KeyId:     &r.keyId,
		Plaintext: plaintext,
		EncryptionContext: map[string]*string{
			"Recipient": &input.Grantee,
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &AssumeRoleOutput{
		Context: input.Context,
		Credentials: Credentials{
			AccessKeyId:  *c.AccessKeyId,
			SessionToken: *c.SessionToken,
			Encrypted:    kmsResp.CiphertextBlob,
			Grant:        input.Grant,
			Grantee:      input.Grantee,
		},
	}, nil
}
