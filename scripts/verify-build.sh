#!/usr/bin/env bash

# Copyright 2023 uwu-tools Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

PLATFORMS=(
    linux/amd64
    linux/arm64
    darwin/amd64
    darwin/arm64
)

for PLATFORM in "${PLATFORMS[@]}"; do
    OS="${PLATFORM%/*}"
    ARCH=$(basename "$PLATFORM")

    output_name=gh-jira-issue-sync'-'$OS'-'$ARCH

    if [ "$OS" = "windows" ]; then
        output_name+='.exe'
    fi

    echo "Building project for $PLATFORM"
    CGO_ENABLED=0 GOARCH="$ARCH" GOOS="$OS" go build -trimpath -ldflags "${GHJIRA_LDFLAGS}" -o output/$output_name ./main.go
    pushd output
    sha256sum $output_name >> checksums.txt
    popd
done
