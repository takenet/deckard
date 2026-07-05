# Security Policy

## Supported Versions

This project has been used in the production environment for the past two years, handling thousands of requests per second and millions of messages with great success.

However, in order to open source the project and make it available to a wider audience, we had to make some changes.

Deckard is currently in the `1.0.0` release-candidate phase.

We are maintaining the release-candidate line while we finalize hardening for high-scale workloads, especially Redis Cluster operation and housekeeper resilience, before the final `1.0.0` release.

The following versions are supported and will keep getting security updates.

| Version | Supported          |
| ------- | ------------------ |
| 1.0.0-rc.x | :white_check_mark: |
| 0.1.x | :white_check_mark: |

## Reporting a Vulnerability

Please open a issue [here](https://github.com/takenet/deckard/issues) and add the security label. Security will always be our priority.
