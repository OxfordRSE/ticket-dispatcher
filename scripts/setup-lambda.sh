#!/bin/bash
set -eou pipefail
bash ./scripts/require-env.sh
echo "=> Creating lambda role"
aws iam create-role --role-name ticket-dispatcher-lambda-role \
    --assume-role-policy-document file://aws/lambda-role.json \
    --tags $TAGS
echo "=> Set role policy"
lambda_policy="$(mktemp -t ticket-dispatcher-)"
envsubst < ./aws/lambda-policy.json > "$lambda_policy"
aws iam put-role-policy --role-name ticket-dispatcher-lambda-role \
    --policy-name ticket-dispatcher-inline --policy-document file://$lambda_policy
echo "=> Create lambda function"
aws lambda create-function \
  --function-name ticket-dispatcher \
  --runtime provided.al2023 \
  --role arn:aws:iam::$ACCOUNT_ID:role/ticket-dispatcher-lambda-role \
  --handler bootstrap \
  --zip-file fileb://function.zip \
  --timeout 100 \
  --memory-size 512 \
  --region eu-west-2 --tags $TAGS
echo "=> Assign environment variables"
aws lambda update-function-configuration \
  --function-name ticket-dispatcher \
  --environment Variables="{TICKET_DISPATCHER_DOMAIN=$TICKET_DISPATCHER_DOMAIN,WHITELIST_DOMAIN=$WHITELIST_DOMAIN,GITHUB_TOKEN=$GITHUB_TOKEN,GITHUB_PROJECT=$GITHUB_PROJECT}" \
  --region $AWS_REGION
