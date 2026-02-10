#!/bin/bash
set -eou pipefail
if [[ -z "$GITHUB_TOKEN" || \
    -z "$ACCOUNT_ID" || \
    -z "$TICKET_DISPATCHER_DOMAIN" || \
    -z "$GITHUB_PROJECT" || \
    -z "$WHITELIST_DOMAIN"|| \
    -z "$AWS_REGION" || \
    -z "$BUCKET" || \
    -z "$TAGS" ]]; then
    cat << EOF
Setting up the AWS infrastructure for the ticket-dispatcher
lambda handler requires setting the following environment variables:

GITHUB_TOKEN                A personal access token with scoped 'issues:write'
                            access for the repository to which emails are sent
ACCOUNT_ID                  AWS account ID
TICKET_DISPATCHER_DOMAIN    Domain for which SES is set up (e.g. issues.example.com)
WHITELIST_DOMAIN            Domain from which emails are accepted
GITHUB_PROJECT              GitHub project whose issues will be updated
AWS_REGION                  AWS region where infrastructure is setup
TAGS                        AWS tags to apply to created resources created
EOF
    exit 1
fi