# Changelog

## Fork Notice

This project is forked from [syself/cluster-api-provider-hetzner](https://github.com/syself/cluster-api-provider-hetzner) and is now maintained independently under `github.com/Assertive-Yield/cluster-api-provider-hetzner`.

## Changes from upstream

- Upgraded to Cluster API v1.12
- Wipe-disk annotation is no longer auto-removed after provisioning

## Unreleased

### Features

- **HCloud `imageURL` / `imageURLCommand`**: provision HCloud machines from a custom image URL via rescue + controller-mounted command (port of syself CAPH 1.1.x). Mutually exclusive with `imageName`. Requires `HetznerCluster.spec.sshKeys.robotRescueSecretRef` and a binary named `image-url-command-*` under `/shared` on the controller pod.
