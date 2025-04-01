#!/usr/bin/env bash

AWS_ACCESS_KEY_ID="$1"
AWS_SECRET_ACCESS_KEY="$2"

if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "Usage: $0 <AWS_ACCESS_KEY_ID> <AWS_SECRET_ACCESS_KEY>"
    echo "Example: $0 AKIA... abc123"
    exit 1
fi

export AWS_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY

ZIP_NAME=$(ls -1 *.zip 2>/dev/null | head -n 1)

if [ -z "$ZIP_NAME" ]; then
    echo "No zip file found in the current directory"
    exit 1
fi

PACKAGE_NAME=$(echo "$ZIP_NAME" | sed -E 's/(.*)-[^-]+\.zip/\1/')
PACKAGE_VERSION=$(echo "$ZIP_NAME" | sed -E 's/.*-([0-9]+\.[0-9]+\.[0-9]+)\.zip/\1/')

if [ -z "$PACKAGE_NAME" ] || [ -z "$PACKAGE_VERSION" ]; then
    echo "Failed to extract package name or version"
    exit 1
fi

PATH_SUFFIX="grafana-datasource-plugin/$ZIP_NAME"

S3_PATH="s3://hdx-public/$PATH_SUFFIX"
PUBLIC_PATH="https://hdx-public.s3.us-east-2.amazonaws.com/$PATH_SUFFIX"

echo "Uploading $ZIP_NAME to $S3_PATH ..."
aws s3 cp "$ZIP_NAME" "$S3_PATH"

echo "Run curl -O $PUBLIC_PATH to get it"
