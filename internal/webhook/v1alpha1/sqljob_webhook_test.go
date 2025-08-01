package v1alpha1

import (
	"github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("v1alpha1.SqlJob webhook", func() {
	Context("When creating a v1alpha1.SqlJob", func() {
		objMeta := metav1.ObjectMeta{
			Name:      "sqljob-create-webhook",
			Namespace: testNamespace,
		}
		DescribeTable(
			"Should validate",
			func(s *v1alpha1.SqlJob, wantErr bool) {
				_ = k8sClient.Delete(testCtx, s)
				err := k8sClient.Create(testCtx, s)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"No SQL",
				&v1alpha1.SqlJob{
					ObjectMeta: objMeta,
					Spec: v1alpha1.SqlJobSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
						PasswordSecretKeyRef: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "foo",
							},
							Key: "foo",
						},
					},
				},
				true,
			),
			Entry(
				"Invalid schedule",
				&v1alpha1.SqlJob{
					ObjectMeta: objMeta,
					Spec: v1alpha1.SqlJobSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Schedule: &v1alpha1.Schedule{
							Cron: "foo",
						},
						Username: "foo",
						PasswordSecretKeyRef: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "foo",
							},
							Key: "foo",
						},
					},
				},
				true,
			),
			Entry(
				"Invalid history limits",
				&v1alpha1.SqlJob{
					ObjectMeta: objMeta,
					Spec: v1alpha1.SqlJobSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Schedule: &v1alpha1.Schedule{
							Cron: "foo",
						},
						Username: "foo",
						PasswordSecretKeyRef: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "foo",
							},
							Key: "foo",
						},
						Sql: func() *string { s := "foo"; return &s }(),
						CronJobTemplate: v1alpha1.CronJobTemplate{
							SuccessfulJobsHistoryLimit: ptr.To[int32](-5),
							FailedJobsHistoryLimit:     ptr.To[int32](-5),
						},
					},
				},
				true,
			),
			Entry(
				"Valid",
				&v1alpha1.SqlJob{
					ObjectMeta: objMeta,
					Spec: v1alpha1.SqlJobSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Username: "foo",
						PasswordSecretKeyRef: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "foo",
							},
							Key: "foo",
						},
						Sql: func() *string { s := "foo"; return &s }(),
					},
				},
				false,
			),
			Entry(
				"Valid with schedule",
				&v1alpha1.SqlJob{
					ObjectMeta: objMeta,
					Spec: v1alpha1.SqlJobSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Schedule: &v1alpha1.Schedule{
							Cron: "*/1 * * * *",
						},
						Username: "foo",
						PasswordSecretKeyRef: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "foo",
							},
							Key: "foo",
						},
						Sql: func() *string { s := "foo"; return &s }(),
					},
				},
				false,
			),
			Entry(
				"Valid with schedule and history limits",
				&v1alpha1.SqlJob{
					ObjectMeta: objMeta,
					Spec: v1alpha1.SqlJobSpec{
						MariaDBRef: v1alpha1.MariaDBRef{
							ObjectReference: v1alpha1.ObjectReference{
								Name: "foo",
							},
						},
						Schedule: &v1alpha1.Schedule{
							Cron: "foo",
						},
						Username: "foo",
						PasswordSecretKeyRef: v1alpha1.SecretKeySelector{
							LocalObjectReference: v1alpha1.LocalObjectReference{
								Name: "foo",
							},
							Key: "foo",
						},
						Sql: func() *string { s := "foo"; return &s }(),
						CronJobTemplate: v1alpha1.CronJobTemplate{
							SuccessfulJobsHistoryLimit: ptr.To[int32](5),
							FailedJobsHistoryLimit:     ptr.To[int32](5),
						},
					},
				},
				true,
			),
		)
	})

	Context("When updating a v1alpha1.SqlJob", Ordered, func() {
		key := types.NamespacedName{
			Name:      "sqljob-update-webhook",
			Namespace: testNamespace,
		}
		BeforeAll(func() {
			sqlJob := v1alpha1.SqlJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      key.Name,
					Namespace: key.Namespace,
				},
				Spec: v1alpha1.SqlJobSpec{
					DependsOn: []v1alpha1.LocalObjectReference{
						{
							Name: "sqljob-webhook",
						},
					},
					MariaDBRef: v1alpha1.MariaDBRef{
						ObjectReference: v1alpha1.ObjectReference{
							Name: "mariadb-webhook",
						},
						WaitForIt: true,
					},
					Username: "test",
					PasswordSecretKeyRef: v1alpha1.SecretKeySelector{
						LocalObjectReference: v1alpha1.LocalObjectReference{
							Name: "test",
						},
						Key: "test",
					},
					Database: func() *string { d := "test"; return &d }(),
					Sql: func() *string {
						sql := `CREATE TABLE IF NOT EXISTS users (
							id bigint PRIMARY KEY AUTO_INCREMENT,
							username varchar(255) NOT NULL,
							email varchar(255) NOT NULL,
							UNIQUE KEY name__unique_idx (username),
							UNIQUE KEY email__unique_idx (email)
						);`
						return &sql
					}(),
				},
			}
			Expect(k8sClient.Create(testCtx, &sqlJob)).To(Succeed())
		})
		DescribeTable(
			"Should validate",
			func(patchFn func(job *v1alpha1.SqlJob), wantErr bool) {
				var sqlJob v1alpha1.SqlJob
				Expect(k8sClient.Get(testCtx, key, &sqlJob)).To(Succeed())

				patch := client.MergeFrom(sqlJob.DeepCopy())
				patchFn(&sqlJob)

				err := k8sClient.Patch(testCtx, &sqlJob, patch)
				if wantErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry(
				"Updating BackoffLimit",
				func(job *v1alpha1.SqlJob) {
					job.Spec.BackoffLimit = 20
				},
				false,
			),
			Entry(
				"Updating RestartPolicy",
				func(job *v1alpha1.SqlJob) {
					job.Spec.RestartPolicy = corev1.RestartPolicyNever
				},
				true,
			),
			Entry(
				"Updating Resources",
				func(job *v1alpha1.SqlJob) {
					job.Spec.Resources = &v1alpha1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu": resource.MustParse("200m"),
						},
					}
				},
				false,
			),
			Entry(
				"Updating MariaDBRef",
				func(job *v1alpha1.SqlJob) {
					job.Spec.MariaDBRef.Name = "another-mariadb"
				},
				true,
			),
			Entry(
				"Updating Username",
				func(job *v1alpha1.SqlJob) {
					job.Spec.Username = "foo"
				},
				true,
			),
			Entry(
				"Updating PasswordSecretKeyRef",
				func(job *v1alpha1.SqlJob) {
					job.Spec.PasswordSecretKeyRef.Name = "foo"
				},
				true,
			),
			Entry(
				"Updating Database",
				func(job *v1alpha1.SqlJob) {
					job.Spec.Database = func() *string { d := "foo"; return &d }()
				},
				true,
			),
			Entry(
				"Updating DependsOn",
				func(job *v1alpha1.SqlJob) {
					job.Spec.DependsOn = nil
				},
				true,
			),
			Entry(
				"Updating Sql",
				func(job *v1alpha1.SqlJob) {
					job.Spec.Sql = func() *string { d := "foo"; return &d }()
				},
				true,
			),
			Entry(
				"Updating SqlConfigMapKeyRef",
				func(job *v1alpha1.SqlJob) {
					job.Spec.SqlConfigMapKeyRef = &v1alpha1.ConfigMapKeySelector{
						LocalObjectReference: v1alpha1.LocalObjectReference{
							Name: "foo",
						},
					}
				},
				false,
			),
			Entry(
				"Updating Schedule",
				func(job *v1alpha1.SqlJob) {
					job.Spec.Schedule = &v1alpha1.Schedule{
						Cron:    "*/1 * * * *",
						Suspend: false,
					}
				},
				false,
			),
			Entry(
				"Updating with wrong Schedule",
				func(job *v1alpha1.SqlJob) {
					job.Spec.Schedule = &v1alpha1.Schedule{
						Cron:    "foo",
						Suspend: false,
					}
				},
				true,
			),
			Entry(
				"Updating SuccessfulJobsHistoryLimit",
				func(job *v1alpha1.SqlJob) {
					job.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](5)
				},
				false,
			),
			Entry(
				"Updating with wrong SuccessfulJobsHistoryLimit",
				func(job *v1alpha1.SqlJob) {
					job.Spec.SuccessfulJobsHistoryLimit = ptr.To[int32](-5)
				},
				true,
			),
			Entry(
				"Updating FailedJobsHistoryLimit",
				func(job *v1alpha1.SqlJob) {
					job.Spec.FailedJobsHistoryLimit = ptr.To[int32](5)
				},
				false,
			),
			Entry(
				"Updating with wrong FailedJobsHistoryLimit",
				func(job *v1alpha1.SqlJob) {
					job.Spec.FailedJobsHistoryLimit = ptr.To[int32](-5)
				},
				true,
			),
			Entry(
				"Removing SQL",
				func(job *v1alpha1.SqlJob) {
					job.Spec.Sql = nil
					job.Spec.SqlConfigMapKeyRef = nil
				},
				true,
			),
		)
	})
})
