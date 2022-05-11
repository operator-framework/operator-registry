#!/usr/bin/env bash

INTERACTIVE=${INTERACTIVE:-"1"}
NOEXEC=${NOEXEC:-"0"}

prompt() {
    echo ""
    echo -n "$ "
}

typeline() {
    case $1 in
       -x) 
           CMD=$2
           NOEXEC=1
           ;;
       *) 
           CMD=$1 
           ;;
    esac

    prompt
    sleep 1
    for (( i=0; i<${#CMD}; i++ )); do
        echo -n "${CMD:$i:1}"
        sleep 0.06
    done
    echo ""
    sleep 0.25
    if [ "$NOEXEC" = "0" ] ; then
        $CMD
        [[ "$INTERACTIVE" == "1" ]] && read -p "hit <ENTER> to continue..."
    fi
}
