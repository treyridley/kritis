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

echo ""
echo ""

set -eux

GOOD_IMAGE_URL=gcr.io/$PROJECT_ID/signer-int-good-image:$BUILD_ID
docker build --no-cache -t $GOOD_IMAGE_URL -f ./Dockerfile.good .

trap 'delete_image $GOOD_IMAGE_URL'  EXIT

# push good image
docker push $GOOD_IMAGE_URL
# get image url with digest format
GOOD_IMG_DIGEST_URL=$(docker image inspect $GOOD_IMAGE_URL --format '{{index .RepoDigests 0}}')

trap 'delete_occ $GOOD_IMG_DIGEST_URL'  EXIT

# sign good image in bypass mode
./signer -v 10 \
-alsologtostderr \
-mode=bypass-and-sign \
-image=${GOOD_IMG_DIGEST_URL} \
-pgp_private_key=private.key \
-policy=policy.yaml \
-note_name=${NOTE_NAME}

sleep 5

# save occ id
OLD_OCC_ID="$(get_occ $GOOD_IMG_DIGEST_URL)"

# sign good image in bypass mode
./signer -v 10 \
-alsologtostderr \
-mode=bypass-and-sign \
-image=${GOOD_IMG_DIGEST_URL} \
-pgp_private_key=private.key \
-policy=policy.yaml \
-note_name=${NOTE_NAME} \
-overwrite

# check the current occ id is not same as old id
NEW_OCC_ID="$(get_occ $GOOD_IMG_DIGEST_URL)"

if [ "$OLD_OCC_ID" == "$NEW_OCC_ID" ]; then
    echo "Attestation is not overwritten as expected"
    exit 1
else
    echo "Attestation is overwritten as expected"
fi

echo ""
echo ""
