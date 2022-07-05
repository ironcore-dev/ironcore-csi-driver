## Basic use cases 

(Assuming "csi-test" as namespace where all resources will be created)

- Create volume
```
kubectl apply -f onmetal-api/config/samples/storage_v1alpha1_volume.yaml -n csi-test
```

- Create pvc to use the above volume
```
kubectl apply -f onmetal-csi-driver/config/samples/pvc.yaml -n csi-test
```

- Check status
```
kubectl get volume -A
kubectl get pvc -A
```

- Create onmetal-api machine
```
kubectl apply -f onmetal-api/config/samples/compute_v1alpha1_machine.yaml -n csi-test
```

- Create pod 
```
kubectl apply -f config/samples/pod.yaml -n csi-test
```

- Check if the volume is mount
```
kubectl exec -it pod-demo -n csi-test /bin/sh
cd /tmp/data
touch test.txt
ls
```