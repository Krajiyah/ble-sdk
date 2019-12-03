#!/bin/bash

cov=$(cat report.txt | tail -n 1 | tr -s '\t' | sed 's/\t/ /g' | cut -d ' ' -f 3 | tr -d '%')
cov=${cov%.*}
if (($cov < 65)); then
    echo ""
    echo ""
    echo "==========================ERROR=============================="
    echo "$cov% is less than required 70% code coverage. Please write tests."
    exit 1
fi