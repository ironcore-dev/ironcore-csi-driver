## Deploy *onmetal-csi-driver* 
### Prerequisites
- Access to a Kubernetes cluster ([minikube](https://minikube.sigs.k8s.io/docs/), [kind](https://kind.sigs.k8s.io/) or a real [kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/) cluster)
- [docker](https://docs.docker.com/get-docker/) or it's alternative to build the image 
- [make](https://www.gnu.org/software/make/) - to execute build goals
- [golang](https://golang.org/) - to compile the code
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) - to interact with k8s cluster via CLI
- [onmetal-api](https://github.com/onmetal/onmetal-api/) deployed on the cluster
- kubeconfig of the cluster where onmetal-api deployed
- [cloud-provider-onmetal](https://github.com/onmetal/cloud-provider-onmetal/) deployed on the cluster


    
### Steps to deploy *onmetal-csi-driver*
Clone the repository [onmetal-csi-driver](https://github.com/onmetal/onmetal-csi-driver)

```shell
git clone https://github.com/onmetal/onmetal-csi-driver.git
cd onmetal-csi-driver
```
Create a local docker build
```
make buildlocal
```
Create a namespace
```
kubectl create ns onmetal-csi
```
Create a secret in onmetal-csi namespace with data "target-kubeconfig", "onmetal-kubeconfig" and the "namespace" where machines are running

A sample file is present under "config/samples/kube_secret_template.yaml"
```
kubectl apply -f config/samples/kube_secret_template.yaml -n onmetal-csi
```
> Note 1 : Remember to encode the `kubeconfig` file content and the `namespace` field data to base64 before adding them to the [config/samples/kube_secret_template.yaml](https://github.com/onmetal/onmetal-csi-driver/blob/main/config/samples/kube_secret_template.yaml) file.

> Note 2 : In case of kind or minikube cluster deployment in local target-kubeconfig and onmetal-kubeconfig will be same


Deploy onmetal-csi-driver
```
make deploy
```
Validate CSI driver is Running

```bash
root@node1:~# kubectl get pods -n onmetal-csi
NAME                    READY   STATUS      RESTARTS        AGE
onmetal-csi-driver-0    5/5     Running      0              51s
onmetal-csi-node-mkfs9  2/2     Running      0              29s
```

