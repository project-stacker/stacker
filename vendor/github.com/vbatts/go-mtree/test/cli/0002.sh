#!/bin/bash
set -e

name=$(basename $0)
root="$(dirname $(dirname $(dirname $0)))"
gomtree=$(readlink -f ${root}/gomtree)
t=$(mktemp -t -d go-mtree.XXXXXX)

echo "[${name}] Running in ${t}"
# This test is for basic running check of manifest, and check against tar and file system
#

pushd ${root}

# Create a symlink with spaces in the entries.
mkdir ${t}/root
ln -s "this is a dummy symlink" ${t}/root/link

# Create manifest and check it against the same symlink.
${gomtree} -K link,sha256digest -c -p ${t}/root > ${t}/root.mtree
${gomtree} -K link,sha256digest -f ${t}/root.mtree -p ${t}/root

popd
rm -rf ${t}
