#!/usr/bin/env bash

ZIP_NAME=$(ls -1 *.zip 2>/dev/null | head -n 1)

if [ -z "$ZIP_NAME" ]; then
    echo "No zip file found in the current directory"
    exit 1
fi

PACKAGE_NAME="${ZIP_NAME%-*.*.zip}"
PACKAGE_VERSION=$(echo "$ZIP_NAME" | sed -E 's/^.*-([0-9]+\.[0-9]+\.[0-9]+[^/]*)\.zip$/\1/')

if [ -z "$PACKAGE_NAME" ] || [ -z "$PACKAGE_VERSION" ]; then
    echo "Failed to extract package name or version"
    exit 1
fi

PATH_SUFFIX="grafana-datasource-plugin/$ZIP_NAME"

S3_PATH="s3://hdx-public/$PATH_SUFFIX"
PUBLIC_PATH="https://hdx-public.s3.us-east-2.amazonaws.com/${PATH_SUFFIX/+/%2B}"

echo "Uploading $ZIP_NAME to $S3_PATH ..."
aws s3 cp "$ZIP_NAME" "$S3_PATH"

echo "Run curl -O $PUBLIC_PATH to get it"
