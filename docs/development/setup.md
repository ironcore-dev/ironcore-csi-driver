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
make localbuild
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
Get pods status
```
kubectl get pods -n onmetal-csi
```

