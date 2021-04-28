#!/bin/bash

# Copyright PeerFintech, All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0

set -e

# shellcheck source=/dev/null
source "$(cd "$(dirname "$0")" && pwd)/functions.sh"

normal_dir="$(cd "$(dirname "$0")/.." && pwd)"
source_dirs=()
while IFS=$'\n' read -r source_dir; do
    source_dirs+=("$source_dir")
done < <(go list -f '{{.Dir}}' ./... | sed s,"${normal_dir}".,,g | cut -f 1 -d / | sort -u)

echo "Checking with gofmt"
OUTPUT="$(gofmt -l -s "${source_dirs[@]}")"
OUTPUT="$(filterExcludedAndGeneratedFiles "$OUTPUT")"
if [ -n "$OUTPUT" ]; then
    echo "The following files contain gofmt errors"
    echo "$OUTPUT"
    echo "The gofmt command 'gofmt -l -s -w' must be run for these files"
    exit 1
fi

echo "Installing with goimports"
go get golang.org/x/tools/cmd/goimports

echo "Checking with goimports"
## goimports@latest -l 执行发现不符合文件会exit 1会自动退出所在shell
## 故将goimports -l命令放在子shell（subshell）中
## subshell两种方式；方式一：使用管道(pipe)示例：$(goimports -l "${source_dirs[@]}"| tee));方式二使用组合命令，将imports命令放进()内,示例："$(echo $(goimports -l "${source_dirs[@]}"))"。
##
OUTPUT="$(echo $(goimports -l "${source_dirs[@]}"))"
OUTPUT="$(filterExcludedAndGeneratedFiles "$OUTPUT")"
if [ -n "$OUTPUT" ]; then
    echo "The following files contain goimports errors"
    echo "$OUTPUT"
    echo "The goimports command 'goimports -l -w' must be run for these files"
    exit 1
fi

echo "Checking with go vet"
PRINTFUNCS="Print,Printf,Info,Infof,Warning,Warningf,Error,Errorf,Critical,Criticalf,Sprint,Sprintf,Log,Logf,Panic,Panicf,Fatal,Fatalf,Notice,Noticef,Wrap,Wrapf,WithMessage"
OUTPUT="$(go vet -all -printfuncs "$PRINTFUNCS" ./...)"
if [ -n "$OUTPUT" ]; then
    echo "The following files contain go vet errors"
    echo "$OUTPUT"
    exit 1
fi
