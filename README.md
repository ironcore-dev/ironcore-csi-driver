# IronCore CSI Driver

[![REUSE status](https://api.reuse.software/badge/github.com/ironcore-dev/ironcore-csi-driver)](https://api.reuse.software/info/github.com/ironcore-dev/ironcore-csi-driver)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironcore-dev/ironcore-csi-driver)](https://goreportcard.com/report/github.com/ironcore-dev/ironcore-csi-driver)
[![GitHub License](https://img.shields.io/static/v1?label=License&message=Apache-2.0&color=blue)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://makeapullrequest.com)

This document provides an overview of the IronCore CSI Driver, its components, and usage instructions.

## Overview

The IronCore CSI Driver is a Kubernetes storage plugin that enables the management of IronCore volumes as Kubernetes 
Persistent Volumes (PVs). The driver supports dynamic provisioning, mounting, and management of IronCore volumes.

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

To install the IronCore CSI Driver, clone the repository and build the binary using the following command:

```bash
git clone https://github.com/ironcore-dev/ironcore-csi-driver.git
cd ironcore-csi-driver
go build -o ironcore-csi-driver ./cmd
```

## Configuration

The driver can be configured through environment variables and command-line flags.

### Environment Variables

- `X_CSI_MODE`: Set the CSI driver mode. Supported modes are node and controller.
- `KUBE_NODE_NAME`: Set the Kubernetes node name when the driver is running in node mode.
- `VOLUME_NS`: Set the IronCore driver namespace when the driver is running in controller mode.

### Command-Line Flags

- `--target-kubeconfig`: Path pointing to the target kubeconfig.
- `--ironcore-kubeconfig`: Path pointing to the IronCore kubeconfig.
- `--driver-name`: Override the default driver name. Default value is `driver.CSIDriverName`.

## Usage

1. Run the IronCore CSI Driver as a controller:

```bash
export X_CSI_MODE=controller
export VOLUME_NS=my-driver-namespace
./ironcore-csi-driver --target-kubeconfig=/path/to/target/kubeconfig --ironcore-kubeconfig=/path/to/ironcore/kubeconfig
```

2. Run the IronCore CSI Driver as a node:

```bash
export X_CSI_MODE=node
export KUBE_NODE_NAME=my-node-name
./ironcore-csi-driver --target-kubeconfig=/path/to/target/kubeconfig --ironcore-kubeconfig=/path/to/ironcore/kubeconfig
```

## Development

To contribute to the development of the IronCore CSI Driver, follow these steps:

1. Fork the repository and clone your fork:

```bash
git clone https://github.com/yourusername/ironcore-csi-driver.git
cd ironcore-csi-driver
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

Please report bugs, suggestions or post questions by opening a [Github issue](https://github.com/ironcore-dev/ironcore-csi-driver/issues).
## License

[Apache License 2.0](/LICENSE)
