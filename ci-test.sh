#!/bin/bash
set -e

filename=coverage.out
tmpfile=coverage.tmp

echo 'mode: count' > $filename

for dir in $(find . -maxdepth 10 -not -path './.git*' -not -path '*/_*' -not -path './vendor/*' -type d);
do
if ls $dir/*.go &> /dev/null; then
    go test -short -covermode=count -coverprofile=$dir/$tmpfile $dir
    if [ -f $dir/$tmpfile ]
    then
        cat $dir/$tmpfile | tail -n +2 >> $filename
        rm $dir/$tmpfile
    fi
fi
done

go tool cover -func $filename