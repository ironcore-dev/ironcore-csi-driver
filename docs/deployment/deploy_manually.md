## Deploy onmetal-csi-driver manually in a minikube environment

#### Run it locally on minikube
Start a local Kubernetes cluster
```shell
minikube start
```
```shell
eval $(minikube docker-env) 
```
Create a local docker build
```shell
make localbuild
```
Create a namespace
```shell
kubectl create ns onmetal-csi
```
Create a secret for kubeconfig file in onmetal-csi namespace
```shell
kubectl apply -f config/samples/kube_secret_template.yaml -n onmetal-csi
```
⚠ please note to encode the kubeconfig file content into base64 before placing it inside config/samples/kube_secret_template.yaml
Create a configmap from literal
```shell
kubectl create configmap csi-configmap --from-literal=namespace=onmetal-csi -n onmetal-csi
```
Add annotations to the node (temporary)
```shell
kubectl annotate node minikube onmetal-machine=minikube
kubectl annotate node minikube onmetal-namespace=onmetal-csi
```
Deploy onmetal-csi
```shell
make deploy
```
Get pods status
```shell
kubectl get pods -n onmetal-csi
```

