## Deploy onmetal-csi-driver manually in a minikube environment

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
The namespace value(csi-test) should be same namespace where the machine is running and Volume will be created

Add annotations to the node (temporary)
```
kubectl annotate node minikube onmetal-machine=csi-machine
kubectl annotate node minikube onmetal-namespace=csi-test
```

> Note: ```onmetal-machine``` and ```onmetal-namespace``` value should be the machine and namespace name respectively where the volume will be published, disk will be mounted to.


Deploy onmetal-csi
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
