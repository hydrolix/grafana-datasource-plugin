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

echo "$CI_API_V4_URL/projects/$CI_PROJECT_ID/packages/generic/$PACKAGE_NAME/$PACKAGE_VERSION/$ZIP_NAME"

curl --location --header "JOB-TOKEN: $CI_JOB_TOKEN" \
     --upload-file "$ZIP_NAME" \
     "$CI_API_V4_URL/projects/$CI_PROJECT_ID/packages/generic/$PACKAGE_NAME/$PACKAGE_VERSION/$ZIP_NAME"
