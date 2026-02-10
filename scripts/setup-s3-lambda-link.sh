#!/bin/bash
set -eou pipefail
bash scripts/require-env.sh
echo "=> Adding permissions to enable S3 to call Lambda"
aws lambda add-permission \
  --function-name ticket-dispatcher \
  --statement-id s3-invoke-lambda \
  --action "lambda:InvokeFunction" \
  --principal s3.amazonaws.com \
  --source-arn arn:aws:s3:::$BUCKET \
  --region $AWS_REGION
echo "=> Setup notification from S3 bucket to Lambda"
notification="$(mktemp -t ticket-dispatcher-)"
envsubst < ./aws/s3-notify-lambda.json > "$notification"
aws s3api put-bucket-notification-configuration \
--bucket $BUCKET --notification-configuration file://$notification