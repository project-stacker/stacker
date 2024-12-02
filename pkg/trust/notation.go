package trust

import (
	"context"
	"errors"
	"math"
	"os"
	"path"

	"github.com/notaryproject/notation-go"
	"github.com/notaryproject/notation-go/verifier"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func VerifyWithNotation(reference string, tdir string) error {
	// check if trustpolicy.json exists
	trustpolicyPath := path.Join(tdir, "notation/trustpolicy.json")

	if _, err := os.Stat(trustpolicyPath); errors.Is(err, os.ErrNotExist) {
		trustPolicy := `
			{
				"version": "1.0",
				"trustPolicies": [
					{
						"name": "good",
						"registryScopes": [ "*" ],
						"signatureVerification": {
							"level" : "audit" 
						},
						"trustStores": ["ca:good"],
						"trustedIdentities": [
							"*"
						]
					}
				]
			}`

		file, err := os.Create(trustpolicyPath)
		if err != nil {
			return err
		}

		defer file.Close()

		_, err = file.WriteString(trustPolicy)
		if err != nil {
			return err
		}
	}

	// start verifying signatures
	ctx := context.TODO()

	// getRepositoryClient
	authClient := &auth.Client{
		Credential: func(ctx context.Context, reg string) (auth.Credential, error) {
			return auth.EmptyCredential, nil
		},
		Cache:    auth.NewCache(),
		ClientID: "notation",
	}

	authClient.SetUserAgent("notation/zot_tests")

	plainHTTP := true

	// Resolve referance
	ref, err := registry.ParseReference(reference)
	if err != nil {
		return err
	}

	remoteRepo := &remote.Repository{
		Client:    authClient,
		Reference: ref,
		PlainHTTP: plainHTTP,
	}

	repo := notreg.NewRepository(remoteRepo)

	manifestDesc, err := repo.Resolve(ctx, ref.Reference)
	if err != nil {
		return err
	}

	if err := ref.ValidateReferenceAsDigest(); err != nil {
		ref.Reference = manifestDesc.Digest.String()
	}

	// getVerifier
	newVerifier, err := verifier.NewFromConfig()
	if err != nil {
		return err
	}

	remoteRepo = &remote.Repository{
		Client:    authClient,
		Reference: ref,
		PlainHTTP: plainHTTP,
	}

	repo = notreg.NewRepository(remoteRepo)

	configs := map[string]string{}

	verifyOpts := notation.RemoteVerifyOptions{
		ArtifactReference:    ref.String(),
		PluginConfig:         configs,
		MaxSignatureAttempts: math.MaxInt64,
	}

	_, outcomes, err := notation.Verify(ctx, newVerifier, repo, verifyOpts)
	if err != nil || len(outcomes) == 0 {
		return ErrSignatureVerification
	}

	return nil
}
