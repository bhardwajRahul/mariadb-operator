package builder

import (
	"testing"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	discoveryv1 "k8s.io/api/discovery/v1"

	"k8s.io/apimachinery/pkg/types"
)

func TestEndpointsMeta(t *testing.T) {
	builder := newDefaultTestBuilder(t)
	key := types.NamespacedName{
		Name: "endpoints",
	}
	addressType := discoveryv1.AddressTypeIPv4
	endpoints := []discoveryv1.Endpoint{}
	ports := []discoveryv1.EndpointPort{}
	serviceName := "test"
	tests := []struct {
		name     string
		mariadb  *mariadbv1alpha1.MariaDB
		wantMeta *mariadbv1alpha1.Metadata
	}{
		{
			name:    "no meta",
			mariadb: &mariadbv1alpha1.MariaDB{},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					metadata.KubernetesEndpointSliceManagedByLabel: metadata.KubernetesEndpointSliceManagedByValue,
					metadata.KubernetesServiceLabel:                serviceName,
				},
				Annotations: map[string]string{},
			},
		},
		{
			name: "meta",
			mariadb: &mariadbv1alpha1.MariaDB{
				Spec: mariadbv1alpha1.MariaDBSpec{
					InheritMetadata: &mariadbv1alpha1.Metadata{
						Labels: map[string]string{
							"database.myorg.io": "mariadb",
						},
						Annotations: map[string]string{
							"database.myorg.io": "mariadb",
						},
					},
				},
			},
			wantMeta: &mariadbv1alpha1.Metadata{
				Labels: map[string]string{
					metadata.KubernetesEndpointSliceManagedByLabel: metadata.KubernetesEndpointSliceManagedByValue,
					metadata.KubernetesServiceLabel:                serviceName,
					"database.myorg.io":                            "mariadb",
				},
				Annotations: map[string]string{
					"database.myorg.io": "mariadb",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints, err := builder.BuildEndpointSlice(key, tt.mariadb, addressType, endpoints, ports, serviceName)
			if err != nil {
				t.Fatalf("unexpected error building Endpoints: %v", err)
			}
			assertObjectMeta(t, &endpoints.ObjectMeta, tt.wantMeta.Labels, tt.wantMeta.Annotations)
		})
	}
}
