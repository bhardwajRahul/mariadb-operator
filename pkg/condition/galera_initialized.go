package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetGaleraInitialized(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeGaleraInitialized,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonGaleraInitialized,
		Message: "Galera initialized",
	})
}

func SetGaleraInitializing(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeGaleraInitialized,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonGaleraInitialized,
		Message: "Galera initializing",
	})
}
