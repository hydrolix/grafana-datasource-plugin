#!/usr/bin/env bash

# Usage:
#   ./remove-image.sh <JOB_TOKEN> <PROJECT-ID> <REPOSITORY-NAME> <TAG-NAME>
#
# Example:
#   ./remove-image.sh my-super-token 123456 foo bar

set -euo pipefail
#set -o xtrace

JOB_TOKEN="$1"
PROJECT_ID="$2"
REPOSITORY_NAME="$3"
TAG_NAME="$4"

all_repositories="$(curl --silent --header "JOB-TOKEN: ${JOB_TOKEN}" \
  "https://gitlab.com/api/v4/projects/${PROJECT_ID}/registry/repositories")"

for repository_id in $(echo "${all_repositories}" | jq -r --arg NAME "${REPOSITORY_NAME}" '.[] | select(.name == $NAME) | .id'); do

  echo "Found repository '${REPOSITORY_NAME}' (${repository_id})"

  all_tags="$(curl --silent --header "JOB-TOKEN: ${JOB_TOKEN}" \
    "https://gitlab.com/api/v4/projects/${PROJECT_ID}/registry/repositories/${repository_id}/tags")"

  matching_tags="$(echo "${all_tags}" | jq -r --arg TAG_NAME "${TAG_NAME}" '.[] | select(.name == $TAG_NAME) | .name')"

  if [ -z "${matching_tags}" ]; then
    echo "No tags named '${TAG_NAME}' found in repository '${REPOSITORY_NAME}' (${repository_id})"
  else
    for tag in ${matching_tags}; do
      curl --silent --request DELETE \
        --header "JOB-TOKEN: ${JOB_TOKEN}" \
        "https://gitlab.com/api/v4/projects/${PROJECT_ID}/registry/repositories/${repository_id}/tags/${tag}"
      echo "Tag '${tag}' removed from repository '${REPOSITORY_NAME}' (${repository_id})"
    done
  fi

done
