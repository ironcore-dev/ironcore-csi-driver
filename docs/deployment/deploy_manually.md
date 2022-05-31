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

Create a configmap from literal
```
kubectl create configmap csi-configmap --from-literal=namespace=onmetal-csi -n onmetal-csi
```
Add annotations to the node (temporary)
```
kubectl annotate node minikube onmetal-machine=minikube
kubectl annotate node minikube onmetal-namespace=onmetal-csi
```
Deploy onmetal-csi
```
make deploy
```


