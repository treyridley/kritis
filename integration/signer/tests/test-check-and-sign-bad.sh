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

# build a "bad" example image
BAD_IMAGE_URL=gcr.io/$PROJECT_ID/signer-int-bad-image:$BUILD_ID
docker build --no-cache -t $BAD_IMAGE_URL -f ./Dockerfile.bad .

trap 'delete_image $BAD_IMAGE_URL'  EXIT

# push bad image
docker push $BAD_IMAGE_URL
# get image url with digest format
BAD_IMG_DIGEST_URL=$(docker image inspect $BAD_IMAGE_URL --format '{{index .RepoDigests 0}}')

trap 'delete_occ $BAD_IMG_DIGEST_URL'  EXIT

signing_bad_image_failed=false
./signer -v 10 \
-alsologtostderr \
-image=${BAD_IMG_DIGEST_URL} \
-pgp_private_key=private.key \
-policy=policy.yaml \
-note_name=${NOTE_NAME} || signing_bad_image_failed=true


if [ "$signing_bad_image_failed" = true ] ; then
	echo "Signing failed for bad image as expected."
    exit 0
else
	echo "Error: signing should fail for bad image, but succeeded."
    exit 1
fi

echo ""
echo ""
