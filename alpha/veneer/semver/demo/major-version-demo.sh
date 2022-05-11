#!/usr/bin/env bash

SCRIPTPATH="$( cd -- "$(dirname "$0")" > /dev/null 2>&1 ; pwd -P )"

#echo "SCRIPTPATH is $SCRIPTPATH"
. $SCRIPTPATH/demo-functions.sh

#echo "INTERACTIVE is $INTERACTIVE"

INFILE=$HOME/devel/example-operator-index/semver-veneer.yaml

function run() {
	# pretty-print the input schema
	typeline "yq $INFILE"
	# generate the major channels with skips 
	typeline -x "clear"
	clear
	typeline -x "./bin/opm alpha render-veneer semver $INFILE -o yaml --skip-patch --major-channels"
	# using 'noexec' flag because we want to pretty-print the output with yq
	./bin/opm alpha render-veneer semver $INFILE -o yaml --skip-patch --major-channels | yq "select(.schema == \"olm.channel\")"
	typeline -x "clear"
	clear
	# generate the major channels with replaces 
	typeline -x "./bin/opm alpha render-veneer semver $INFILE -o yaml --major-channels"
	# using 'noexec' flag because we want to pretty-print the output with yq
	./bin/opm alpha render-veneer semver $INFILE -o yaml --major-channels | yq "select(.schema == \"olm.channel\")"
	sleep 10
}

run
