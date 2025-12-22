package idmap

import (
	"os/user"
	"strconv"

	"github.com/lxc/incus/v6/shared/idmap"
	"github.com/pkg/errors"
)

func ResolveCurrentIdmapSet() (*idmap.Set, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't resolve current user")
	}
	return resolveIdmapSet(currentUser)
}

func resolveIdmapSet(user *user.User) (*idmap.Set, error) {
	idmapSet, err := idmap.NewSetFromSystem(user.Username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed parsing /etc/sub{u,g}idmap")
	}

	if idmapSet != nil {
		/* Let's make our current user the root user in the ns, so that when
		 * stacker emits files, it does them as the right user.
		 */
		uid, err := strconv.Atoi(user.Uid)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't decode uid")
		}

		gid, err := strconv.Atoi(user.Gid)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't decode gid")
		}
		hostMap := []idmap.Entry{
			idmap.Entry{
				IsUID:    true,
				HostID:   int64(uid),
				NSID:     0,
				MapRange: 1,
			},
			idmap.Entry{
				IsGID:    true,
				HostID:   int64(gid),
				NSID:     0,
				MapRange: 1,
			},
		}

		for _, hm := range hostMap {
			err := idmapSet.AddSafe(hm)
			if err != nil {
				return nil, errors.Wrapf(err, "failed adding idmap entry: %v", hm)
			}
		}
	}

	return idmapSet, nil
}
