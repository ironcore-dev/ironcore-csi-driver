# OnMetal CSI Driver

[![Go Report Card](https://goreportcard.com/badge/github.com/onmetal/onmetal-csi-driver)](https://goreportcard.com/report/github.com/onmetal/onmetal-csi-driver)
[![Test](https://github.com/onmetal/onmetal-csi-driver/actions/workflows/test.yml/badge.svg)](https://github.com/onmetal/onmetal-csi-driver/actions/workflows/test.yml)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://makeapullrequest.com)
[![GitHub License](https://img.shields.io/static/v1?label=License&message=Apache-2.0&color=blue&style=flat-square)](LICENSE)

This document provides an overview of the OnMetal CSI Driver, its components, and usage instructions.

## Overview

The OnMetal CSI Driver is a Kubernetes storage plugin that enables the management of OnMetal volumes as Kubernetes 
Persistent Volumes (PVs). The driver supports dynamic provisioning, mounting, and management of OnMetal volumes.

## Table of Contents

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Development](#development)

## Prerequisites

- Kubernetes cluster v1.20 or later
- Go 1.16 or later for building the driver

## Installation

To install the OnMetal CSI Driver, clone the repository and build the binary using the following command:

```bash
git clone https://github.com/onmetal/onmetal-csi-driver.git
cd onmetal-csi-driver
go build -o onmetal-csi-driver ./cmd
```

## Configuration

The driver can be configured through environment variables and command-line flags.

### Environment Variables

- `X_CSI_MODE`: Set the CSI driver mode. Supported modes are node and controller.
- `KUBE_NODE_NAME`: Set the Kubernetes node name when the driver is running in node mode.
- `VOLUME_NS`: Set the OnMetal driver namespace when the driver is running in controller mode.

### Command-Line Flags

- `--target-kubeconfig`: Path pointing to the target kubeconfig.
- `--onmetal-kubeconfig`: Path pointing to the OnMetal kubeconfig.
- `--driver-name`: Override the default driver name. Default value is `driver.CSIDriverName`.

## Usage

1. Run the OnMetal CSI Driver as a controller:

```bash
export X_CSI_MODE=controller
export VOLUME_NS=my-driver-namespace
./onmetal-csi-driver --target-kubeconfig=/path/to/target/kubeconfig --onmetal-kubeconfig=/path/to/onmetal/kubeconfig
```

2. Run the OnMetal CSI Driver as a node:

```bash
export X_CSI_MODE=node
export KUBE_NODE_NAME=my-node-name
./onmetal-csi-driver --target-kubeconfig=/path/to/target/kubeconfig --onmetal-kubeconfig=/path/to/onmetal/kubeconfig
```

## Development

To contribute to the development of the OnMetal CSI Driver, follow these steps:

1. Fork the repository and clone your fork:

```bash
git clone https://github.com/yourusername/onmetal-csi-driver.git
cd onmetal-csi-driver
```

2. Create a new branch for your changes:

```bash
$ git checkout -b my-new-feature
```

3. Make your changes and commit them to your branch.

4. Push your branch to your fork:

```bash
$ git push origin my-new-feature
```

5. Create a pull request against the original repository.

Remember to keep your fork and branch up-to-date with the original repository before submitting a pull request.

## Feedback and Support

Feedback and contributions are always welcome!

Please report bugs, suggestions or post questions by opening a [Github issue](https://github.com/onmetal/onmetal-csi-driver/issues).
## License

[Apache License 2.0](/LICENSE)
