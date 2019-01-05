#!/bin/bash
##

RELEASE_BASE_URL=https://github.com/noisebridge/rfid-access-control/releases
RELEASE_ARCH=armv5   # Earl is running on an old Raspberry Pi

INSTALL_DEST=/usr/local/bin/earl

RELEASE_TMP=$(mktemp -d)
if [ ! -d "${RELEASE_TMP}" ] ; then exit 1; fi

trap '{ rm -rf $RELEASE_TMP/; }' EXIT

if [ $# -ne 1 ] ; then
    echo "Usage: $0 <version>"
    echo "Deploy tagged $RELEASE_ARCH arch version from $RELEASE_BASE_URL to $INSTALL_DEST"
    exit
fi

VERSION=$1

DOWNLOAD_LINK=$RELEASE_BASE_URL/download/$VERSION/earl-${VERSION}.${RELEASE_ARCH}.tar.gz

echo "Downloading ${DOWNLOAD_LINK}"
wget -O $RELEASE_TMP/release.tar.gz ${DOWNLOAD_LINK}

if [ $? -ne 0 ] ; then
    echo "FAILED to download version"
    exit 1
fi

echo "Unpacking"
cd $RELEASE_TMP
tar xvzf ./release.tar.gz

# TODO: should we install with a version number attached and shuffle around
# symbolic links for quick version recovery ?

echo "Installing version ${VERSION} to ${INSTALL_DEST}"
install -o root -g root $RELEASE_TMP/earl-${RELEASE_ARCH} ${INSTALL_DEST}

if [ $? -eq 0 ] ; then echo "SUCCESS" ; else "FAIL"; fi
