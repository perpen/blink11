#!/bin/bash

trap 'echo SIGTERM; exit 1' SIGTERM
trap 'echo SIGHUP; exit 1' SIGHUP
trap 'echo SIGINT; exit 1' SIGINT

set | grep BLINK11_ADDR_
[ -z "$BLINK11_ADDR_0" ] && {
    echo "BLINK11_ADDR_0 not set" 1>&2
    exit 1
}
n=$BLINK11_ADDR_0

echo "start with BLINK11_ADDR_0=$n"
for i in $(seq 0 $n); do
    bad=""
    # [ $i = 3 ] && bad=" true"
    echo "metric: hack $i $n$bad"
    sleep .2
done
echo end
