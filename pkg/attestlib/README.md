# Attestation Library

The Attestation Library provides an interface for creating and verifying Attestations through a Signer and a Verifier, respectively.

## Components

### Attestation

An [Attestation](https://github.com/grafeas/kritis/blob/master/pkg/attestlib/attestation.go#L25) is a signed statement about a container image in a known format. A container is allowed to be deployed to a Kubernetes cluster if it presents Attestations that satisfy the cluster's policy. An Attestation contains a payload and a signature generated over the payload with a trusted entity’s private key. It also contains the ID of the public key which can verify the Attestation’s signature.

### Public Key
A [PublicKey](https://github.com/grafeas/kritis/blob/master/pkg/attestlib/public_key.go#L29) is the definitive trust anchor used to verify that an Attestation’s signature is valid. Unlike Attestations, which are considered untrustworthy until verified, PublicKeys are assumed to contain trustworthy information. Consequently, this information should be provided directly by the trusted party.

A PublicKey contains the raw public key material and an ID. It also contains a KeyType, one of {`Pgp`, `Pkix`, or `Jwt`}, indicating how the trusted entity stores data within the Attestation. It also contains a SignatureAlgorithm, indicating the cryptographic algorithm, padding algorithm, and hash function used on the payload to create the signature in the Attestation.

### Private Key
The trusted entity has a private key, which is used by the Signer to generate an Attestation’s signature.

### Payload
The payload is a message provided by the trusted entity regarding a container image. It is signed by their private key to create a signature, and both the signature and payload are stored in the Attestation. A payload should not be trusted until the Verifier has verified the Attestation's signature. The Verifier will also assert that the payload describes image being deployed.

By convention, the payload is a JSON-encoded string conforming to the [Red Hat Atomic Host signature format](https://github.com/aweiteka/image/blob/e5a20d98fe698732df2b142846d007b45873627f/docs/signature.md).

## Interface

### Signing

#### Signer
A trusted entity will use a Signer to generate Attestations. There are three signer implementations, one for each KeyType: a `PgpSigner`, `PkixSigner`, and a `JwtSigner`. Each signer has its own constructor (e.g. `NewPgpSigner`), which creates a Signer storing the trusted entity’s private key and any other data required to create an Attestation.

Each signer implements the Signer interface: the `CreateAttestation` method. When passed a payload to sign over, `CreateAttestation` will generate and return an Attestation containing the signature.

### Verifying

#### PublicKey
To create a PublicKey, the user can call `NewPublicKey`, passing in the public key material, key ID, KeyType, and any other data necessary to verify an Attestation.

#### Verifier
Anyone who wishes to verify an Attestation will use a Verifier. There is a single verifier implementation, which is capable of verifying any type of Attestation. It has a constructor `NewVerifier` which receives a slice of PublicKeys that will be used to verify an Attestation and the image name that the Attestations should be associated with.

The verifier satisfies the Verifier interface: the `VerifyAttestation` method. When passed an Attestation, it checks that the Attestation’s signature can be verified by any of the verifier’s public keys. It also extracts the payload used to generate the signature and checks that it corresponds with the given image. If either step is unsuccessful, it returns an error.