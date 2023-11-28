# Basic use cases

(Assuming the `default` namespace will be used to create all the resources)

## Create volume

- Create a `StorageClass` (SC)

    ```
    kubectl apply -f ironcore-csi-driver/config/samples/storage-class.yaml
    ```

- Create a `PersistentVolumeClaim` (PVC)

    ```
    kubectl apply -f ironcore-csi-driver/config/samples/pvc.yaml
    ```

- Check the `PVC` status

    ```
    kubectl get pvc
    ```

- After creating the `StorageClass` and `PersistentVolumeClaim`, create a `Pod` that uses the `PersistentVolumeClaim` as a volume

    ```
    kubectl apply -f ironcore-csi-driver/config/samples/pod.yaml
    ```

- Check that the volume has been created, staged and published

    ```
    kubectl describe pod pod-demo   | grep Volume
    ```

- Check if the volume is mounted by creating a file

    ```
    kubectl exec -it pod-demo /bin/sh
    cd /tmp/data
    touch test.txt
    ls
    ```

## Expand volume

- To expand the volume, first make sure that the `StorageClass` that the `PVC` is referring to has field `allowVolumeExpansion: true`.
`StorageClass` should also have a `parameters.type` field set to the name of the volume class that supports `ExpandOnly` resize policy

- Update the `PVC` with the desired size by editing the pvc.yaml file present at `ironcore-csi-driver/config/samples/pvc.yaml` then re-apply the `PVC`

    ```
    kubectl apply -f ironcore-csi-driver/config/samples/pvc.yaml
    ```

## Delete volume

- To delete the volume, delete the `Pod` that is using the volume and finally, delete the `PVC`

    ```
    kubectl delete pvc pvc-demo
    kubectl delete pod pod-demo
    ```
