## Troubleshooting guide for onmetal-csi-driver

### CSI driver failed to start
1. Check whether the Kubernetes cluster has required feature-gates enabled for the CSI driver.
2. Check whether the correct kubeconfig is provided, and is accessible from the current cluster.
3. Check cluster node is annotated with required annotations.

### Error while creating PVC
1. Check whether volume(s) are available in the target cluster.
2. Check whether the volume is in the targeted namespace.
3. Check whether volume satisfies the required storage requirement of PVC.

### Failed to mount volume
1. Check whether machine(s) are available in the target cluster.
2. Check whether the machine is in the targeted namespace and the name matches node annotation.
3. Check whether the disk to mount is available with volume.
4. Check whether required permissions and features required for mount operation are allowed (kind/minikube cluster may not support mount operation).
5. Check disk to mount is available at /dev/disks/by-id directory.