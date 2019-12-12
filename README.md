# `step-everywhere`

Do you ever find yourself wanting to run some code in 10-100s of AWS accounts 
and regions? Maybe even stick in a Lambda function, but not have to deploy it
300 times? `step-everywhere` aims to solve that pain point.

You provide:

* A list of regions 
* The name of a role to assume in each account
* The name of a Lambda function to execute in each account + region
* (Optionally) A custom payload to send to the Lambda.

It uses a combination of Lambda, Step Functions, KMS, STS and S3 to:

* Retrieve temporary credentials in every account
* Generate presigned S3 URLs for storing results
* Encrypt those credentials and S3 URLs
* Pass those credentials and region to your Lambda
* Your Lambda uploads its results to the presigned URL 

All permutations of (account, region) are run as concurrently as
Step Functions will permit.

## Usage

Start an execution of the Step Function with the following payload:

```json5
{
  /* if not provided, defaults to every AWS region (even those disabled by default!) */
  "Regions": ["ap-southeast-2", "us-east-1"],
  
  /* name of role available to be assumed in every account in your org (see security
     section below) */
  "RoleName": "OrganizationAccountAccessRole",

  /* ARN of the "worker" lambda function that will get invoked for every permutation */
  "Function": "arn:aws:lambda:ap-southeast-2:607481581596:function:StepEverywhere_example",
 
  /* optionally you can provide a payload to be sent to every lambda invocation of your lambda */
  "Payload": [
    {
      "Key": "Name", 
      "Option": "Contains", 
      "Values": ["Zen"]
    }
  ]
}
```

By default, output from each invocation will be stored in S3 at the path
`s3://bucket/StepEverywhere/{{ .Function }}/{{ .FunctionQualifier }}/{{ .ExecutionId }}/{{ .AccountId }}/{{ .Region }}.json`
where the placeholders are resolved to:

* `Function`: the name of your Lambda worker function
* `FunctionQualifier`: either `$LATEST` or the specific version/alias specified 
   in your input's Function ARN.
* `ExecutionId`: the unique ID (typically a GUID) of the execution of the Step Function
* `AccountId`: numeric AWS account ID
* `Region`: region code, e.g. `us-east-1`

## Security considerations

You're running arbitrary code in every AWS account you own! That's inherently a 
risky proposition, but that doesn't mean we can't put *some* guard-rails around
this.

a) Encrypting temporary credentials using KMS. Standard workflow Step Functions
persist input and output from each stage during execution and for 90 days afterwards.
If these credentials *weren't* encrypted, anyone with even read-only access to the
Step Function could view and use the credentials for themselves.

b) Fine-grained KMS key policy. If you use the KMS key resource in the CloudFormation
template (rather that providing your own key ID), the ability to decrypt the
credentials is restricted to only the role of Lambda that should rightfully have 
access. This means that even if a human with `kms:Decrypt` on `*` privileges
tries to decrypt the ciphertext, it will be denied.

c) Only Lambda functions with a `StepEverywhere_` prefix in their name can be
specified as the "worker" function. This prevents you from being able to specify
*completely* arbitrary Lambdas as worker functions, as things could implode if
a Lambda receives unexpected credentials.

d) The role assumed in each account must have a `stepeverywhere:assumable: true`
tag on it. This means that users cannot specify IAM roles that 1) trust the
source account in their trust policy but b) the administrators did not intend
to be used by `step-everywhere`.

## TODO

* Only assume role once per account and share for each region (maybe)
* Try-catch around worker so that a single failure doesn't cause others to abort
* Lambda layers to hold helpers for nodejs and python
* Golang package niceties
* An example of how to parse using Athena (maybe)
* CI/CD for publishing to the public Serverless App Repository
