# syntax=docker/dockerfile:1.7

# Ubuntu 24.04 — NOT bare `ubuntu` (which today resolves to 26.04 / Resolute
# and is rejected by Playwright ≤ 1.59: `does not support chromium on
# ubuntu26.04-arm64`).
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    CI=1

# apt + node 24 via NodeSource. Pinned to noble's nodesource setup. The
# project's package.json `engines` requires Node ≥ 24; the older 20 build
# only emits an EBADENGINE warning and silently mismatches deps — foot-gun.
# (No Ubuntu suite carries Node 24: noble ships 18, resolute ships 22.)
#
# The setup script is downloaded to disk and checked against a recorded
# sha256 instead of being piped straight into bash — a pipe executes the
# bytes as they arrive, leaving nothing to verify. NodeSource publishes no
# checksum or signature of its own, so this digest is trust-on-first-use:
# recorded 2026-09-22, and it pins the bootstrap only. The bytes that
# actually get installed are verified separately by apt against the
# NodeSource GPG key the script imports.
#
# The script carries no version stamp and NodeSource rewrites it without
# notice, so their next rewrite surfaces here as a build failure. Re-record
# it after reading the diff:
#   curl -fsSL https://deb.nodesource.com/setup_24.x | sha256sum
ARG NODESOURCE_SETUP_SHA256=6e3d580f5bd7ccf2aa1e8df8d35c60d78e873c3ff8beb282c9bebd914904ad72
RUN apt-get update -qq \
 && apt-get install -y -qq --no-install-recommends \
        ca-certificates curl gnupg \
 && curl -fsSL -o /tmp/nodesource_setup.sh https://deb.nodesource.com/setup_24.x \
 && echo "${NODESOURCE_SETUP_SHA256}  /tmp/nodesource_setup.sh" | sha256sum -c - \
 && bash /tmp/nodesource_setup.sh \
 && rm -f /tmp/nodesource_setup.sh \
 && apt-get install -y -qq --no-install-recommends nodejs \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /work

# Copy *only* the dependency manifests first so the npm-ci layer cache is
# keyed off package.json + package-lock.json. Source changes do NOT bust
# this layer.
COPY package.json package-lock.json ./

RUN npm ci --prefer-offline --no-audit --fund=false

# Pull down the Chromium build + the apt deps Playwright needs to run it.
# Pinned to the Playwright version in package-lock.json above; rebuild this
# image whenever @playwright/test is bumped.
RUN npx playwright install --with-deps chromium

# At runtime the project root is bind-mounted over /work (see
# docker-compose.yaml). The anonymous volume at /work/node_modules
# initializes from the contents this image baked in here, preserving the
# npm install across the bind mount.
#
# ENTRYPOINT keeps the static prefix (test runner + reporter) so extra args
# passed to `docker compose run --rm playwright …` are appended naturally,
# e.g. `… playwright tests/configEditor.spec.ts --grep "smoke"`.
ENTRYPOINT ["npx", "playwright", "test", "--reporter=list,junit"]
