# Configuration

> [!NOTE]  
> This documentation applies to `mariadb-operator` version >= v0.0.28

This documentation aims to provide guidance on various configuration aspects across many `mariadb-operator` CRs. 

## Table of contents
<!-- toc -->
- [my.cnf](#mycnf)
- [Timezones](#timezones)
- [Passwords](#passwords)
- [External resources](#external-resources)
- [Probes](#probes)
<!-- /toc -->

## my.cnf

An inline [configuration file (my.cnf)](https://mariadb.com/kb/en/configuring-mariadb-with-option-files/) can be provisioned in the `MariaDB` resource via the `myCnf` field:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
  myCnf: |
    [mariadb]
    bind-address=*
    default_storage_engine=InnoDB
    binlog_format=row
    innodb_autoinc_lock_mode=2
    innodb_buffer_pool_size=1024M
    max_allowed_packet=256M 
```
In this field, you may provide any [configuration option](https://mariadb.com/kb/en/mariadbd-options/) or [system variable](https://mariadb.com/kb/en/server-system-variables/) supported by MariaDB.

Under the hood, the operator automatically creates a `ConfigMap` with the contents of  the `myCnf` field, which will be mounted in the `MariaDB` instance. Alternatively, you can manage your own configuration using a pre-existing `ConfigMap` by linking it via `myCnfConfigMapKeyRef`. It is important to note that the key in this `ConfigMap` i.e. the config file name, must have a `.cnf` extension in order to be detected by MariaDB:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
  myCnfConfigMapKeyRef:
    name: mariadb
    key: mycnf
```

To ensure your configuration changes take effect, the operator triggers a [rolling update](./updates.md) whenever the `myCnf` field or a `ConfigMap` is updated. For the operator to detect changes in a `ConfigMap`, it must be labeled with `k8s.mariadb.com/watch`. Refer to the [external resources](#external-resources) section for further detail.

## Timezones

By default, MariaDB does not load timezone data on startup for performance reasons and defaults the timezone to `SYSTEM`, obtaining the timezone information from the environment where it runs. See the [MariaDB docs](https://mariadb.com/kb/en/time-zones/) for further information.

You can explicitly configure a timezone in your `MariaDB` instance by setting the `timeZone` field: 

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  timeZone: "UTC"
```

This setting is immutable and implies loading the timezone data on startup.

In regards to `Backup` and `SqlJob` resources, which get reconciled into `CronJobs`, you can also define a `timeZone` associated with their cron expression:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: Backup
metadata:
  name: backup-scheduled
spec:
  mariaDbRef:
    name: mariadb
  schedule:
    cron: "*/1 * * * *"
    suspend: false
  timeZone: "UTC"
```

If `timeZone` is not provided, the local timezone will be used, as described in the [Kubernetes docs](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#time-zones).

## Passwords

Some CRs require passwords provided as `Secret` references to function properly. For instance, the root password for a `MariaDB` resource:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
``` 

By default, fields like `rootPasswordSecretKeyRef` are optional and defaulted by the operator, resulting in random password generation if not provided:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
    generate: true
``` 

You may choose to explicitly provide a `Secret` reference via `rootPasswordSecretKeyRef` and opt-out from random password generation by either not providing the `generate` field or setting it to `false`: 

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  rootPasswordSecretKeyRef:
    name: mariadb
    key: root-password
    generate: false
``` 

This way, we are telling the operator that we are expecting a `Secret` to be available eventually, enabling the use of GitOps tools to seed the password:
- [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets): The `Secret` is reconciled from a `SealedSecret`, which is decrypted by the sealed-secrets controller.
- [external-secrets](https://github.com/external-secrets/external-secrets): The `Secret` is reconciled fom an `ExternalSecret`, which is read by the external-secrets controller from an external secrets source (Vault, AWS Secrets Manager ...).

## External resources

Many CRs have a references to external resources (i.e. `ConfigMap`, `Secret`) not managed by the operator. 

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb
spec:
  ...
  myCnfConfigMapKeyRef:
    name: mariadb
    key: mycnf
```

These external resources should be labeled with `k8s.mariadb.com/watch` so the operator can watch them and perform reconciliations based on their changes. For example, see the `my.cnf` `ConfigMap`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mariadb
  labels:
    k8s.mariadb.com/watch: ""
data:
  my.cnf: |
    [mariadb]
    bind-address=*
    default_storage_engine=InnoDB
    binlog_format=row
    innodb_autoinc_lock_mode=2
    innodb_buffer_pool_size=1024M
    max_allowed_packet=256M
```

## Probes

Kubernetes probes serve as an inversion of control mechanism, enabling the application to communicate its health status to Kubernetes. This enables Kubernetes to take appropriate actions when the application is unhealthy, such as restarting or stop sending traffic to `Pods`.

> [!IMPORTANT]  
> Make sure you check the [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/) if you are unfamiliar with Kubernetes probes.

Fine tunning of probes for databases running in Kubernetes is critical, you may do so by tweaking the following fields:

```yaml
apiVersion: k8s.mariadb.com/v1alpha1
kind: MariaDB
metadata:
  name: mariadb-galera
spec:
  # Tune your liveness probe accordingly to avoid Pod restarts.
  livenessProbe:
    periodSeconds: 10
    timeoutSeconds: 5

  # Tune your readiness probe accordingly to prevent disruptions in network traffic.
  readinessProbe:
    periodSeconds: 10
    timeoutSeconds: 5

  # Tune your startup probe accordingly to ensure that the SST completes with a large amount of data.
  # failureThreshold × periodSeconds = 30 × 10 = 300s = 5m until the container gets restarted if unhealthy
  startupProbe:
    failureThreshold: 30
    periodSeconds: 10
    timeoutSeconds: 5
```

There isn't an universally correct default value for these thresholds, so we recommend determining your own based on factors like the compute resources, network, storage, and other aspects of the environment where your `MariaDB` and `MaxScale` instances are running.