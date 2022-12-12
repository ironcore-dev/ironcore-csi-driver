## Run onmetal-csi-driver locally in a minikube environment

### Prerequisites
- [onmetal-api](https://github.com/onmetal/onmetal-api/) deployed on a cluster
- kubeconfig of the cluster where onmetal-api deployed
- [make](https://www.gnu.org/software/make/) - to execute build goals
- [golang](https://golang.org/) - to compile the code
- [minikube](https://minikube.sigs.k8s.io/) or access to k8s cluster - to deply and test the result
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) - to interact with k8s cluster via CLI

Start a local Kubernetes cluster using minikube
```
minikube start
```
```
eval $(minikube docker-env) 
```
Create a local docker build
```
make buildlocal
```
Create a namespace
```
kubectl create ns onmetal-csi
```
Create a secret for kubeconfig file in onmetal-csi namespace
```
kubectl apply -f config/samples/kube_secret_template.yaml -n onmetal-csi
```

> Please note, you need to encode the kubeconfig file content into base64 before placing it inside config/samples/kube_secret_template.yaml


Create a configmap

```
kubectl create configmap csi-configmap --from-literal=namespace=csi-test -n onmetal-csi
```
> Note: ```namespace=csi-test```
The namespace value(onmetal-csi) should be same namespace where the machine is running and Volume will be created

Add annotations to the node (temporary)
```
kubectl annotate node minikube onmetal-machine=csi-machine
kubectl annotate node minikube onmetal-namespace=csi-test
```

> Note: ```onmetal-machine``` and ```onmetal-namespace``` value should be the machine and namespace name respectively where the volume will be published, disk will be mounted to.


Deploy onmetal-csi-driver
```
make deploy
```
Validate CSI driver is deployed and Running

```bash
root@node1:~# kubectl get pods -n onmetal-csi
NAME                    READY   STATUS      RESTARTS        AGE
onmetal-csi-driver-0    5/5     Running      0              51s
onmetal-csi-node-mkfs9  2/2     Running      0              29s
```

