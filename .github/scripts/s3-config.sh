#!/usr/bin/env bash
# Single source for the S3 bucket and its public base URL, shared by
# publish-aws.sh (upload + digest round-trip) and release.yml's
# verify-release job (post-publish verification), so a bucket change is one
# edit. Intended to be sourced, not executed.

export S3_BUCKET="hdx-public"
export S3_PUBLIC_BASE_URL="https://hdx-public.s3.us-east-2.amazonaws.com"
