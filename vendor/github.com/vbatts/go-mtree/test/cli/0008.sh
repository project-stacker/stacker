#!/bin/bash
set -e

name=$(basename $0)
root="$(dirname $(dirname $(dirname $0)))"
gomtree=$(readlink -f ${root}/gomtree)
t=$(mktemp -d /tmp/go-mtree.XXXXXX)

echo "[${name}] Running in ${t}"

pushd ${root}
mkdir -p ${t}/extract
git archive --format=tar HEAD^{tree} . | tar -C ${t}/extract/ -x

${gomtree} -K sha256digest -c -p ${t}/extract/ > ${t}/${name}.mtree

## This is a use-case for checking a directory, but by reading the manifest from stdin
## since the `-f` flag is not provided.
cat ${t}/${name}.mtree | ${gomtree} -p ${t}/extract/

popd
rm -rf ${t}
