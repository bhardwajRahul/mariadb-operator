<p align="center" width="100%">
<img src="https://mariadb-operator.github.io/mariadb-operator/assets/mariadb_centered_whitebg.svg" alt="mariadb" width="500"/>
</p>

<p align="center">
<a href="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/ci.yml"><img src="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
<a href="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/release.yml"><img src="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/release.yml/badge.svg" alt="Release"></a>
<a href="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/helm.yml"><img src="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/helm.yml/badge.svg" alt="Helm"></a>
<a href="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/helm-release.yml"><img src="https://github.com/mariadb-operator/mariadb-operator/actions/workflows/helm-release.yml/badge.svg" alt="Helm release"></a>
</p>

<p align="center">
<a href="https://goreportcard.com/report/github.com/mariadb-operator/mariadb-operator"><img src="https://goreportcard.com/badge/github.com/mariadb-operator/mariadb-operator" alt="Go Report Card"></a>
<a href="https://pkg.go.dev/github.com/mariadb-operator/mariadb-operator/v25"><img src="https://pkg.go.dev/badge/github.com/mariadb-operator/mariadb-operator.svg" alt="Go Reference"></a>
<a href="https://r.mariadb.com/join-community-slack"><img alt="Slack" src="https://img.shields.io/badge/slack-join_chat-blue?logo=Slack&label=slack&style=flat"></a>
<a href="https://artifacthub.io/packages/helm/mariadb-operator/mariadb-operator"><img src="https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/mariadb-operator" alt="Artifact Hub"></a>
<a href="https://operatorhub.io/operator/mariadb-operator"><img src="https://img.shields.io/badge/Operator%20Hub-mariadb--operator-red" alt="Operator Hub"></a>
</p>

# 🦭 mariadb-operator

Run and operate MariaDB in a cloud native way. Declaratively manage your MariaDB using Kubernetes [CRDs](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) rather than imperative commands.
- [Easily provision](./examples/manifests/mariadb_minimal.yaml) and [configure](./examples/manifests/mariadb_full.yaml) MariaDB servers in Kubernetes.
- Multiple [HA modes](./docs/high_availability.md): Galera Cluster or MariaDB Replication.
- Automated Galera [primary failover](./docs/high_availability.md) and [cluster recovery](./docs/galera.md#galera-cluster-recovery).
- Advanced HA with [MaxScale](./docs/maxscale.md): a sophisticated database proxy, router, and load balancer for MariaDB.
- Flexible [storage](./docs/storage.md) configuration. [Volume expansion](./docs/storage.md#volume-resize).
- [Physical backups](./docs/physical_backup.md) based on [mariadb-backup](https://mariadb.com/docs/server/server-usage/backup-and-restore/mariadb-backup/full-backup-and-restore-with-mariadb-backup) and [Kubernetes VolumeSnapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/).
- [Logical backups](./docs/logical_backup.md) based on [mariadb-dump](https://mariadb.com/docs/server/clients-and-utilities/backup-restore-and-import-clients/mariadb-dump). 
- Multiple [backup storage types](./docs/physical_backup.md#storage-types): S3 compatible, PVCs, Kubernetes volumes and `VolumeSnapshots`.
- Flexible backup configuration: [scheduling](./docs/physical_backup.md#scheduling), [compression](./docs/physical_backup.md#compression), [retention policy](./docs/physical_backup.md#retention-policy), [timeout](./docs/physical_backup.md#timeout), [staging area](./docs/physical_backup.md#staging-area)...
- [Target recovery time](./docs/physical_backup.md#target-recovery-time): restore the closest available backup to the specified time.
- [Bootstrap new instances](./docs/physical_backup.md#restoration) from: Physical backups, logical backups, S3, PVCs, `VolumeSnapshots`...
- [Cluster-aware rolling update](./docs/updates.md#replicasfirstprimarylast): roll out replica Pods one by one, wait for each of them to become ready, and then proceed with the primary Pod, using `ReplicasFirstPrimaryLast`.
- Manual [update strategies](./docs/updates.md#update-strategies): `OnDelete` and `Never`.
- Automated [data-plane updates](./docs/updates.md#auto-update-data-plane).
- [my.cnf change detection](./docs/configuration.md#mycnf). Automatically trigger [updates](./docs/updates.md) when my.cnf changes.
- [Suspend](./docs/suspend.md) operator reconciliation for maintenance operations.
- Issue, configure and rotate [TLS certificates](./docs/tls.md) and CAs.
- Native integration with [cert-manager](https://github.com/cert-manager/cert-manager). Automatically create `Certificate` resources.
- [Prometheus metrics](./docs/metrics.md) via [mysqld-exporter](https://github.com/prometheus/mysqld_exporter) and maxscale-exporter.
- Native integration with [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator). Automatically create `ServiceMonitor` resources.
- Declaratively manage [SQL resources](./docs/sql_resources.md): [users](./examples/manifests/user.yaml), [grants](./examples/manifests/grant.yaml) and logical [databases](./examples/manifests/database.yaml).
- Configure [connections](./examples/manifests/connection.yaml) for your applications.
- Orchestrate and schedule [sql scripts](./examples/manifests/sqljobs).
- Validation webhooks to provide CRD immutability.
- Additional printer columns to report the current CRD status.
- CRDs designed according to the Kubernetes [API conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md).
- Install it using [helm](./docs/helm.md), [OLM](https://operatorhub.io/operator/mariadb-operator) or [static manifests](./deploy/manifests).
- Multiple [deployment modes](./docs/helm.md#deployment-modes): cluster-wide and single namespace.
- Helm chart to deploy [MariaDB clusters](./docs/helm.md#mariadb-cluster-helm-chart) and its associated CRs.
- Multi-arch distroless [image](https://github.com/orgs/mariadb-operator/packages/container/package/mariadb-operator).
- [GitOps](#gitops) friendly.

## Documentation

For the user manual, getting started, guides, and API reference, please see the [**📚docs directory**](./docs/README.md).

## Examples catalog

For example Custom Resources (CRs) demonstrating how to use the operator, refer to the [**🛠️examples directory**](./examples/manifests/).

## Helm installation

You can easily deploy the operator to your cluster by installing the `mariadb-operator-crds` and `mariadb-operator` Helm charts:

```bash
helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
helm install mariadb-operator-crds mariadb-operator/mariadb-operator-crds
helm install mariadb-operator mariadb-operator/mariadb-operator
```

Refer to the [helm documentation](./docs/helm.md) for further detail.

## Upgrading from older releases
When upgrading from an older version of the operator, it’s important to understand how both operator and operand resources are affected.  Ensure you read both the [updates section of the helm docs](https://github.com/mariadb-operator/mariadb-operator/blob/main/docs/helm.md#updates), and the [release notes](https://github.com/mariadb-operator/mariadb-operator/releases) for any additional version-specific steps that may be required. Do not attempt to skip intermediate version upgrades. Upgrade progressively through each version to the next.

## Openshift installation

The Openshift installation is managed separately in the [mariadb-operator-helm](https://github.com/mariadb-operator/mariadb-operator-helm) repository, which contains a [helm based operator](https://sdk.operatorframework.io/docs/building-operators/helm/) that allows you to install `mariadb-operator` via [OLM](https://olm.operatorframework.io/docs/).

## Image compatibility
`mariadb-operator` is only compatible with official MariaDB images. Refer to the [images documentation](./docs/docker.md) for further detail.

## MariaDB compatibility
- MariaDB Community >= 10.6

## MaxScale compatibility
- MaxScale >= 23.08 

## Kubernetes compatibility
- Kubernetes >= 1.31
- OpenShift >= 4.18

## Migrate your MariaDB instance to Kubernetes

This [migration guide](./docs/logical_backup.md#migrating-an-external-mariadb-to-a-mariadb-running-in-kubernetes) will streamline your onboarding process and assist you in migrating your data into a `MariaDB` instance running on Kubernetes.

## GitOps

You can embrace [GitOps](https://opengitops.dev/) best practises by using this operator, just place your CRDs in a git repo and reconcile them with your favorite tool, see an example with [flux](https://fluxcd.io/):
- [Run and operate MariaDB in a GitOps fashion using Flux](./examples/flux/)

## Roadmap

Take a look at our [roadmap](./ROADMAP.md) and feel free to open an issue to suggest new features.

## Adopters

Please create a PR and add your company or project to our [ADOPTERS.md file](./ADOPTERS.md) if you are using our project!

## Contributing

We welcome and encourage contributions to this project! Please check our [contributing](./CONTRIBUTING.md) and [development](./docs/development.md) guides. PRs welcome!

## Community

- [We Tested and Compared 6 Database Operators. The Results are In!](https://www.youtube.com/watch?v=l33pcnQ4cUQ&t=17m25s) - KubeCon EU, March 2024
- [Get Started with MariaDB in Kubernetes and mariadb-operator](https://mariadb.com/resources/blog/get-started-with-mariadb-in-kubernetes-and-mariadb-operator/) - MariaDB Corporation blog, February 2024
- [Run and operate MariaDB in Kubernetes with mariadb-operator](https://mariadb.org/mariadb-in-kubernetes-with-mariadb-operator/) - MariaDB Foundation blog, July 2023
- [L'enfer des DB SQL sur Kubernetes face à la promesse des opérateurs](https://www.youtube.com/watch?v=d_ka7PlWo1I&t=2415s&ab_channel=KCDFrance) - KCD France, March 2023

## Get in touch

Join us on Slack: **[MariaDB Community Slack](https://r.mariadb.com/join-community-slack)**.

## Star history

[![Star history](https://api.star-history.com/svg?repos=mariadb-operator/mariadb-operator&type=Date)](https://star-history.com/#mariadb-operator/mariadb-operator&Date)
