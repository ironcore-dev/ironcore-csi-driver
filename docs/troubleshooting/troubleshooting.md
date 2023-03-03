
## Troubleshooting guide for *`onmetal-csi-driver`*

### CSI driver failed to start
1. Check whether the Kubernetes cluster has required feature-gates enabled for the CSI driver.
    
    For reference- [cloud-provider-onmetal](https://github.com/onmetal/cloud-provider-onmetal/blob/main/docs/deployment/deploy_manually.md#:~:text=Update%20the%20kubelet%20service%20on%20each%20node%20with%20Args%20%2D%2Dcloud%2Dprovider%3Dexternal%20Follow%20below%20steps%20to%20update%20the%20kubelet%20service%20environment%20variable%20KUBELET_KUBECONFIG_ARGS)
    > Note: kind/minikube cluster may not support mount operations.
2. Check whether the correct kubeconfig is provided and accessible from the current cluster.
    ```
    kubectl --kubeconfig=path_to_kubeconfig_of_target_cluster get nodes
    ```
    Example:
    ```
    kubectl --kubeconfig=/home/<USER>/.kube/config get nodes
    NAME         STATUS   ROLES    AGE    VERSION
    csi-master   Ready    master   106m   v1.18.1

    ```
3. Check if the current cluster node has a `providerID` field set.
    ```
    kubectl get node csi-worker -o yaml | grep -i spec -A 3
    ```
    Example:
    ```
    kubectl get node csi-worker -o yaml | grep -i spec -A 3
    spec:
      providerID: onmetal://csi/csi-worker
    status:
      addresses:
    ```

    Verify that the [cloud-provider-onmetal](https://github.com/onmetal/cloud-provider-onmetal/) is properly configured and deployed, as it is responsible for setting the provider ID. Refer to the [`cloud-provider-onmetal` deployment documentation](https://github.com/onmetal/cloud-provider-onmetal/blob/main/docs/deployment/deploy_manually.md) for guidance.
   

4. Check CSI driver is deployed and Running

     ```bash
    kubectl get pods -n onmetal-csi
    NAME                    READY   STATUS      RESTARTS        AGE
    onmetal-csi-driver-0    5/5     Running      0              51s
    onmetal-csi-node-mkfs9  2/2     Running      0              29s
    ```

### Error while creating PVC
1. Check whether volume(s) are available in the target cluster.
2. Check whether the volume is in the targeted namespace.
3. Check whether volume satisfies the required storage requirement of PVC.

   failed pvc request status:
    ```bash
        kubectl get pvc -n onmetal-csi
        NAMESPACE   NAME      STATUS    VOLUME   CAPACITY   ACCESS  MODES   STORAGECLASS                AGE
        csi-test    pvc-demo  Pending                                       onmetal-storageclass-demo   2s
    ```
   error logs for failed pvc request:
    ```bash
        time="2022-05-31T06:02:51Z" level=info msg="create volume request received with volume name csi-onmetal-e022cb7f52"
        time="2022-05-31T06:02:51Z" level=info msg="requested size 1073741824" 
        time="2022-05-31T06:02:51Z" level=info msg="requested size Gi 1Gi"
        time="2022-05-31T06:02:56Z" level=info msg="volume is not satisfied"
    ```
   Ideal onmetal volume:
    ```bash
        kubectl get volume -n onmetal-csi
        NAMESPACE     NAME            VOLUMEPOOLREF       VOLUMECLASS   STATE       PHASE     AGE
        onmetal-csi   volume-sample   volumepool-sample   fast          Available   Bound   8m43s
    ```
   Ideal pvc status:
    ```bash
        kubectl get pvc -n onmetal-csi
        NAMESPACE    NAME      STATUS   VOLUME                  CAPACITY  ACCESS MODES   STORAGECLASS                AGE
        onmetal-csi  pvc-demo  Bound    csi-onmetal-44eb33bc46  1Gi       RWO            onmetal-storageclass-demo   9s
    ```

### Failed to mount volume
1. Check whether machine(s) are available in the target cluster.

    Example error:
    ```bash
        kubectl logs -f onmetal-csi-driver-0 -c driver  -n onmetal-csi 
        time="2022-05-31T06:41:15Z" level=info msg="request received to publish volume csi-onmetal-44eb33bc46 at node 192.168.0.108\n"
        time="2022-05-31T06:41:15Z" level=info msg="get machine with provided name and namespace"
        time="2022-05-31T06:41:15Z" level=error msg="could not get machine with name node1,namespace onmetal-csi, error:machines.compute.api.onmetal.de \"node1\" not found"
    ```
2. Check whether the machine is in the targeted namespace and the machine name and namespace matches the providerID fields.

    providerID format  `{CloudProviderName}://{Namespace}/{MachineName}`
    
    For example, if the providerID is 
    `providerID: onmetal://csi/csi-worker`
    
    This shows a providerID for a machine named "csi-worker" that is provided by the "onmetal" provider in the "csi" namespace.

    Example error:
    
    ```bash
        kubectl logs -f onmetal-csi-driver-0 -c driver  -n onmetal-csi 
        time="2022-05-31T06:41:15Z" level=info msg="request received to publish volume csi-onmetal-44eb33bc46 at node 192.168.0.108\n"
        time="2022-05-31T06:41:15Z" level=info msg="get machine with provided name and namespace"
        time="2022-05-31T06:41:15Z" level=error msg="could not get machine with name node1,namespace onmetal-csi, error:machines.compute.api.onmetal.de \"node1\" not found"
    ```
3. Check whether the disk to mount is available with volume.
    ```bash
        kubectl logs -f onmetal-csi-driver-0 -c driver  -n onmetal-csi 
        time="2022-05-31T06:51:40Z" level=info msg="request received to publish volume csi-onmetal-4c50e230e1 at node 192.168.0.108\n"
        time="2022-05-31T06:51:40Z" level=info msg="get machine with provided name and namespace"
        time="2022-05-31T06:51:40Z" level=info msg="update machine with volumeattachment"
        time="2022-05-31T06:51:40Z" level=info msg="check machine is updated"
        time="2022-05-31T06:51:40Z" level=info msg="could not found device for given volume volume-sample"
        time="2022-05-31T06:51:40Z" level=error msg="unable to get disk to mount"
    ```
4. Check disk to mount is available at /dev/disks/by-id directory.

    Look for the disk by manually entering into the machine at below path `/dev/disk/by-id/`

    Example disk-path format: `/dev/disk/by-id/virtio-odc-e50014ee2b3f4627a`

   Example error:
    ```bash
        kubectl logs -f onmetal-csi-node-n9gjf -c driver -n onmetal-csi
        time="2022-05-31T06:35:13Z" level=error msg="failed to stage volume:format of disk \"/host/dev/disk/by-id/virtio-odc-e50014ee2b3f4627a\" failed: type:(\"ext4\") target:(\"/var/lib/kubelet/plugins/kubernetes.io/csi/onmetal-csi-driver/b6fef28a18a856aa16c7a1201db104c250b95a02e4ec959377f589a096655b4e/globalmount\") options:(\"rw,defaults\") errcode:(exit status 1) output:(mke2fs 1.44.5 (15-Dec-2018)\nThe file /host/dev/disk/by-id/virtio-odc-e50014ee2b3f4627a does not exist and no size was specified.\n) "
    ```
    Ideal onmetal machine:
    ```bash
        kubectl get machine -n onmetal-csi
        NAME    MACHINECLASSREF       IMAGE                   MACHINEPOOLREF       STATE     AGE
        node1   machineclass-sample   myimage_repo_location   machinepool-sample   Running   9m28s
    ```
   Ideal onmetal volume:
    ```bash
        kubectl get volume -n onmetal-csi
        NAMESPACE     NAME            VOLUMEPOOLREF       VOLUMECLASS   STATE       PHASE   AGE
        onmetal-csi   volume-sample   volumepool-sample   fast          Available   Bound   24s
    ```
   Volume status with disk available (Wwn):
    ```bash
       kubectl describe volume volume-sample -n onmetal-csi
        Name:         volume-sample
        Namespace:    onmetal-csi
        ...
        ...
        Status:
        Access:
            Driver:
            Volume Attributes:
            Wwn:                     50014ee2b3f4627a
        Last Phase Transition Time:  2022-05-31T06:56:50Z
        Phase:                       Bound
        State:                       Available
    ```
    Create pod to mount volume:
    ```bash
        kubectl apply -f onmetal-csi-driver/config/samples/pod.yaml -n onmetal-csi
        pod/pod-demo created
    ```

    ```bash
        kubectl get pods pod-demo -n onmetal-csi
        NAME       READY   STATUS    RESTARTS   AGE
        pod-demo   1/1     Running   0          4m57s
    ```
