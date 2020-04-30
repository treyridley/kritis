/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cryptolib

import (
	"testing"
)

// These keys and signatures generated by the following commands:
// `gpg --quick-generate-key --yes pgp@cryptolib.com`
// `gpg --export --armor pgp@cryptolib.com`
// `gpg --output gpgSignature --armor -u pgp@cryptolib.com --sign payload`

const gpgSignature = `-----BEGIN PGP MESSAGE-----

owGbwMvMwMXYscfgj8vNNfGMa0ySuAsSK3PyE1P0SipK4hYpLizJyCxWAKKSjFQF
qJRCpkJ5ZnGGQkm+QnFmeh5XJ+MxFgZGLgYDMUWWdxL7y7ZZP960tfZzGsxYViaQ
QUIyBekFDslFlQUl+TmZSXrJ+bkMXJwCMEVpujwMk9o/R97sS71SqfTl6a6bq8Qz
49zlgvL9WrS7nfJ/JiRUy9dHHmP/ke+V8uH8i4VLHn5xbRU9IXlxoeSjz8dL9828
+szN/Oo0mVlMu492RlisK+R8PiVmeUBw96b1sTV8TjeFPbKPbHJ2X3fueIWFX7RB
2YvfMofMz+5q9Ck/YSw5oU3z57+fL+tnd7wOeVoQb3C6ed5sPe4l4Ze68u9NMIhQ
te+NWWVacap3wltdxwKWCt3ieuXE91wXhBeu+jfh9P0u0xN1atVxj0UlMlWuJD9h
0mh48bRgvd2Kl023FSUu3Yh7G/hM/PvK9ln2os0rWEyFD9zK6I2r1Nfk2FKVF6/P
yvjtzcyFZ5xLJbYdDLvctXHXw+qCKh2eW3eygma0fkriYX/py+cf7j5zmtEnoUV/
v15dp5ceJWmjsb3pIavHzSPPAr3n5DIfyHyQcjlnqmqw2Os731+L2jD6NncdS3aw
r9ls06gvnSG3ZAm/6dUAWcY0F8FbW/e/TbkvdPDm2vknTH0TwiuPtMlsvNAhLZr3
5jsA
=qeZN
-----END PGP MESSAGE-----
`

const gpgPublicKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
N0Tk
=8PoI
-----END PGP PUBLIC KEY BLOCK-----
`

const payload = `this is the payload i wish to sign
`

func TestVerifyPgp(t *testing.T) {
	tcs := []struct {
		name        string
		signature   []byte
		publicKey   []byte
		expectedErr bool
	}{
		{
			name:        "valid signature and public key",
			signature:   []byte(gpgSignature),
			publicKey:   []byte(gpgPublicKey),
			expectedErr: false,
		},
		{
			name:        "invalid signature",
			signature:   []byte("invalid-sig"),
			publicKey:   []byte(gpgPublicKey),
			expectedErr: true,
		},
		{
			name:        "invalid public key",
			signature:   []byte(gpgSignature),
			publicKey:   []byte("invalid-public-key"),
			expectedErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actualPayload, err := verifyPgp(tc.signature, tc.publicKey)
			if tc.expectedErr {
				if err == nil {
					t.Fatalf("Expected error, but returned none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				expectedPayload := []byte(payload)
				if string(actualPayload) != string(expectedPayload) {
					t.Errorf("Incorrect payload extracted: got: %s, want: %s", string(actualPayload), string(expectedPayload))
				}
			}
		})
	}
}