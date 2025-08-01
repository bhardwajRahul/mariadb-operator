package sql

import (
	"context"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	sqlClient "github.com/mariadb-operator/mariadb-operator/v25/pkg/sql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Resource interface {
	metav1.Object
	MariaDBRef() *mariadbv1alpha1.MariaDBRef
	IsBeingDeleted() bool
	RequeueInterval() *metav1.Duration
	RetryInterval() *metav1.Duration
	CleanupPolicy() *mariadbv1alpha1.CleanupPolicy
}

type Reconciler interface {
	Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error)
}

type WrappedReconciler interface {
	Reconcile(context.Context, *sqlClient.Client) error
	PatchStatus(context.Context, condition.Patcher) error
}

type Finalizer interface {
	AddFinalizer(context.Context) error
	Finalize(context.Context, Resource) (ctrl.Result, error)
}

type WrappedFinalizer interface {
	AddFinalizer(context.Context) error
	RemoveFinalizer(context.Context) error
	ContainsFinalizer() bool
	Reconcile(context.Context, *sqlClient.Client) error
}
