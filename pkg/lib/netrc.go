package lib

import (
	"os"
	"path/filepath"

	"github.com/git-lfs/go-netrc/netrc"
)

// NetrcCredentials looks up credentials for host in the user's netrc file. uses
// $NETRC if set or ~/.netrc - The bool return is false when there is no
// matching machine or default entry with credentials.
func NetrcCredentials(host string) (string, string, bool, error) {
	filename := os.Getenv("NETRC")
	if filename == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", false, err
		}
		filename = filepath.Join(home, ".netrc")
	}

	machine, err := netrc.FindMachine(filename, host, "")
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	if machine == nil {
		return "", "", false, nil
	}

	login, password := machine.Login, machine.Password
	return login, password, login != "" || password != "", nil
}
