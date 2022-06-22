![Gardener on Metal Logo](docs/assets/logo.png)

# onmetal-csi-driver

 [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](http://makeapullrequest.com) 
[![GitHub License](https://img.shields.io/static/v1?label=License&message=Apache-2.0&color=blue&style=flat-square)](LICENSE)

> CSI driver for Gardener on Metal

## Overview 

The onmetal Container Storage Interface (CSI) driver is a [CSI specification-compliant](https://github.com/onmetal/onmetal-csi-driver/tree/main/docs) driver used by Gardener on Metal to manage the lifecycle of onmetal volumes.

The CSI is a standard for exposing arbitrary block and file storage systems to containerized workloads on Kubernetes. 

This driver provides CSI implementation for Gardener on Metal,
Persistent volumes (PVC) created using this driver will be linked to exiting onmetal volumes by creating onmetal volumes and mounting relevant disks to machine(s) (VMs) created on onmetal.

## Installation, using and developing 

For more details please refer to documentation folder  [/docs](https://github.com/onmetal/onmetal-csi-driver/tree/main/docs)

## Contributing 

We`d love to get feedback from you. 
Please report bugs, suggestions or post questions by opening a [Github issue](https://github.com/onmetal/onmetal-csi-driver/issues)

## License

[Apache License 2.0](/LICENSE)