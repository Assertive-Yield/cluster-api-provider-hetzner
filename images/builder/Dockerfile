# Copyright 2023 The Kubernetes Authors.
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

# This is the Dockerfile for the Builder Image that is used by the CI pipelines.

# ============ Tool installation stages ============

FROM docker.io/library/alpine:3.21.3 AS lychee
# update: datasource=github-tags depName=lycheeverse/lychee versioning=semver
ARG LYCHEE_VERSION="v0.15.1"
RUN apk add --no-cache curl && \
    curl -fsSL "https://github.com/lycheeverse/lychee/releases/download/${LYCHEE_VERSION}/lychee-${LYCHEE_VERSION}-x86_64-unknown-linux-gnu.tar.gz" | tar xz -C /usr/bin lychee

FROM docker.io/library/alpine:3.21.3 AS golangci
# update: datasource=github-tags depName=golangci/golangci-lint versioning=semver
ARG GOLANGCI_VERSION="v1.64.8"
RUN apk add --no-cache curl && \
    curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s ${GOLANGCI_VERSION}

FROM docker.io/library/alpine:3.21.3 AS helm
# update: datasource=github-tags depName=helm/helm versioning=semver
ARG HELM_VERSION="v3.18.6"
RUN apk add --no-cache curl && \
    curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" | tar xz -C /usr/bin --strip-components=1 linux-amd64/helm

FROM docker.io/hadolint/hadolint:v2.12.0-alpine AS hadolint
FROM docker.io/aquasec/trivy:0.60.0 AS trivy

# ============ Final image ============

FROM docker.io/library/golang:1.25.6-bookworm

# update: datasource=github-tags depName=adrienverge/yamllint versioning=semver
ARG YAMLLINT_VERSION="v1.37.1"

ENV DEBIAN_FRONTEND=noninteractive \
    GOCACHE=/go/cache \
    PIPX_HOME="/opt/pipx" \
    PIPX_BIN_DIR="/usr/local/bin"

# Install system packages in single layer
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && \
    apt-get install -qy --no-install-recommends \
        file \
        gettext \
        gnupg \
        jq \
        libprotobuf-dev \
        libsystemd-dev \
        pipx \
        protobuf-compiler \
        python3 \
        skopeo \
        sudo \
        unzip \
        zip && \
    pipx ensurepath && \
    pipx install --global "yamllint==${YAMLLINT_VERSION#v}"

# Install Go tools
RUN go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0

# Copy tools from builder stages
COPY --from=lychee /usr/bin/lychee /usr/bin/
COPY --from=golangci /bin/golangci-lint /usr/local/bin/
COPY --from=helm /usr/bin/helm /usr/local/bin/
COPY --from=hadolint /bin/hadolint /usr/bin/
COPY --from=trivy /usr/local/bin/trivy /usr/bin/

COPY build.sh /
RUN chmod +x /build.sh

ENTRYPOINT ["/build.sh"]
