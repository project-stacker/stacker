package trust

import (
	"context"
	"encoding/json"
	"fmt"

	"sort"

	"github.com/docker/cli/cli/command/image"
	"github.com/docker/cli/cli/trust"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/registry"
	"github.com/sirupsen/logrus"
	"github.com/storageos/go-api/types"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/tuf/data"
)

// GetImageReferencesAndAuth retrieves the necessary reference and auth information for an image name
// as an ImageRefAndAuth struct
func GetImageReferencesAndAuth(ctx context.Context, rs registry.Service,
	authResolver func(ctx context.Context, index *registrytypes.IndexInfo) types.AuthConfig,
	imgName string,
) (ImageRefAndAuth, error) {
	ref, err := reference.ParseNormalizedNamed(imgName)
	if err != nil {
		return ImageRefAndAuth{}, err
	}

	// Resolve the Repository name from fqn to RepositoryInfo
	var repoInfo *registry.RepositoryInfo
	if rs != nil {
		repoInfo, err = rs.ResolveRepository(ref)
	} else {
		repoInfo, err = registry.ParseRepositoryInfo(ref)
	}

	if err != nil {
		return ImageRefAndAuth{}, err
	}

	authConfig := authResolver(ctx, repoInfo.Index)
	return ImageRefAndAuth{
		original:   imgName,
		authConfig: &authConfig,
		reference:  ref,
		repoInfo:   repoInfo,
		tag:        getTag(ref),
		digest:     getDigest(ref),
	}, nil
}

// lookupTrustInfo returns processed signature and role information about a notary repository.
// This information is to be pretty printed or serialized into a machine-readable format.
func lookupTrustInfo(remote string) ([]trustTagRow, []client.RoleWithSignatures, []data.Role, error) {
	ctx := context.Background()
	imgRefAndAuth, err := trust.GetImageReferencesAndAuth(ctx, nil, image.AuthResolver(cli), remote)
	if err != nil {
		return []trustTagRow{}, []client.RoleWithSignatures{}, []data.Role{}, err
	}
	tag := imgRefAndAuth.Tag()
	notaryRepo, err := cli.NotaryClient(imgRefAndAuth, trust.ActionsPullOnly)
	if err != nil {
		return []trustTagRow{}, []client.RoleWithSignatures{}, []data.Role{}, trust.NotaryError(imgRefAndAuth.Reference().Name(), err)
	}

	if err = clearChangeList(notaryRepo); err != nil {
		return []trustTagRow{}, []client.RoleWithSignatures{}, []data.Role{}, err
	}
	defer clearChangeList(notaryRepo)

	// Retrieve all released signatures, match them, and pretty print them
	allSignedTargets, err := notaryRepo.GetAllTargetMetadataByName(tag)
	if err != nil {
		logrus.Debug(trust.NotaryError(remote, err))
		// print an empty table if we don't have signed targets, but have an initialized notary repo
		if _, ok := err.(client.ErrNoSuchTarget); !ok {
			return []trustTagRow{}, []client.RoleWithSignatures{}, []data.Role{}, fmt.Errorf("no signatures or cannot access %s", remote)
		}
	}
	signatureRows := matchReleasedSignatures(allSignedTargets)

	// get the administrative roles
	adminRolesWithSigs, err := notaryRepo.ListRoles()
	if err != nil {
		return []trustTagRow{}, []client.RoleWithSignatures{}, []data.Role{}, fmt.Errorf("no signers for %s", remote)
	}

	// get delegation roles with the canonical key IDs
	delegationRoles, err := notaryRepo.GetDelegationRoles()
	if err != nil {
		logrus.Debugf("no delegation roles found, or error fetching them for %s: %v", remote, err)
	}

	return signatureRows, adminRolesWithSigs, delegationRoles, nil
}

func getRepoTrustInfo(remote string) ([]byte, error) {
	signatureRows, adminRolesWithSigs, delegationRoles, err := lookupTrustInfo(cli, remote)
	if err != nil {
		return []byte{}, err
	}
	// process the signatures to include repo admin if signed by the base targets role
	for idx, sig := range signatureRows {
		if len(sig.Signers) == 0 {
			signatureRows[idx].Signers = append(sig.Signers, releasedRoleName)
		}
	}

	signerList, adminList := []trustSigner{}, []trustSigner{}

	signerRoleToKeyIDs := getDelegationRoleToKeyMap(delegationRoles)

	for signerName, signerKeys := range signerRoleToKeyIDs {
		signerKeyList := []trustKey{}
		for _, keyID := range signerKeys {
			signerKeyList = append(signerKeyList, trustKey{ID: keyID})
		}
		signerList = append(signerList, trustSigner{signerName, signerKeyList})
	}
	sort.Slice(signerList, func(i, j int) bool { return signerList[i].Name > signerList[j].Name })

	for _, adminRole := range adminRolesWithSigs {
		switch adminRole.Name {
		case data.CanonicalRootRole:
			rootKeys := []trustKey{}
			for _, keyID := range adminRole.KeyIDs {
				rootKeys = append(rootKeys, trustKey{ID: keyID})
			}
			adminList = append(adminList, trustSigner{"Root", rootKeys})
		case data.CanonicalTargetsRole:
			targetKeys := []trustKey{}
			for _, keyID := range adminRole.KeyIDs {
				targetKeys = append(targetKeys, trustKey{ID: keyID})
			}
			adminList = append(adminList, trustSigner{"Repository", targetKeys})
		}
	}
	sort.Slice(adminList, func(i, j int) bool { return adminList[i].Name > adminList[j].Name })

	return json.Marshal(trustRepo{
		Name:               remote,
		SignedTags:         signatureRows,
		Signers:            signerList,
		AdministrativeKeys: adminList,
	})
}
