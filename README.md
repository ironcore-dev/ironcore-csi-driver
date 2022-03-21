# onmetal-csi-driver

## Overview

onmetal-csi-driver implements csi implementation for onmetal-api

## Installation, Usage and Development

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
Apply kubeconfig secret on -n onmetal-csi 
```shell
kubectl apply -f config/examples/kube_secret_template.yaml -n onmetal-csi
```
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

