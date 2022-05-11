#!/usr/bin/env bash
set -e
USERNAME=
IMG=
VERSION_LIST="0.1.0 0.1.1 0.1.2 0.1.3 0.2.0 0.2.1 0.2.2 0.3.0 1.0.0 1.0.1 1.1.0"

fbc_gen_script="$(dirname ${BASH_SOURCE[0]})/generate_bundle.sh"

# dumb as a box of hammers scripts to generate a bunch of versions of the same bundle in a repo 
# which can be used to do some testing on semver veneers
# To use, 
# 1 - get a local clone of the bundle image you wish to use (ex: cd ~/devel; git clone git@github.com:grokspawn/etcd-bundle.git)
# 2 - update the VERSION_LIST above to represent the test versions of the bundle image you wish to generate
# 3 - from the local bundle image repo clone, execute this script with correct image tag and username inputs (ex: ~/devel/operator-registry/alpha/veneer/semver/demo/gen_bundle_images.sh -u grokspawn -i docker.io/grokspawn/foo-bundle)
# 4 - PROFIT!

function usage() {
    echo ""
    echo "USAGE: "
    echo "$0 [-i image-file -u username]"
    echo ""
    echo "-h :  this text "
    echo "-u :  the username to use with the operator bundle image push "
    echo "          without quotes (e.g. \"barberOfSeville\") "
    echo "-i :  the image URI to be used for docker build/push bundle"
    echo "          image targets, without version (e.g. quay.io/helpoperators/foo-bundle)"
}

while getopts "u:i:h" o; do
    case "${o}" in 
        u)  
            USERNAME=$OPTARG; 
            ;;
        i) 
            IMG=$OPTARG; 
            ;;
        h | *)
            usage 
            exit 1
            ;;
    esac
done
shift $((OPTIND-1))

# echo "SHELL is $SHELL"
# echo "USERNAME: $USERNAME"
# echo "IMG: $IMG"

if [ -z $USERNAME ]; then
    echo "ERROR:  No username specified"
    usage
    exit 2
fi

if [ -z $IMG ]; then
    echo "ERROR:  No image URI specified"
    usage
    exit 3
fi

# zsh requires 
# set -o shwordsplit
# in order to actually split a word list by whitespace
for version in $VERSION_LIST
do
    targetrepo=$IMG version=$version force=true $fbc_gen_script
#    docker build --tag $IMG:$version .
#    docker push $IMG:$version
done
