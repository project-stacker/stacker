#!/bin/bash

# allow fmt.Print in inspect.go, since it is supposed to print output to
# stdout. disallow it everywhere else, since this stuff often sneaks into
# commits as debug statements (and we should be using log anyway).
[ "$(git grep "fmt.Print" | grep -c -v -e "cmd/stacker/inspect.go" -e "test/static-analysis.sh")" -gt 0 ] && {
    RED='\033[0;31m'
    NC='\033[0m'
    printf "${RED}using fmt.Print* directive outside of inspect${NC}\n"

    # Die in CI, but just print the warning in regular mode. It is handy to
    # use fmt.Println() for debugging.
    if [ -n "$CI" ]; then
        exit 1
    fi
}

# disallow all fmt.Errorf errors, as these do not give stack traces and are
# harder to debug.
[ "$(git grep "fmt.Errorf" | grep -c -v "test/static-analysis.sh")" -gt 0 ] && {
    echo "using fmt.Errorf directive; use errors.Errorf instead"
    exit 1
}

exit 0
