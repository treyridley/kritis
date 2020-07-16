#!/bin/bash

# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Signer integration testing script
set -ex

# set note id
NOTE_ID=kritis-attestor-note
export NOTE_NAME=projects/${PROJECT_ID}/notes/${NOTE_ID}
export KMS_KEYRING=signer-int-test-keyring
export KMS_KEYNAME=signer-int-test-asymmetric-sign-key
export KMS_KEYLOCATION=global
export KMS_KEYVERSION=1
export KMS_PROJECT=$PROJECT_ID
export KMS_DIGESTALG=SHA512
export NOTE_NAME=projects/${PROJECT_ID}/notes/${NOTE_ID}

# create policy.yaml
cp policy_template.yaml policy.yaml

# install jq
# TODO: bake jq into a custom image
apt-get install -y -q jq
# Helper functions
urlencode() {
    # urlencode <string>
    local LC_COLLATE=C

    local length="${#1}"
    for (( i = 0; i < length; i++ )); do
        local c="${1:i:1}"
        case $c in
            [a-zA-Z0-9.~_-]) printf "$c" ;;
            *) printf '%%%02X' "'$c" ;;
        esac
    done
}

delete_image() {
    ARG=$?
    set +ex
    IMG_TO_DELETE=$1
    echo "Delete image if uploaded."
    gcloud container images delete $IMG_TO_DELETE --force-delete-tags \
      --quiet
    exit $ARG
}

get_occ() {
  IMG_URL_TO_FETCH_OCC=$1
  ACCESS_TOKEN=$(gcloud --project ${PROJECT_ID} auth print-access-token)
  ENCODED_RESOURCE_URL=$(urlencode https://$IMG_URL_TO_FETCH_OCC)
  OCC_NAME=""
  _OCCURRENCES_TO_CLEANUP=$(curl -X GET \
         -H "Content-Type: application/json" \
         -H "Authorization: Bearer ${ACCESS_TOKEN}"  \
         https://containeranalysis.googleapis.com/v1/projects/${PROJECT_ID}/occurrences?filter=kind%3D%22ATTESTATION%22%20AND%20resourceUrl%3D%22${ENCODED_RESOURCE_URL}%22)
  if [ "$(echo ${_OCCURRENCES_TO_CLEANUP} | jq length)" -gt 0 ]; then
    _OCC_NAMES=$(echo ${_OCCURRENCES_TO_CLEANUP} | jq '.occurrences | .[] | .name' | tr -d '"')
    OCC_NAME=${_OCC_NAMES[0]}
  fi

  echo $OCC_NAME
}

delete_occ() {
    ARG=$?
    set +ex
    IMG_DIGEST_URL_TO_DELETE=$1
    if [ -n "$IMG_DIGEST_URL_TO_DELETE" ]; then
      OCC_NAME = $(get_occ $IMG_DIGEST_URL_TO_DELETE)
      echo "Delete occurrence if created."
      if [ -n "$OCC_NAME" ]; then
          ACCESS_TOKEN=$(gcloud --project ${PROJECT_ID} auth print-access-token)
          echo "Delete occurrence ${OCC_NAME}."
          curl -X DELETE \
              -H "Content-Type: application/json" \
              -H "Authorization: Bearer ${ACCESS_TOKEN}"  \
              -H "x-goog-user-project: ${PROJECT_ID}" \
              "https://containeranalysis.googleapis.com/v1/${OCC_NAME}"
      fi
    fi
    exit $ARG
}

export -f urlencode
export -f delete_image
export -f delete_occ
export -f get_occ

#### TEST 1: bypass-and-sign mode ####
./tests/test-bypass-and-sign.sh

#### TEST 2: check-and-sign mode, good case ####
./tests/test-check-and-sign-good.sh

#### TEST 3: check-and-sign mode, bad case ####
./tests/test-check-and-sign-bad.sh

#### TEST 4: check-only mode, good case ####
./tests/test-check-only-good.sh

#### TEST 5: check-only mode, bad case ####
./tests/test-check-only-bad.sh

#### TEST 6: bypass-and-sign mode, with kms ####
./tests/test-bypass-and-sign-with-kms.sh

#### TEST 7: bypass-and-sign mode, overwrite ####
./tests/test-overwrite.sh
