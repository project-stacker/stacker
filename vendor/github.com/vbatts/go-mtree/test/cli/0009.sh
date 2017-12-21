#!/bin/bash
set -e

name=$(basename $0)
root="$(dirname $(dirname $(dirname $0)))"
gomtree=$(readlink -f ${root}/gomtree)
t=$(mktemp -t -d go-mtree.XXXXXX)

echo "[${name}] Running in ${t}"

pushd ${root}

# Create some unicode files.
mkdir ${t}/root
echo "some data" > "${t}/root/$(printf 'this file has \u042a some unicode !!')"
echo "more data" > "${t}/root/$(printf 'even more \x07 unicode \ua4ff characters \udead\ubeef\ucafe')"
mkdir -p "${t}/root/$(printf '\024 <-- some more weird characters --> \u4f60\u597d\uff0c\u4e16\u754c')"
ln -s "$(printf '62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r^\u00bfp\u00c6u"q\u00fbc2\u00f0u\u00b8\u00dd\u00e8v\u00ff\u00b0\u00dc\u00c2\u00f53\u00db')" "${t}/root/$(printf 'k\u00f2sd4\\p\u00da\u00a6\u00d3\u00eea<\u00e6s{\u00a0p\u00f0\u00ffj\u00e0\u00e8\u00b8\u00b8\u00bc\u00fcb')"
printf 'some lovely data 62_\u00c6\u00c62\u00ae\u00b7m\u00db\u00c3r' > "${t}/root/$(printf 'T\u00dcB\u0130TAK_UEKAE_K\u00f6k_Sertifika_Hizmet_Sa\u011flay\u0131c\u0131s\u0131_-_S\u00fcr\u00fcm_3.pem')"

# Create manifest and check it against the same root.
${gomtree} -k uid,gid,size,type,link,nlink,sha256digest -c -p ${t}/root > ${t}/root.mtree
${gomtree} -k uid,gid,size,type,link,nlink,sha256digest -f ${t}/root.mtree -p ${t}/root

# Modify it and make sure that it successfully figures out what changed.
echo "othe data" > "${t}/root/$(printf 'this file has \u042a some unicode !!')"
! ${gomtree} -k uid,gid,size,type,link,nlink,sha256digest -f ${t}/root.mtree -p ${t}/root

echo "some data" > "${t}/root/$(printf 'this file has \u042a some unicode !!')"
${gomtree} -k uid,gid,size,type,link,nlink,sha256digest -f ${t}/root.mtree -p ${t}/root

popd
rm -rf ${t}
