#!/bin/bash
set -eou pipefail
echo "=> Building lambda code"
./scripts/build-linux.sh
echo "=> Updating lambda code"
aws lambda update-function-code --function-name ticket-dispatcher --zip-file fileb://function.zip
