package builder

import (
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	builderpki "github.com/mariadb-operator/mariadb-operator/v25/pkg/builder/pki"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/command"
	galeraresources "github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/galera/resources"
	kadapter "github.com/mariadb-operator/mariadb-operator/v25/pkg/kubernetes/adapter"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var (
	MariadbContainerName = "mariadb"
	MariadbPortName      = "mariadb"

	MaxScaleContainerName = "maxscale"
	MaxScaleAdminPortName = "admin"

	InitContainerName  = "init"
	AgentContainerName = "agent"

	defaultProbe = corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"-c",
					"mariadb -u root -p\"${MARIADB_ROOT_PASSWORD}\" -e \"SELECT 1;\"",
				},
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
	defaultGaleraAgentProbe = func(galera mariadbv1alpha1.Galera) *corev1.Probe {
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(int(galera.Agent.ProbePort)),
				},
			},
		}
	}
)

func (b *Builder) mariadbContainers(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) ([]corev1.Container, error) {
	mariadbOpts := newMariadbPodOpts(opts...)
	mariadbContainer, err := b.buildContainerWithTemplate(
		mariadb.Spec.Image,
		mariadb.Spec.ImagePullPolicy,
		&mariadb.Spec.ContainerTemplate,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	mariadbContainer.Name = MariadbContainerName
	mariadbContainer.Env = mariadbEnv(mariadb)
	mariadbContainer.VolumeMounts = mariadbVolumeMounts(mariadb, opts...)

	if mariadbOpts.includePorts {
		mariadbContainer.Ports = mariadbPorts(mariadb)
	}
	if mariadbOpts.includeProbes {
		mariadbContainer.StartupProbe = mariadbStartupProbe(mariadb)
		mariadbContainer.LivenessProbe = mariadbLivenessProbe(mariadb)
		mariadbContainer.ReadinessProbe = mariadbReadinessProbe(mariadb)
	}

	if mariadbOpts.command != nil {
		mariadbContainer.Command = mariadbOpts.command
	}
	if mariadbOpts.args != nil {
		mariadbContainer.Args = mariadbOpts.args
	} else {
		mariadbContainer.Args = mariadbArgs(mariadb)
	}

	var containers []corev1.Container
	containers = append(containers, *mariadbContainer)

	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGaleraContainers {
		agentContainer, err := b.galeraAgentContainer(mariadb)
		if err != nil {
			return nil, err
		}
		containers = append(containers, *agentContainer)
	}
	if mariadb.Spec.SidecarContainers != nil {
		for index, container := range mariadb.Spec.SidecarContainers {
			sidecarContainer := b.buildContainer(mariadb, &container)
			if sidecarContainer.Name == "" {
				sidecarContainer.Name = fmt.Sprintf("sidecar-%d", index)
			}
			containers = append(containers, *sidecarContainer)
		}
	}

	return containers, nil
}

func (b *Builder) maxscaleContainers(mxs *mariadbv1alpha1.MaxScale) ([]corev1.Container, error) {
	tpl := mxs.Spec.ContainerTemplate
	container, err := b.buildContainerWithTemplate(mxs.Spec.Image, mxs.Spec.ImagePullPolicy, &tpl)
	if err != nil {
		return nil, err
	}
	command := command.NewCommand(
		[]string{
			"maxscale",
		},
		[]string{
			"--config",
			fmt.Sprintf("%s/%s", MaxscaleConfigMountPath, mxs.ConfigSecretKeyRef().Key),
			"-dU",
			"maxscale",
			"-l",
			"stdout",
		},
	)

	container.Name = MaxScaleContainerName
	container.Command = command.Command
	if tpl.Command != nil {
		container.Command = tpl.Command
	}
	container.Args = command.Args
	if len(tpl.Args) > 0 {
		container.Args = tpl.Args
	}
	container.Ports = []corev1.ContainerPort{
		{
			Name:          MaxScaleAdminPortName,
			ContainerPort: int32(mxs.Spec.Admin.Port),
		},
	}
	container.VolumeMounts = maxscaleVolumeMounts(mxs)
	container.LivenessProbe = maxscaleProbe(mxs, mxs.Spec.LivenessProbe)
	container.ReadinessProbe = maxscaleProbe(mxs, mxs.Spec.ReadinessProbe)
	container.StartupProbe = maxscaleProbe(mxs, mxs.Spec.StartupProbe)

	return []corev1.Container{*container}, nil
}

func (b *Builder) galeraAgentContainer(mariadb *mariadbv1alpha1.MariaDB) (*corev1.Container, error) {
	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	agent := galera.Agent

	container, err := b.buildContainerWithTemplate(agent.Image, agent.ImagePullPolicy, &agent.ContainerTemplate)
	if err != nil {
		return nil, err
	}

	container.Name = AgentContainerName
	container.Ports = []corev1.ContainerPort{
		{
			Name:          galeraresources.AgentPortName,
			ContainerPort: agent.Port,
		},
		{
			Name:          galeraresources.AgentProbePortName,
			ContainerPort: agent.ProbePort,
		},
	}
	container.Args = func() []string {
		var args []string
		args = append(args, []string{
			"agent",
			fmt.Sprintf("--addr=:%d", agent.Port),
			fmt.Sprintf("--probe-addr=:%d", agent.ProbePort),
			fmt.Sprintf("--config-dir=%s", galeraresources.GaleraConfigMountPath),
			fmt.Sprintf("--state-dir=%s", MariadbStorageMountPath),
		}...)
		if agent.GracefulShutdownTimeout != nil {
			args = append(args, fmt.Sprintf("--graceful-shutdown-timeout=%s", agent.GracefulShutdownTimeout.Duration))
		}

		kubernetesAuth := ptr.Deref(agent.KubernetesAuth, mariadbv1alpha1.KubernetesAuth{})
		basicAuth := ptr.Deref(agent.BasicAuth, mariadbv1alpha1.BasicAuth{})

		if kubernetesAuth.Enabled {
			args = append(args, []string{
				"--kubernetes-auth",
				fmt.Sprintf("--kubernetes-trusted-name=%s", b.env.MariadbOperatorName),
				fmt.Sprintf("--kubernetes-trusted-namespace=%s", b.env.MariadbOperatorNamespace),
			}...)
		} else if basicAuth.Enabled && !reflect.ValueOf(basicAuth.PasswordSecretKeyRef).IsZero() {
			args = append(args, []string{
				"--basic-auth",
				fmt.Sprintf("--basic-auth-username=%s", basicAuth.Username),
				fmt.Sprintf("--basic-auth-password-path=%s", path.Join(galeraresources.AgentAuthVolumeMount, basicAuth.PasswordSecretKeyRef.Key)),
			}...)
		}

		args = append(args, container.Args...)
		return args
	}()
	container.Env = mariadbEnv(mariadb)
	container.VolumeMounts = mariadbVolumeMounts(mariadb)
	container.LivenessProbe = func() *corev1.Probe {
		if container.LivenessProbe != nil {
			return container.LivenessProbe
		}
		return defaultGaleraAgentProbe(galera)
	}()
	container.ReadinessProbe = func() *corev1.Probe {
		if container.ReadinessProbe != nil {
			return container.ReadinessProbe
		}
		return defaultGaleraAgentProbe(galera)
	}()
	return container, nil
}

func (b *Builder) mariadbInitContainers(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) ([]corev1.Container, error) {
	mariadbOpts := newMariadbPodOpts(opts...)
	initContainers := []corev1.Container{}
	if mariadb.Spec.InitContainers != nil {
		for index, container := range mariadb.Spec.InitContainers {
			initContainer := b.buildContainer(mariadb, &container)
			if initContainer.Name == "" {
				initContainer.Name = fmt.Sprintf("init-%d", index)
			}
			initContainers = append(initContainers, *initContainer)
		}
	}
	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGaleraContainers {
		initContainer, err := b.galeraInitContainer(mariadb)
		if err != nil {
			return nil, err
		}
		initContainers = append(initContainers, *initContainer)
	}
	return initContainers, nil
}

func (b *Builder) galeraInitContainer(mariadb *mariadbv1alpha1.MariaDB) (*corev1.Container, error) {
	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	if !galera.Enabled {
		return nil, errors.New("Galera is not enabled") //nolint:staticcheck
	}
	init := galera.InitContainer
	container, err := b.buildContainerWithTemplate(init.Image, init.ImagePullPolicy, &init.ContainerTemplate)
	if err != nil {
		return nil, err
	}

	container.Name = InitContainerName
	container.Args = func() []string {
		args := container.Args
		args = append(args, []string{
			"init",
			fmt.Sprintf("--config-dir=%s", galeraresources.GaleraConfigMountPath),
			fmt.Sprintf("--state-dir=%s", MariadbStorageMountPath),
		}...)
		return args
	}()
	container.Env = mariadbEnv(mariadb)
	container.VolumeMounts = mariadbVolumeMounts(mariadb)

	return container, nil
}

func (b *Builder) buildContainerWithTemplate(image string, pullPolicy corev1.PullPolicy, tpl *mariadbv1alpha1.ContainerTemplate,
	opts ...mariadbPodOpt) (*corev1.Container, error) {
	mariadbOpts := newMariadbPodOpts(opts...)

	sc, err := b.buildContainerSecurityContext(tpl.SecurityContext)
	if err != nil {
		return nil, err
	}

	container := corev1.Container{
		Image:           image,
		ImagePullPolicy: pullPolicy,
		Command:         tpl.Command,
		Args:            tpl.Args,
		Env:             kadapter.ToKubernetesSlice(tpl.Env),
		EnvFrom:         kadapter.ToKubernetesSlice(tpl.EnvFrom),
		VolumeMounts:    kadapter.ToKubernetesSlice(tpl.VolumeMounts),
		SecurityContext: sc,
	}
	if mariadbOpts.resources != nil {
		container.Resources = *mariadbOpts.resources
	} else if tpl.Resources != nil && mariadbOpts.includeMariadbResources {
		container.Resources = tpl.Resources.ToKubernetesType()
	}
	return &container, nil
}

func (b *Builder) buildContainer(mdb *mariadbv1alpha1.MariaDB, mdbContainer *mariadbv1alpha1.Container) *corev1.Container {
	env := mariadbEnv(mdb)
	if mdbContainer.Env != nil {
		env = append(env, kadapter.ToKubernetesSlice(mdbContainer.Env)...)
	}

	volumeMounts := mariadbVolumeMounts(mdb)
	if mdbContainer.VolumeMounts != nil {
		volumeMounts = append(volumeMounts, kadapter.ToKubernetesSlice(mdbContainer.VolumeMounts)...)
	}

	container := corev1.Container{
		Name:            mdbContainer.Name,
		Image:           mdbContainer.Image,
		ImagePullPolicy: mdbContainer.ImagePullPolicy,
		Command:         mdbContainer.Command,
		Args:            mdbContainer.Args,
		Env:             env,
		VolumeMounts:    volumeMounts,
	}
	if mdbContainer.Resources != nil {
		container.Resources = mdbContainer.Resources.ToKubernetesType()
	}
	return &container
}

func mariadbArgs(mariadb *mariadbv1alpha1.MariaDB) []string {
	var mariadbArgs []string
	if mariadb.Replication().Enabled {
		mariadbArgs = append(mariadbArgs, []string{
			"--log-bin",
			fmt.Sprintf("--log-basename=%s", mariadb.Name)}...)
	}
	if mariadb.Spec.Args != nil {
		mariadbArgs = append(mariadbArgs, mariadb.Spec.Args...)
	}
	return mariadbArgs
}

func mariadbEnv(mariadb *mariadbv1alpha1.MariaDB) []corev1.EnvVar {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}

	env := []corev1.EnvVar{
		{
			Name:  "MYSQL_TCP_PORT",
			Value: strconv.Itoa(int(mariadb.Spec.Port)),
		},
		{
			Name:  "MARIADB_ROOT_HOST",
			Value: "%",
		},
		{
			Name:  "CLUSTER_NAME",
			Value: clusterName,
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name:  "MARIADB_NAME",
			Value: mariadb.Name,
		},
	}

	if mariadb.IsTLSEnabled() {
		env = append(env, []corev1.EnvVar{
			{
				Name:  "TLS_ENABLED",
				Value: strconv.FormatBool(mariadb.IsTLSEnabled()),
			},
			{
				Name:  "TLS_CA_CERT_PATH",
				Value: builderpki.CACertPath,
			},
			{
				Name:  "TLS_SERVER_CERT_PATH",
				Value: builderpki.ServerCertPath,
			},
			{
				Name:  "TLS_SERVER_KEY_PATH",
				Value: builderpki.ServerKeyPath,
			},
			{
				Name:  "TLS_CLIENT_CERT_PATH",
				Value: builderpki.ClientCertPath,
			},
			{
				Name:  "TLS_CLIENT_KEY_PATH",
				Value: builderpki.ClientKeyPath,
			},
		}...)

		// By default, wsrep_sst_mariabackup.sh validates the client certificate commonName against the Pod IP.
		// This doesn't work with Kubernetes, we cannot issue a certificate for a specific IP, as Pod IPs are ephemeral and unpredictable.
		// Instead, we could configure wsrep_sst_mariabackup.sh to validate the certificate against the expected commonName:
		// See:
		// https://github.com/codership/mariadb-server/blob/16394f1aa1b4097f897b8ab01ea2064726cca059/scripts/wsrep_sst_common.sh#L1064
		// https://github.com/codership/mariadb-server/blob/16394f1aa1b4097f897b8ab01ea2064726cca059/scripts/wsrep_sst_mariabackup.sh#L407
		clientNames := mariadb.TLSClientNames()
		if mariadb.IsGaleraEnabled() && len(clientNames) > 0 {
			env = append(env, corev1.EnvVar{
				Name:  "WSREP_SST_OPT_REMOTE_AUTH",
				Value: fmt.Sprintf("%s:", clientNames[0]),
			})
		}
	}

	if mariadb.IsRootPasswordEmpty() {
		env = append(env, corev1.EnvVar{
			Name:  "MARIADB_ALLOW_EMPTY_ROOT_PASSWORD",
			Value: "yes",
		})
	} else {
		env = append(env, corev1.EnvVar{
			Name: "MARIADB_ROOT_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: ptr.To(mariadb.Spec.RootPasswordSecretKeyRef.ToKubernetesType()),
			},
		})
	}

	if mariadb.Spec.TimeZone == nil {
		env = append(env, corev1.EnvVar{
			Name:  "MYSQL_INITDB_SKIP_TZINFO",
			Value: "1",
		})
	}

	if mariadb.Spec.Env != nil {
		idx := make(map[string]int, len(env))
		for i, envVar := range env {
			idx[envVar.Name] = i
		}
		for _, envVar := range mariadb.Spec.Env {
			if i, ok := idx[envVar.Name]; ok {
				env[i] = envVar.ToKubernetesType()
			} else {
				env = append(env, envVar.ToKubernetesType())
			}
		}
	}

	return env
}

func mariadbVolumeMounts(mariadb *mariadbv1alpha1.MariaDB, opts ...mariadbPodOpt) []corev1.VolumeMount {
	mariadbOpts := newMariadbPodOpts(opts...)
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      ConfigVolume,
			MountPath: MariadbConfigMountPath,
		},
	}

	if mariadb.IsTLSEnabled() {
		_, tlsVolumeMounts := mariadbTLSVolumes(mariadb)
		volumeMounts = append(volumeMounts, tlsVolumeMounts...)
	}

	galera := ptr.Deref(mariadb.Spec.Galera, mariadbv1alpha1.Galera{})
	reuseStorageVolume := ptr.Deref(galera.Config.ReuseStorageVolume, false)

	storageVolumeMount := corev1.VolumeMount{
		Name:      StorageVolume,
		MountPath: MariadbStorageMountPath,
	}
	if mariadb.IsGaleraEnabled() && reuseStorageVolume {
		storageVolumeMount.SubPath = StorageVolume
	}
	volumeMounts = append(volumeMounts, storageVolumeMount)

	if mariadb.Replication().Enabled && ptr.Deref(mariadb.Replication().ProbesEnabled, false) {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ProbesVolume,
			MountPath: ProbesMountPath,
		})
	}
	if mariadb.IsGaleraEnabled() && mariadbOpts.includeServiceAccount {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ServiceAccountVolume,
			MountPath: ServiceAccountMountPath,
		})
	}
	if mariadb.IsGaleraEnabled() && mariadbOpts.includeGaleraConfig {
		galeraConfigVolumeMount := corev1.VolumeMount{
			MountPath: galeraresources.GaleraConfigMountPath,
		}
		if reuseStorageVolume {
			galeraConfigVolumeMount.Name = StorageVolume
			galeraConfigVolumeMount.SubPath = galeraresources.GaleraConfigVolume
		} else {
			galeraConfigVolumeMount.Name = galeraresources.GaleraConfigVolume
		}

		basicAuth := ptr.Deref(galera.Agent.BasicAuth, mariadbv1alpha1.BasicAuth{})
		if basicAuth.Enabled {
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      galeraresources.AgentAuthVolume,
				MountPath: galeraresources.AgentAuthVolumeMount,
			})
		}

		volumeMounts = append(volumeMounts, galeraConfigVolumeMount)
	}
	if mariadb.Spec.VolumeMounts != nil {
		volumeMounts = append(volumeMounts, kadapter.ToKubernetesSlice(mariadb.Spec.VolumeMounts)...)
	}
	if mariadbOpts.extraVolumeMounts != nil {
		volumeMounts = append(volumeMounts, mariadbOpts.extraVolumeMounts...)
	}
	return volumeMounts
}

func maxscaleVolumeMounts(maxscale *mariadbv1alpha1.MaxScale) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      StorageVolume,
			MountPath: MaxscaleStorageMountPath,
		},
		{
			Name:      ConfigVolume,
			MountPath: MaxscaleConfigMountPath,
		},
		{
			Name:      RunVolume,
			MountPath: MaxScaleRunMountPath,
		},
		{
			Name:      LogVolume,
			MountPath: MaxScaleLogMountPath,
		},
		{
			Name:      CacheVolume,
			MountPath: MaxScaleCacheMountPath,
		},
	}
	if maxscale.IsTLSEnabled() {
		_, tlsVolumeMounts := maxscaleTLSVolumes(maxscale)
		volumeMounts = append(volumeMounts, tlsVolumeMounts...)
	}
	if maxscale.Spec.VolumeMounts != nil {
		volumeMounts = append(volumeMounts, kadapter.ToKubernetesSlice(maxscale.Spec.VolumeMounts)...)
	}
	return volumeMounts
}

func mariadbPorts(mariadb *mariadbv1alpha1.MariaDB) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          MariadbPortName,
			ContainerPort: mariadb.Spec.Port,
		},
	}
	if mariadb.IsGaleraEnabled() {
		ports = append(ports, []corev1.ContainerPort{
			{
				Name:          galeraresources.GaleraClusterPortName,
				ContainerPort: galeraresources.GaleraClusterPort,
			},
			{
				Name:          galeraresources.GaleraISTPortName,
				ContainerPort: galeraresources.GaleraISTPort,
			},
			{
				Name:          galeraresources.GaleraSSTPortName,
				ContainerPort: galeraresources.GaleraSSTPort,
			},
		}...)
	}
	return ports
}

func mariadbLivenessProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.IsGaleraEnabled() {
		return mariadbGaleraProbe(mariadb, "/liveness", mariadb.Spec.LivenessProbe)
	}
	return mariadbProbe(mariadb, mariadb.Spec.LivenessProbe)
}

func mariadbStartupProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.IsGaleraEnabled() {
		return mariadbGaleraProbe(mariadb, "/liveness", mariadb.Spec.StartupProbe)
	}
	return mariadbProbe(mariadb, mariadb.Spec.StartupProbe)
}

func mariadbReadinessProbe(mariadb *mariadbv1alpha1.MariaDB) *corev1.Probe {
	if mariadb.IsGaleraEnabled() {
		return mariadbGaleraProbe(mariadb, "/readiness", mariadb.Spec.ReadinessProbe)
	}
	return mariadbProbe(mariadb, mariadb.Spec.ReadinessProbe)
}

func mariadbProbe(mariadb *mariadbv1alpha1.MariaDB, probe *mariadbv1alpha1.Probe) *corev1.Probe {
	if mariadb.Replication().Enabled && ptr.Deref(mariadb.Replication().ProbesEnabled, false) {
		replProbe := mariadbReplProbe(mariadb, probe)
		if probe != nil {
			setProbeThresholds(replProbe, ptr.To(probe.ToKubernetesType()))
		}
		return replProbe
	}
	if probe != nil && probe.ProbeHandler != (mariadbv1alpha1.ProbeHandler{}) {
		return ptr.To(probe.ToKubernetesType())
	}

	defaultProbe := defaultProbe.DeepCopy()
	if probe != nil {
		setProbeThresholds(defaultProbe, ptr.To(probe.ToKubernetesType()))
	}
	return defaultProbe
}

func mariadbReplProbe(mariadb *mariadbv1alpha1.MariaDB, probe *mariadbv1alpha1.Probe) *corev1.Probe {
	replProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"bash",
					"-c",
					fmt.Sprintf("%s/%s", ProbesMountPath, mariadb.ReplConfigMapKeyRef().Key),
				},
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
	if probe != nil {
		setProbeThresholds(replProbe, ptr.To(probe.ToKubernetesType()))
	}
	return replProbe
}

func mariadbGaleraProbe(mdb *mariadbv1alpha1.MariaDB, path string, probe *mariadbv1alpha1.Probe) *corev1.Probe {
	agent := ptr.Deref(mdb.Spec.Galera, mariadbv1alpha1.Galera{}).Agent
	galeraProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt(int(agent.ProbePort)),
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
	if probe != nil {
		setProbeThresholds(&galeraProbe, ptr.To(probe.ToKubernetesType()))
	}
	return &galeraProbe
}

func maxscaleProbe(mxs *mariadbv1alpha1.MaxScale, probe *mariadbv1alpha1.Probe) *corev1.Probe {
	if probe != nil && probe.ProbeHandler != (mariadbv1alpha1.ProbeHandler{}) {
		return ptr.To(probe.ToKubernetesType())
	}
	mxsProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt(int(mxs.Spec.Admin.Port)),
			},
		},
		InitialDelaySeconds: 20,
		TimeoutSeconds:      5,
		PeriodSeconds:       10,
	}
	if probe != nil {
		setProbeThresholds(&mxsProbe, ptr.To(probe.ToKubernetesType()))
	}
	return &mxsProbe
}

func setProbeThresholds(source, target *corev1.Probe) {
	if target == nil {
		return
	}
	if target.InitialDelaySeconds > 0 {
		source.InitialDelaySeconds = target.InitialDelaySeconds
	}
	if target.TimeoutSeconds > 0 {
		source.TimeoutSeconds = target.TimeoutSeconds
	}
	if target.PeriodSeconds > 0 {
		source.PeriodSeconds = target.PeriodSeconds
	}
	if target.SuccessThreshold > 0 {
		source.SuccessThreshold = target.SuccessThreshold
	}
	if target.FailureThreshold > 0 {
		source.FailureThreshold = target.FailureThreshold
	}
}
