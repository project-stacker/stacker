#!/bin/bash
set -e
#set -x

name=$(basename $0)
root="$(dirname $(dirname $(dirname $0)))"
gomtree=$(readlink -f ${root}/gomtree)
t=$(mktemp -t -d go-mtree.XXXXXX)

echo "[${name}] Running in ${t}"

pushd ${root}
git archive --format=tar HEAD^{tree} . > ${t}/${name}.tar
mkdir -p ${t}/extract
tar -C ${t}/extract/ -xf ${t}/${name}.tar

## This is a checking that keyword synonyms are respected
${gomtree} -k sha1digest -c -p ${t}/extract/ > ${t}/${name}.mtree
${gomtree} -k sha1 -f ${t}/${name}.mtree -p ${t}/extract/
${gomtree} -k sha1 -c -p ${t}/extract/ > ${t}/${name}.mtree
${gomtree} -k sha1digest -f ${t}/${name}.mtree -p ${t}/extract/

popd
rm -rf ${t}
