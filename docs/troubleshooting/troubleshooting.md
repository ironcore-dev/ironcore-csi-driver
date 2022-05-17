## Troubleshooting guide for onmetal-csi-driver

### CSI driver failed to start
1. check whether kubernetes cluster have required feature-gates are enabled for csi driver
2. check whether correct kubeconfig is provided, and it is accessible from current cluster
3. check cluster node is annotated with required annotations.

### Error while create PVC
1. check whether volume(s) are available in target cluster.
2. check whether volume is in targeted namespace
3. check whether volume satishfies required storage requirement of pvc.

### Failed to mount volume
1. check whether machine(s) are available in target cluster.
2. check whether machine is in targeted namespace and matche name matches node annotation
3. check whether disk to mount is available with volume.
4. check whether required permission and features allowed which are required for mount operation. (kind/minikube cluster may not be support mount operation)
5. check disk to mount is available in /dev/disks/by-id directory.


