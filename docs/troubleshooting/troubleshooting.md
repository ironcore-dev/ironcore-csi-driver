## Troubleshooting guide for onmetal-csi-driver

### CSI driver failed to start
1. Check whether the Kubernetes cluster has required feature-gates enabled for the CSI driver.
2. Check whether the correct kubeconfig is provided, and is accessible from the current cluster.
```
kubectl --kubeconfig=path_to_kubeconfig_of_target_cluster get nodes
```
Example:
```
rohit@ubuntu:~/gitrepo/onmetal$ kubectl --kubeconfig=/home/rohit/.kube/config get nodes
NAME       STATUS   ROLES    AGE    VERSION
minikube   Ready    master   106m   v1.18.1

```
3. Check current cluster node is annotated with required annotations.
```
kubectl describe node node_name | grep -i annotations -A5
```
Example:
```
rohit@ubuntu:~/gitrepo/onmetal$ kubectl describe node minikube | grep -i annotations -A5
Annotations:        kubeadm.alpha.kubernetes.io/cri-socket: /var/run/dockershim.sock
                    node.alpha.kubernetes.io/ttl: 0
                    volumes.kubernetes.io/controller-managed-attach-detach: true
CreationTimestamp:  Thu, 19 May 2022 16:41:09 +0530
Taints:             <none>
Unschedulable:      false
```
To add annotations to the node
```
kubectl annotate node minikube onmetal-machine=minikube
kubectl annotate node minikube onmetal-namespace=onmetal-csi
```
```bash
rohit@ubuntu:~/gitrepo/onmetal$ kubectl describe node minikube | grep -i annotations -A5
Annotations:        kubeadm.alpha.kubernetes.io/cri-socket: /var/run/dockershim.sock
                    node.alpha.kubernetes.io/ttl: 0
                    onmetal-machine: minikube
                    onmetal-namespace: onmetal-csi
                    volumes.kubernetes.io/controller-managed-attach-detach: true
CreationTimestamp:  Thu, 19 May 2022 16:41:09 +0530

```

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