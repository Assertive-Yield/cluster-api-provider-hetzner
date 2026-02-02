# Cluster API Provider Hetzner

A Kubernetes Cluster API infrastructure provider for [Hetzner](https://www.hetzner.com/).

> **Note:** This is a fork of [syself/cluster-api-provider-hetzner](https://github.com/syself/cluster-api-provider-hetzner), now maintained independently. See [CHANGELOG.md](CHANGELOG.md) for details.

## What is CAPH?

The Cluster API Provider Hetzner (CAPH) allows you to declaratively create and manage Kubernetes clusters on Hetzner infrastructure using the Kubernetes API.

Key benefits:

- **Self-healing**: Controllers react to infrastructure changes and resolve issues automatically
- **Declarative**: Specify desired state and let the operators handle the rest
- **Kubernetes native**: Everything is a Kubernetes resource

## Compatibility

| CAPH Version | Cluster API | Kubernetes |
|--------------|-------------|------------|
| v1.0.x       | v1.12       | 1.28 - 1.33 |

## Documentation

Documentation is available in the `/docs` directory.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
