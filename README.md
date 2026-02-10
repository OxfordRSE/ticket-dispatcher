# ticket-dispatcher

ticket-dispatcher posts comments on GitHub issues via email sent to a specific
domain such as `NNN@issues.example.com` which will post to a GitHub repository
for issue NNN.

## Development

ticket-dispatcher requires Go >= 1.25.

To build and test locally:

```shell
git clone https://github.com/OxfordRSE/ticket-dispatcher
cd ticket-dispatcher
go build
go test
```

## Deployment

### Generate a GitHub PAT

This can be done by following the instructions at
https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#creating-a-fine-grained-personal-access-token

You may have to switch to an organisation view in the token creation to see all
repositories. 

> [!WARNING]
> Only grant write access to issues for the selected repository

### Create S3 bucket

A S3 bucket will be required to store emails briefly before forwarding to the
AWS Lambda function; for the purposes of this documentation, we will call it
`anomalies-unseen-incoming`. Attach the following policy to the bucket, which
will allow SES to write emails here.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowSES",
            "Effect": "Allow",
            "Principal": {
                "Service": "ses.amazonaws.com"
            },
            "Action": [
                "s3:PutObject",
                "s3:PutObjectAcl"
            ],
            "Resource": "arn:aws:s3:::anomalies-unseen-incoming/*",
            "Condition": {
                "StringEquals": {
                    "aws:SourceAccount": "ACCOUNT_ID"
                },
                "ArnLike": {
                    "aws:SourceArn": "arn:aws:ses:AWS_REGION:ACCOUNT_ID:receipt-rule-set/*"
                }
            }
        }
    ]
}
```

Optionally, enable a **lifecycle rule**, which will delete emails after N days.
To do this, go to **Management** > **Lifecycle rules** > **Create lifecycle rule**.
Select *Apply to all objects in the bucket*; under Lifecycle rule
actions, select *Expire current versions of objects*, and select the number of
days after which objects (emails) are deleted.

### Set up SES

SES is Amazon's Simple Email Service. This can be configured to drop emails in a
S3 bucket which can be read by ticket-dispatcher. The following documentation
assumes that you will be handling emails to `NNN@anomalies.unseen.ac.uk`.

* **Domain verification**: This is easier if `unseen.ac.uk` is already under
  Route53 as AWS creates the verification TXT records automatically. Otherwise
  you will need to create the TXT records manually. You can create a new domain
  identity to receive emails at **AWS SES** > **Configuration: Identities** >
  **Create Identity**. Verification may take a couple of hours.
* **Set MX record**: MX records route email for a domain to servers. Add a MX
  record for `anomalies.unseen.ac.uk` pointing to
  `inbound-smtp.AWS_REGION.amazonaws.com`, where `AWS_REGION` is the region you
  are using for ticket-dispatcher infrastructure.
* Ensure SES and the MX endpoint are in the same region
* **Receipt rule set**: Create an SES receipt rule set (**Configuration** >
  **Email receiving**) and mark it Active.
* Within the rule set, create a rule, name it anything, enable TLS and
  spam/virus scanning, click Next, set `anomalies.unseen.ac.uk` as the
  *Recipient condition*, click Next to add an action.
* **Adding SES action**: Add a SES action to deliver to a S3 bucket (which
  should be allowed by the policy in the previous section), enter the bucket
  name, review and save the rule.
* **Test email delivery**: Create a new email to 123@anomalies.unseen.ac.uk and
  ensure that it delivers to the S3 bucket.

### Deploy ticket-dispatcher

Set the environment variables in `.env`, `ACCOUNT_ID` is the AWS account ID, and
`GITHUB_TOKEN` is the PAT generated above

```shell
GITHUB_TOKEN=...
TICKET_DISPATCHER_DOMAIN=anomalies.unseen.ac.uk
WHITELIST_DOMAIN=unseen.ac.uk
GITHUB_PROJECT=org/repo
AWS_REGION=eu-west-2
ACCOUNT_ID=123456789
BUCKET=am-post
TAGS="Key=project-name,Value=unseen-anomaly-tracker"
```

Then run the following, in order:

```shell
./scripts/build-linux.sh   # builds a linux/amd64 binary of ticket-dispatcher
./scripts/setup-lambda.sh  # creates lambda function
./scripts/setup-s3-lambda-link.sh  # links S3 bucket to lambda
```

Once deployed, the ticket-dispatcher lambda function can be updated by calling

```shell
./scripts/update-lambda.sh
```

**Logs**: Cloudwatch logs can be found at `/aws/lambda/ticket-dispatcher`