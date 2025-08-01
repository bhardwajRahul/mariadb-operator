package v1alpha1

import (
	"time"

	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("MaxScale types", func() {
	format.MaxLength = 10000
	storageClassName := "test-sc"
	objMeta := metav1.ObjectMeta{
		Name:      "maxscale-obj",
		Namespace: "maxscale-obj",
	}
	env := &environment.OperatorEnv{
		RelatedMariadbImage:          "mariadb/maxscale:23.08",
		RelatedExporterMaxscaleImage: "mariadb/maxscale-prometheus-exporter-ubi:latest",
	}
	mariadbObjMeta := metav1.ObjectMeta{
		Name:      "mdb-maxscale-obj",
		Namespace: "mdb-maxscale-obj",
	}
	mariadb := &MariaDB{
		ObjectMeta: mariadbObjMeta,
	}
	Context("When creating a MaxScale object", func() {
		DescribeTable(
			"Should default",
			func(mxs, expected *MaxScale, env *environment.OperatorEnv) {
				mxs.SetDefaults(env, mariadb)
				Expect(mxs).To(BeEquivalentTo(expected))
			},
			Entry(
				"Single replica",
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Module: MonitorModuleMariadb,
						},
					},
				},
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						MaxScalePodTemplate: MaxScalePodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image: env.RelatedMaxscaleImage,
						Servers: []MaxScaleServer{
							{
								Name:     "mariadb-0",
								Address:  "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
						},
						RequeueInterval: &metav1.Duration{Duration: 30 * time.Second},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Name:     "rw-router-listener",
									Port:     3306,
									Protocol: "MariaDBProtocol",
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Name:     "mariadbmon-monitor",
							Module:   MonitorModuleMariadb,
							Interval: metav1.Duration{Duration: 2 * time.Second},
						},
						Admin: MaxScaleAdmin{
							Port:       8989,
							GuiEnabled: ptr.To(true),
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
								},
							},
						},
						Auth: MaxScaleAuth{
							AdminUsername: "mariadb-operator",
							AdminPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-admin",
									},
									Key: "password",
								},
								Generate: true,
							},
							DeleteDefaultAdmin: ptr.To(true),
							ClientUsername:     "maxscale-obj-client",
							ClientPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-client",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientMaxConnections: 30,
							ServerUsername:       "maxscale-obj-server",
							ServerPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-server",
									},
									Key: "password",
								},
								Generate: true,
							},
							ServerMaxConnections: 30,
							MonitorUsername:      "maxscale-obj-monitor",
							MonitorPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-monitor",
									},
									Key: "password",
								},
								Generate: true,
							},
							MonitorMaxConnections: 30,
						},
					},
				},
				env,
			),
			Entry(
				"Custom config volumeClaimTemplate",
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Module: MonitorModuleMariadb,
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									StorageClassName: &storageClassName,
								},
							},
						},
					},
				},
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						MaxScalePodTemplate: MaxScalePodTemplate{
							ServiceAccountName: &objMeta.Name,
						},
						Image: env.RelatedMaxscaleImage,
						Servers: []MaxScaleServer{
							{
								Name:     "mariadb-0",
								Address:  "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
						},
						RequeueInterval: &metav1.Duration{Duration: 30 * time.Second},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Name:     "rw-router-listener",
									Port:     3306,
									Protocol: "MariaDBProtocol",
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Name:     "mariadbmon-monitor",
							Module:   MonitorModuleMariadb,
							Interval: metav1.Duration{Duration: 2 * time.Second},
						},
						Admin: MaxScaleAdmin{
							Port:       8989,
							GuiEnabled: ptr.To(true),
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
									StorageClassName: &storageClassName,
								},
							},
						},
						Auth: MaxScaleAuth{
							AdminUsername: "mariadb-operator",
							AdminPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-admin",
									},
									Key: "password",
								},
								Generate: true,
							},
							DeleteDefaultAdmin: ptr.To(true),
							ClientUsername:     "maxscale-obj-client",
							ClientPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-client",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientMaxConnections: 30,
							ServerUsername:       "maxscale-obj-server",
							ServerPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-server",
									},
									Key: "password",
								},
								Generate: true,
							},
							ServerMaxConnections: 30,
							MonitorUsername:      "maxscale-obj-monitor",
							MonitorPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-monitor",
									},
									Key: "password",
								},
								Generate: true,
							},
							MonitorMaxConnections: 30,
						},
					},
				},
				env,
			),
			Entry(
				"HA",
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						MaxScalePodTemplate: MaxScalePodTemplate{
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
							},
						},
						Replicas: 3,
						Servers: []MaxScaleServer{
							{
								Name:    "mariadb-0",
								Address: "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-1",
								Address: "mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local",
							},
							{
								Name:    "mariadb-2",
								Address: "mariadb-repl-2.mariadb-repl-internal.default.svc.cluster.local",
							},
						},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Port: 3306,
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Module: MonitorModuleMariadb,
						},
						TLS: &MaxScaleTLS{
							Enabled: true,
							AdminCASecretRef: &LocalObjectReference{
								Name: "admin-ca",
							},
							AdminCertSecretRef: &LocalObjectReference{
								Name: "admin-cert",
							},
							ListenerCASecretRef: &LocalObjectReference{
								Name: "listener-ca",
							},
							ListenerCertSecretRef: &LocalObjectReference{
								Name: "listener-cert",
							},
							VerifyPeerCertificate: ptr.To(true),
							VerifyPeerHost:        ptr.To(true),
						},
						Metrics: &MaxScaleMetrics{
							Enabled: true,
							Exporter: Exporter{
								Affinity: &AffinityConfig{
									AntiAffinityEnabled: ptr.To(true),
								},
							},
						},
					},
				},
				&MaxScale{
					ObjectMeta: objMeta,
					Spec: MaxScaleSpec{
						MaxScalePodTemplate: MaxScalePodTemplate{
							ServiceAccountName: &objMeta.Name,
							Affinity: &AffinityConfig{
								AntiAffinityEnabled: ptr.To(true),
								Affinity: Affinity{
									PodAntiAffinity: &PodAntiAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
											{
												LabelSelector: &LabelSelector{
													MatchExpressions: []LabelSelectorRequirement{
														{
															Key:      "app.kubernetes.io/instance",
															Operator: metav1.LabelSelectorOpIn,
															Values: []string{
																objMeta.Name,
																mariadbObjMeta.Name,
															},
														},
													},
												},
												TopologyKey: "kubernetes.io/hostname",
											},
										},
									},
								},
							},
						},
						Image:           env.RelatedMaxscaleImage,
						Replicas:        3,
						RequeueInterval: &metav1.Duration{Duration: 30 * time.Second},
						Services: []MaxScaleService{
							{
								Name:   "rw-router",
								Router: ServiceRouterReadWriteSplit,
								Listener: MaxScaleListener{
									Name:     "rw-router-listener",
									Port:     3306,
									Protocol: "MariaDBProtocol",
								},
							},
						},
						Monitor: MaxScaleMonitor{
							Name:                  "mariadbmon-monitor",
							Module:                MonitorModuleMariadb,
							Interval:              metav1.Duration{Duration: 2 * time.Second},
							CooperativeMonitoring: ptr.To(CooperativeMonitoringMajorityOfAll),
						},
						Admin: MaxScaleAdmin{
							Port:       8989,
							GuiEnabled: ptr.To(true),
						},
						Config: MaxScaleConfig{
							VolumeClaimTemplate: VolumeClaimTemplate{
								PersistentVolumeClaimSpec: PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: corev1.ResourceList{
											"storage": resource.MustParse("100Mi"),
										},
									},
									AccessModes: []corev1.PersistentVolumeAccessMode{
										corev1.ReadWriteOnce,
									},
								},
							},
							Sync: &MaxScaleConfigSync{
								Database: "mysql",
								Interval: metav1.Duration{Duration: 5 * time.Second},
								Timeout:  metav1.Duration{Duration: 10 * time.Second},
							},
						},
						Auth: MaxScaleAuth{
							AdminUsername: "mariadb-operator",
							AdminPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-admin",
									},
									Key: "password",
								},
								Generate: true,
							},
							DeleteDefaultAdmin: ptr.To(true),
							MetricsUsername:    "metrics",
							MetricsPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-metrics",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientUsername: "maxscale-obj-client",
							ClientPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-client",
									},
									Key: "password",
								},
								Generate: true,
							},
							ClientMaxConnections: 90,
							ServerUsername:       "maxscale-obj-server",
							ServerPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-server",
									},
									Key: "password",
								},
								Generate: true,
							},
							ServerMaxConnections: 90,
							MonitorUsername:      "maxscale-obj-monitor",
							MonitorPasswordSecretKeyRef: GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-monitor",
									},
									Key: "password",
								},
								Generate: true,
							},
							MonitorMaxConnections: 90,
							SyncUsername:          ptr.To("maxscale-obj-sync"),
							SyncPasswordSecretKeyRef: &GeneratedSecretKeyRef{
								SecretKeySelector: SecretKeySelector{
									LocalObjectReference: LocalObjectReference{
										Name: "maxscale-obj-sync",
									},
									Key: "password",
								},
								Generate: true,
							},
							SyncMaxConnections: ptr.To(int32(90)),
						},
						Servers: []MaxScaleServer{
							{
								Name:     "mariadb-0",
								Address:  "mariadb-repl-0.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
							{
								Name:     "mariadb-1",
								Address:  "mariadb-repl-1.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
							{
								Name:     "mariadb-2",
								Address:  "mariadb-repl-2.mariadb-repl-internal.default.svc.cluster.local",
								Port:     3306,
								Protocol: "MariaDBBackend",
							},
						},
						TLS: &MaxScaleTLS{
							Enabled: true,
							AdminCASecretRef: &LocalObjectReference{
								Name: "admin-ca",
							},
							AdminCertSecretRef: &LocalObjectReference{
								Name: "admin-cert",
							},
							ListenerCASecretRef: &LocalObjectReference{
								Name: "listener-ca",
							},
							ListenerCertSecretRef: &LocalObjectReference{
								Name: "listener-cert",
							},
							VerifyPeerCertificate: ptr.To(true),
							VerifyPeerHost:        ptr.To(true),
						},
						Metrics: &MaxScaleMetrics{
							Enabled: true,
							Exporter: Exporter{
								Image: "mariadb/maxscale-prometheus-exporter-ubi:latest",
								Port:  9105,
								Affinity: &AffinityConfig{
									AntiAffinityEnabled: ptr.To(true),
									Affinity: Affinity{
										PodAntiAffinity: &PodAntiAffinity{
											RequiredDuringSchedulingIgnoredDuringExecution: []PodAffinityTerm{
												{
													LabelSelector: &LabelSelector{
														MatchExpressions: []LabelSelectorRequirement{
															{
																Key:      "app.kubernetes.io/instance",
																Operator: metav1.LabelSelectorOpIn,
																Values: []string{
																	objMeta.Name,
																	mariadbObjMeta.Name,
																},
															},
														},
													},
													TopologyKey: "kubernetes.io/hostname",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				env,
			),
		)
	})

	Context("When setting defaults for MaxScaleTLS", func() {
		It("should set defaults when TLS is enabled and MariaDB is provided", func() {
			tls := &MaxScaleTLS{
				Enabled: true,
			}
			mariadb := &MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mdb",
				},
				Spec: MariaDBSpec{
					TLS: &TLS{
						Enabled:  true,
						Required: ptr.To(true),
						ClientCertSecretRef: &LocalObjectReference{
							Name: "client-cert",
						},
					},
					Replication: &Replication{
						Enabled: true,
					},
				},
			}
			tls.SetDefaults(mariadb)

			Expect(tls.ReplicationSSLEnabled).To(Equal(ptr.To(true)))
			Expect(tls.ServerCASecretRef.Name).To(Equal("mdb-ca-bundle"))
			Expect(tls.ServerCertSecretRef.Name).To(Equal("client-cert"))
		})

		It("should not set defaults when TLS is disabled", func() {
			mariadb := &MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mdb",
					Namespace: testNamespace,
				},
			}

			tls := &MaxScaleTLS{
				Enabled: false,
			}
			tls.SetDefaults(mariadb)

			Expect(tls.ReplicationSSLEnabled).To(BeNil())
			Expect(tls.ServerCASecretRef).To(BeNil())
			Expect(tls.ServerCertSecretRef).To(BeNil())
		})

		It("should not set defaults when TLS is not enforced", func() {
			mariadb := &MariaDB{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mdb",
					Namespace: testNamespace,
				},
				Spec: MariaDBSpec{
					TLS: &TLS{
						Enabled:  true,
						Required: ptr.To(false),
					},
				},
			}

			tls := &MaxScaleTLS{
				Enabled: true,
			}
			tls.SetDefaults(mariadb)

			Expect(tls.ReplicationSSLEnabled).To(BeNil())
			Expect(tls.ServerCASecretRef).To(BeNil())
			Expect(tls.ServerCertSecretRef).To(BeNil())
		})

		It("should not set defaults when MariaDB is not provided", func() {
			tls := &MaxScaleTLS{
				Enabled: true,
			}
			tls.SetDefaults(nil)

			Expect(tls.ReplicationSSLEnabled).To(BeNil())
			Expect(tls.ServerCASecretRef).To(BeNil())
			Expect(tls.ServerCertSecretRef).To(BeNil())
		})
	})
})
