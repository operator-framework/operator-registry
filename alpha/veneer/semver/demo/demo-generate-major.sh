#!/usr/bin/env bash

SCRIPTPATH="$( cd -- "$(dirname "$0")" > /dev/null 2>&1 ; pwd -P )"

set -o errexit
set -o pipefail

TOGIF="docker run --rm -v $SCRIPTPATH:/data asciinema/asciicast2gif"

INTERACTIVE=0 asciinema rec --overwrite -c $SCRIPTPATH/major-version-demo.sh $SCRIPTPATH/demo.asciinema.json
$TOGIF -w 102 -h 34 $SCRIPTPATH/demo.asciinema.json $SCRIPTPATH/major-version-demo.gif && rm $SCRIPTPATH/demo.asciinema.json
