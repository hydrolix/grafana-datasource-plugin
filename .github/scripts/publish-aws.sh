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

# AWS S3 bucket and path configuration
S3_BUCKET="${AWS_S3_BUCKET:-grafana-plugins}"
S3_PATH="hydrolix/$PACKAGE_NAME/$PACKAGE_VERSION"

echo "Uploading $ZIP_NAME to s3://$S3_BUCKET/$S3_PATH/$ZIP_NAME"

# Upload to S3
aws s3 cp "$ZIP_NAME" "s3://$S3_BUCKET/$S3_PATH/$ZIP_NAME" --acl public-read

# Create latest symlink
echo "Creating latest version link"
aws s3 cp "s3://$S3_BUCKET/$S3_PATH/$ZIP_NAME" "s3://$S3_BUCKET/hydrolix/$PACKAGE_NAME/latest/$ZIP_NAME" --acl public-read

echo "Successfully published $ZIP_NAME to AWS S3"