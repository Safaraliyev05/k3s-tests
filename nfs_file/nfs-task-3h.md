# Kubernetes NFS Self-Hosted Setup - 3 Hour Task

## Prerequisites
- Docker Desktop with Kubernetes enabled
- Basic understanding of Kubernetes (Pods, Services, Deployments)
- Familiarity with PV and PVC concepts
- kubectl configured to work with Docker Desktop Kubernetes cluster
- Basic understanding of NFS protocol

## Objectives
By the end of this task, you will be able to:
1. Deploy and configure a self-hosted NFS server within Kubernetes
2. Create PersistentVolumes and PersistentVolumeClaims using NFS
3. Deploy stateful applications that use NFS storage
4. Understand NFS volume provisioning and access modes
5. Troubleshoot common NFS connectivity issues in Kubernetes

## Time Allocation
- **Hour 1**: NFS Server Setup and Configuration
- **Hour 2**: PV/PVC Creation and Basic Applications
- **Hour 3**: Advanced Scenarios and Troubleshooting

---

## Hour 1: NFS Server Setup and Configuration

### Task 1.1: Verify Kubernetes Cluster (5 minutes)

Verify your Kubernetes cluster is running:

```bash
kubectl cluster-info
kubectl get nodes
kubectl get namespaces
```

**Expected Output**: Single node (Docker Desktop) should be Ready.

### Task 1.2: Create Namespace for NFS (5 minutes)

Create a dedicated namespace for NFS infrastructure:

```bash
kubectl create namespace nfs-system
kubectl get namespaces
```

### Task 1.3: Deploy NFS Server (20 minutes)

Create an NFS server deployment. The NFS server will run as a Deployment with a Service.

Create `nfs-server.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nfs-server
  namespace: nfs-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nfs-server
  template:
    metadata:
      labels:
        app: nfs-server
    spec:
      containers:
      - name: nfs-server
        image: erichough/nfs-server:latest
        ports:
        - name: nfs
          containerPort: 2049
        - name: mountd
          containerPort: 20048
        - name: rpcbind
          containerPort: 111
        securityContext:
          privileged: true
        env:
        - name: NFS_EXPORT_0_PATH
          value: /exports
        - name: NFS_EXPORT_0_OPTS
          value: "*(rw,no_root_squash,no_subtree_check,sync)"
        volumeMounts:
        - name: nfs-export
          mountPath: /exports
      volumes:
      - name: nfs-export
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: nfs-server
  namespace: nfs-system
spec:
  type: ClusterIP
  ports:
  - name: nfs
    port: 2049
    targetPort: 2049
  - name: mountd
    port: 20048
    targetPort: 20048
  - name: rpcbind
    port: 111
    targetPort: 111
  selector:
    app: nfs-server
```

**Why ClusterIP with DNS?**
- NFS volumes are mounted internally by pods within the cluster
- ClusterIP provides stable internal access without external exposure
- Using DNS service name (`nfs-server.nfs-system.svc.cluster.local`) instead of IP ensures reliability
- Service DNS name is stable and doesn't change when pods restart
- No need for NodePort or LoadBalancer since NFS is cluster-internal only
- More secure (no external network exposure) and follows Kubernetes best practices

Deploy the NFS server:

```bash
kubectl apply -f nfs-server.yaml
kubectl get pods -n nfs-system -w
```

Wait for the pod to be Running:

```bash
kubectl wait --for=condition=ready pod -l app=nfs-server -n nfs-system --timeout=120s
```

**Note on NFS Server Image:**
The task uses `erichough/nfs-server:latest` which is a stable, well-maintained NFS server image. If you encounter issues:

**Common Errors:**
- **"please provide /etc/exports... or set NFS_EXPORT_*"**: The `erichough/nfs-server` image requires `NFS_EXPORT_0_PATH` and `NFS_EXPORT_0_OPTS` environment variables (not `SHARED_DIRECTORY`)
- **"assertion failed" or "Failed to create temporary file"**: Try a different NFS server image
- **Image pull errors**: Verify internet connectivity and Docker Hub access

**Alternative Images:**
- `gists/nfs-server:latest` - Minimal NFS server implementation
- `itsthenetwork/nfs-server-alpine:latest` - Alpine-based (may have compatibility issues)

If you need to test image pull manually:
```bash
docker pull erichough/nfs-server:latest
```

### Task 1.4: Verify NFS Server (10 minutes)

Get the NFS server pod name and verify it's working:

```bash
NFS_POD=$(kubectl get pod -n nfs-system -l app=nfs-server -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $NFS_POD -n nfs-system -- sh

# Inside the pod, verify NFS is running
rpcinfo -p localhost
exit
```

Get the NFS server Service DNS name:

```bash
NFS_SERVER_DNS="nfs-server.nfs-system.svc.cluster.local"
echo "NFS Server DNS: $NFS_SERVER_DNS"

# Verify service is accessible
kubectl get svc nfs-server -n nfs-system
```

Verify NFS export from another pod:

```bash
kubectl run test-nfs --image=busybox --rm -it --restart=Never -- sh

# Inside the test pod
apk add --no-cache nfs-utils
showmount -e nfs-server.nfs-system.svc.cluster.local
exit
```

### Task 1.5: Create PersistentVolume for NFS (15 minutes)

Create `nfs-pv.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nfs-pv
spec:
  capacity:
    storage: 10Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
    - ReadWriteOnce
    - ReadOnlyMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""
  nfs:
    server: nfs-server.nfs-system.svc.cluster.local
    path: /
```

Apply and verify:

```bash
kubectl apply -f nfs-pv.yaml
kubectl get pv nfs-pv
```

**Check**: PV should be Available.

---

## Hour 2: PV/PVC Creation and Basic Applications

### Task 2.1: Create PersistentVolumeClaim (10 minutes)

Create `nfs-pvc.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nfs-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  volumeName: nfs-pv
```

Apply and verify binding:

```bash
kubectl apply -f nfs-pvc.yaml
kubectl get pvc nfs-pvc
kubectl get pv nfs-pv
```

**Check**: PVC should be Bound, PV should be Bound to the PVC.

### Task 2.2: Deploy Application with NFS Storage (20 minutes)

Create a web application that writes data to NFS. Create `web-app-nfs.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web-app
  template:
    metadata:
      labels:
        app: web-app
    spec:
      containers:
      - name: web-server
        image: nginx:alpine
        ports:
        - containerPort: 80
        volumeMounts:
        - name: nfs-storage
          mountPath: /usr/share/nginx/html
      volumes:
      - name: nfs-storage
        persistentVolumeClaim:
          claimName: nfs-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: web-app-service
spec:
  selector:
    app: web-app
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80
  type: LoadBalancer
```

Create an index page on NFS:

```bash
kubectl run init-nfs --image=busybox --rm -it --restart=Never --overrides='
{
  "spec": {
    "containers": [{
      "name": "init-nfs",
      "image": "busybox",
      "command": ["sh", "-c", "echo '<h1>NFS Shared Storage</h1><p>Deployed: '$(date)'</p>' > /mnt/index.html"],
      "volumeMounts": [{
        "mountPath": "/mnt",
        "name": "nfs-storage"
      }]
    }],
    "volumes": [{
      "name": "nfs-storage",
      "persistentVolumeClaim": {
        "claimName": "nfs-pvc"
      }
    }]
  }
}'
```

Deploy the web application:

```bash
kubectl apply -f web-app-nfs.yaml
kubectl get pods -l app=web-app
kubectl get svc web-app-service
```

Verify all pods can read the same file:

```bash
kubectl exec deployment/web-app -- cat /usr/share/nginx/html/index.html
```

### Task 2.3: Test ReadWriteMany Access Mode (15 minutes)

Create multiple pods writing to the same NFS volume simultaneously:

Create `nfs-writers.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nfs-writers
spec:
  replicas: 5
  selector:
    matchLabels:
      app: nfs-writer
  template:
    metadata:
      labels:
        app: nfs-writer
    spec:
      containers:
      - name: writer
        image: busybox
        command:
        - sh
        - -c
        - |
          while true; do
            echo "$(date) - Pod $(hostname) wrote this" >> /data/writes.log
            sleep 5
          done
        volumeMounts:
        - name: nfs-storage
          mountPath: /data
      volumes:
      - name: nfs-storage
        persistentVolumeClaim:
          claimName: nfs-pvc
```

Deploy and verify concurrent writes:

```bash
kubectl apply -f nfs-writers.yaml
sleep 30
kubectl exec deployment/nfs-writers -- cat /data/writes.log
```

**Expected**: Log file should contain entries from all 5 pods.

### Task 2.4: Scale Application and Verify Data Persistence (10 minutes)

Scale the web application and verify all pods access the same data:

```bash
kubectl scale deployment web-app --replicas=5
kubectl get pods -l app=web-app
```

Update the shared index file and verify all pods see the change:

```bash
kubectl run update-nfs --image=busybox --rm -it --restart=Never --overrides='
{
  "spec": {
    "containers": [{
      "name": "update-nfs",
      "image": "busybox",
      "command": ["sh", "-c", "echo '<h1>Updated Content</h1><p>All pods share this!</p>' > /mnt/index.html && cat /mnt/index.html"],
      "volumeMounts": [{
        "mountPath": "/mnt",
        "name": "nfs-storage"
      }]
    }],
    "volumes": [{
      "name": "nfs-storage",
      "persistentVolumeClaim": {
        "claimName": "nfs-pvc"
      }
    }]
  }
}'
```

Verify all web-app pods see the updated content:

```bash
for pod in $(kubectl get pods -l app=web-app -o jsonpath='{.items[*].metadata.name}'); do
  echo "Pod: $pod"
  kubectl exec $pod -- cat /usr/share/nginx/html/index.html
  echo "---"
done
```

---

## Hour 3: Advanced Scenarios and Troubleshooting

### Task 3.1: Multiple PVCs with Storage Classes (20 minutes)

Create a StorageClass for dynamic provisioning (simulated with manual PV creation pattern):

Create `nfs-storageclass.yaml`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nfs-storage
provisioner: example.com/nfs
parameters:
  archiveOnDelete: "false"
```

Create multiple PVs and PVCs:

Create `nfs-multi-pv.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nfs-pv-1
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  nfs:
    server: nfs-server.nfs-system.svc.cluster.local
    path: /
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nfs-pv-2
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  nfs:
    server: nfs-server.nfs-system.svc.cluster.local
    path: /
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nfs-pvc-1
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nfs-pvc-2
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
```

Apply and bind manually:

```bash
kubectl apply -f nfs-multi-pv.yaml
kubectl patch pv nfs-pv-1 -p '{"spec":{"claimRef":{"name":"nfs-pvc-1","namespace":"default"}}}'
kubectl patch pv nfs-pv-2 -p '{"spec":{"claimRef":{"name":"nfs-pvc-2","namespace":"default"}}}'
kubectl get pv
kubectl get pvc
```

### Task 3.2: Stateful Application with NFS (25 minutes)

Deploy a database-like application using NFS for persistence:

Create `app-stateful-nfs.yaml`:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: data-processor
spec:
  serviceName: data-processor
  replicas: 3
  selector:
    matchLabels:
      app: data-processor
  template:
    metadata:
      labels:
        app: data-processor
    spec:
      containers:
      - name: processor
        image: busybox
        command:
        - sh
        - -c
        - |
          mkdir -p /data/processor-$(hostname)
          while true; do
            echo "$(date) - Data from $(hostname)" >> /data/processor-$(hostname)/data.log
            sleep 10
          done
        volumeMounts:
        - name: nfs-storage
          mountPath: /data
      volumes:
      - name: nfs-storage
        persistentVolumeClaim:
          claimName: nfs-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: data-processor
spec:
  selector:
    app: data-processor
  ports:
  - port: 8080
    name: http
  clusterIP: None
```

Deploy and verify:

```bash
kubectl apply -f app-stateful-nfs.yaml
kubectl get statefulset data-processor
kubectl get pods -l app=data-processor
sleep 20
kubectl exec data-processor-0 -- ls -la /data/
```

Delete a pod and verify data persistence:

```bash
kubectl delete pod data-processor-0
kubectl get pods -l app=data-processor
sleep 15
kubectl exec data-processor-0 -- cat /data/processor-data-processor-0/data.log
```

### Task 3.3: Troubleshooting NFS Connectivity (15 minutes)

Test NFS mount from a pod:

```bash
kubectl run nfs-test --image=busybox --rm -it --restart=Never --overrides='
{
  "spec": {
    "containers": [{
      "name": "nfs-test",
      "image": "busybox",
      "command": ["sh"],
      "stdin": true,
      "tty": true,
      "volumeMounts": [{
        "mountPath": "/mnt/nfs",
        "name": "nfs-vol"
      }]
    }],
    "volumes": [{
      "name": "nfs-vol",
      "nfs": {
        "server": "nfs-server.nfs-system.svc.cluster.local",
        "path": "/"
      }
    }]
  }
}'
```

Inside the pod, test:

```bash
ls -la /mnt/nfs
echo "test" > /mnt/nfs/test.txt
cat /mnt/nfs/test.txt
exit
```

Common troubleshooting commands:

```bash
# Check NFS server pod logs
kubectl logs -n nfs-system deployment/nfs-server

# Check pod status and events for image pull errors
kubectl describe pod -n nfs-system -l app=nfs-server

# If image pull fails, check cluster internet connectivity
kubectl run test-connectivity --image=busybox --rm -it --restart=Never -- sh -c "wget -O- https://hub.docker.com"

# Verify NFS service
kubectl get svc -n nfs-system nfs-server

# Check PV/PVC status
kubectl describe pv nfs-pv
kubectl describe pvc nfs-pvc

# Check pod events
kubectl describe pod <pod-name>

# Verify NFS server is accessible
kubectl run nfs-check --image=busybox --rm -it --restart=Never -- sh -c "apk add --no-cache nfs-utils && showmount -e nfs-server.nfs-system.svc.cluster.local"
```

**Image Pull Issues:**
If you encounter `ImagePullBackOff` or `ErrImagePull` errors:
1. Verify Docker Desktop has internet access
2. Try pulling the image manually: `docker pull erichough/nfs-server:latest`
3. Check if your network blocks Docker Hub access (corporate proxy/VPN)

**NFS Server Startup Failures:**
If you see errors like:
- **"please provide /etc/exports to the container or set at least one NFS_EXPORT_* environment variable"**: 
  - Ensure `NFS_EXPORT_0_PATH` and `NFS_EXPORT_0_OPTS` environment variables are set correctly
  - For `erichough/nfs-server`, these are required (not `SHARED_DIRECTORY`)
- **"assertion failed" or "Failed to create temporary file"**: 
  - Check pod logs: `kubectl logs -n nfs-system deployment/nfs-server`
  - Try a different NFS server image: `gists/nfs-server:latest`
  - Ensure the pod has `privileged: true` in securityContext
  - Verify the `/exports` directory exists and is writable in the container

### Task 3.4: NFS Performance Testing (15 minutes)

Create a performance test pod:

Create `nfs-performance-test.yaml`:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: nfs-performance-test
spec:
  template:
    spec:
      containers:
      - name: tester
        image: busybox
        command:
        - sh
        - -c
        - |
          echo "Starting write test..."
          time dd if=/dev/zero of=/data/testfile bs=1M count=100
          echo "Starting read test..."
          time dd if=/data/testfile of=/dev/null bs=1M
          ls -lh /data/testfile
          rm /data/testfile
          echo "Test completed"
        volumeMounts:
        - name: nfs-storage
          mountPath: /data
      volumes:
      - name: nfs-storage
        persistentVolumeClaim:
          claimName: nfs-pvc
      restartPolicy: Never
```

Run the performance test:

```bash
kubectl apply -f nfs-performance-test.yaml
kubectl wait --for=condition=complete job/nfs-performance-test --timeout=300s
kubectl logs job/nfs-performance-test
```

### Task 3.5: Cleanup and Verification (5 minutes)

Clean up all resources (optional, for practice):

```bash
# Delete applications
kubectl delete -f web-app-nfs.yaml
kubectl delete -f nfs-writers.yaml
kubectl delete -f app-stateful-nfs.yaml
kubectl delete job nfs-performance-test

# Delete PVCs
kubectl delete pvc nfs-pvc nfs-pvc-1 nfs-pvc-2

# Delete PVs
kubectl delete pv nfs-pv nfs-pv-1 nfs-pv-2

# Delete NFS server
kubectl delete -f nfs-server.yaml

# Delete namespace
kubectl delete namespace nfs-system
```

---

## Verification Checklist

Before completing the task, verify:

- [ ] NFS server pod is Running in nfs-system namespace
- [ ] NFS service is accessible via DNS: nfs-server.nfs-system.svc.cluster.local
- [ ] PV is created and Available
- [ ] PVC is Bound to PV
- [ ] Web application pods can read/write to NFS
- [ ] Multiple pods can write to the same NFS volume simultaneously
- [ ] Data persists after pod deletion and recreation
- [ ] StatefulSet pods can access NFS storage
- [ ] NFS mount works directly from pods using NFS volume type

## Key Learnings

1. **NFS in Kubernetes**: NFS can be self-hosted within the cluster using a containerized NFS server
2. **PV/PVC Binding**: Manual binding requires matching accessModes and storage capacity
3. **ReadWriteMany**: NFS supports multiple pods reading/writing simultaneously
4. **Service Discovery**: ClusterIP with DNS (service FQDN) is preferred over IP addresses - provides stable, reliable access that works across cluster changes; DNS names like `service.namespace.svc.cluster.local` are resolved automatically by Kubernetes
5. **Persistence**: NFS volumes persist data across pod lifecycles
6. **Troubleshooting**: Check pod logs, service endpoints, and PV/PVC status

## Additional Challenges (Optional)

1. Configure NFS server with multiple export paths
2. Set up NFS authentication using Kerberos
3. Implement NFS server high availability with multiple replicas
4. Create an NFS provisioner for dynamic volume provisioning
5. Configure NFS with specific mount options (e.g., nfsvers, sync, hard)

## Resources

- Kubernetes Persistent Volumes: https://kubernetes.io/docs/concepts/storage/persistent-volumes/
- NFS Volume Plugin: https://kubernetes.io/docs/concepts/storage/volumes/#nfs
- Docker Desktop Kubernetes: https://docs.docker.com/desktop/kubernetes/

---

**Task Duration**: 3 hours  
**Difficulty**: Intermediate  
**Prerequisites**: Kubernetes basics, PV/PVC understanding

