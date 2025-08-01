package conditions

import (
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetRestoringBackup(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonRestoreBackup,
		Message: "Restoring backup",
	})
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeBackupRestored,
		Status:  metav1.ConditionFalse,
		Reason:  mariadbv1alpha1.ConditionReasonRestoreBackup,
		Message: "Restoring backup",
	})
}

func SetRestoredBackup(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeBackupRestored,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonRestoreBackup,
		Message: "Restored backup",
	})
}

func SetRestoredPhysicalBackup(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    mariadbv1alpha1.ConditionTypeBackupRestored,
		Status:  metav1.ConditionTrue,
		Reason:  mariadbv1alpha1.ConditionReasonRestorePhysicalBackup,
		Message: "Restored physical backup",
	})
}
