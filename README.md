# onmetal-csi-driver

[![Go Report Card](https://goreportcard.com/badge/github.com/onmetal/onmetal-csi-driver)](https://goreportcard.com/report/github.com/onmetal/onmetal-csi-driver)
[![Test](https://github.com/onmetal/onmetal-csi-driver/actions/workflows/test.yml/badge.svg)](https://github.com/onmetal/onmetal-csi-driver/actions/workflows/test.yml)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://makeapullrequest.com)
[![GitHub License](https://img.shields.io/static/v1?label=License&message=Apache-2.0&color=blue&style=flat-square)](LICENSE)

CSI driver for Gardener on Metal

## Overview 

The onmetal Container Storage Interface (CSI) driver is a [CSI specification-compliant](https://github.com/onmetal/onmetal-csi-driver/tree/main/docs) 
driver used by Gardener on Metal to manage the lifecycle of onmetal volumes. The CSI is a standard for exposing 
arbitrary block and file storage systems to containerized workloads on Kubernetes. 

This driver provides CSI implementation for Gardener on Metal. For any Persistent volume (PVC) created using this 
driver, it will create a new onmetal volume and mount the relevant disks to machine(s) (VMs).

## Installation, Usage, and Development

For more details please refer to documentation folder  [/docs](https://github.com/onmetal/onmetal-csi-driver/tree/main/docs)

## Feedback and Support

Feedback and contributions are always welcome!

Please report bugs, suggestions or post questions by opening a [Github issue](https://github.com/onmetal/onmetal-csi-driver/issues).
## License

[Apache License 2.0](/LICENSE)
