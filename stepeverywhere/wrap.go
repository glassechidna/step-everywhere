package stepeverywhere

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/glassechidna/awsctx/service/kmsctx"
	"github.com/pkg/errors"
	"net/http"
)

func Wrap(handler interface{}) lambda.Handler {
	var h lambda.Handler
	if lh, ok := handler.(lambda.Handler); ok {
		h = lh
	} else {
		h = lambda.NewHandler(handler)
	}

	sess := session.Must(session.NewSession())
	kmsApi := kmsctx.New(kms.New(sess), nil)
	return &wrapper{inner: h, kms: kmsApi}
}

type sessionKey struct{}

func SessionFromContext(ctx context.Context) *session.Session {
	return ctx.Value(sessionKey{}).(*session.Session)
}

type wrapInput struct {
	Context     Context
	Credentials Credentials
	Payload     json.RawMessage
}

type wrapper struct {
	inner lambda.Handler
	kms   kmsctx.KMS
}

func (w *wrapper) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	var input wrapInput
	err := json.Unmarshal(payload, &input)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	secret, err := w.secretPayload(ctx, input)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewStaticCredentials(input.Credentials.AccessKeyId, secret.SecretAccessKey, input.Credentials.SessionToken)
	config := aws.NewConfig().WithCredentials(creds).WithRegion(input.Context.Region)
	sess := session.Must(session.NewSession(config))
	ctx = context.WithValue(ctx, sessionKey{}, sess)

	output, err := w.inner.Invoke(ctx, input.Payload)
	if err != nil {
		return nil, err
	}

	err = w.putResult(ctx, output, secret.OutputUrl)
	return nil, errors.WithStack(err)
}

func (w *wrapper) putResult(ctx context.Context, output []byte, url string) error {
	body := bytes.NewReader(output)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, body)
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = http.DefaultClient.Do(req)
	return errors.WithStack(err)
}

func (w *wrapper) secretPayload(ctx context.Context, input wrapInput) (*SecretPayload, error) {
	resp, err := w.kms.DecryptWithContext(ctx, &kms.DecryptInput{
		CiphertextBlob: input.Credentials.Encrypted,
		GrantTokens:    []*string{&input.Credentials.Grant},
		EncryptionContext: map[string]*string{
			"Recipient": &input.Credentials.Grantee,
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	secret := SecretPayload{}
	err = json.Unmarshal(resp.Plaintext, &secret)
	return &secret, errors.WithStack(err)
}
