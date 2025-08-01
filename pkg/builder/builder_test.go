package builder

import (
	"reflect"
	"testing"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func newTestBuilder(discovery *discovery.Discovery) *Builder {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(volumesnapshotv1.AddToScheme(scheme))

	env := &environment.OperatorEnv{
		MariadbOperatorName:      "mariadb-operator",
		MariadbOperatorNamespace: "test",
		MariadbOperatorSAPath:    "/var/run/secrets/kubernetes.io/serviceaccount/token",
		MariadbOperatorImage:     "mariadb-operator:test",
		RelatedMariadbImage:      "mariadb:test",
		RelatedMaxscaleImage:     "maxscale:test",
		RelatedExporterImage:     "mysql-exporter:test",
		MariadbGaleraLibPath:     "/usr/lib/galera/libgalera_smm.so",
		WatchNamespace:           "",
	}
	builder := NewBuilder(scheme, env, discovery)

	return builder
}

func newDefaultTestBuilder(t *testing.T) *Builder {
	discovery, err := discovery.NewFakeDiscovery()
	if err != nil {
		t.Fatalf("unexpected error creating discovery: %v", err)
	}
	return newTestBuilder(discovery)
}

func assertObjectMeta(t *testing.T, objMeta *metav1.ObjectMeta, wantLabels, wantAnnotations map[string]string) {
	if objMeta == nil {
		t.Fatal("expecting object metadata to not be nil")
	}
	if !reflect.DeepEqual(wantLabels, objMeta.Labels) {
		t.Errorf("unexpected labels, want: %v  got: %v", wantLabels, objMeta.Labels)
	}
	if !reflect.DeepEqual(wantAnnotations, objMeta.Annotations) {
		t.Errorf("unexpected annotations, want: %v  got: %v", wantAnnotations, objMeta.Annotations)
	}
}

func assertMeta(t *testing.T, meta *mariadbv1alpha1.Metadata, wantLabels, wantAnnotations map[string]string) {
	if meta == nil {
		t.Fatal("expecting metadata to not be nil")
	}
	if !reflect.DeepEqual(wantLabels, meta.Labels) {
		t.Errorf("unexpected labels, want: %v  got: %v", wantLabels, meta.Labels)
	}
	if !reflect.DeepEqual(wantAnnotations, meta.Annotations) {
		t.Errorf("unexpected annotations, want: %v  got: %v", wantAnnotations, meta.Annotations)
	}
}
