package stepeverywhere

type Context struct {
	AccountId string
	Region    string
	RoleName  string
}

type SecretPayload struct {
	SecretAccessKey string
	OutputUrl       string
}

type Credentials struct {
	AccessKeyId  string
	SessionToken string
	Encrypted    []byte
	Grant        string
	Grantee      string
}
