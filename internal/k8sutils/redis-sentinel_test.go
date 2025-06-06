package k8sutils

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
)

func Test_generateRedisSentinelParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-sentinel.yaml")
	expected := statefulSetParameters{
		Replicas:       ptr.To(int32(3)),
		ClusterMode:    false,
		NodeConfVolume: false,
		NodeSelector: map[string]string{
			"node-role.kubernetes.io/infra": "worker",
		},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{
				MaxSkew:           1,
				TopologyKey:       "kubernetes.io/hostname",
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": "sentinel",
						"app":  "redis-sentinel-sentinel",
					},
				},
			},
		},
		PodSecurityContext: &corev1.PodSecurityContext{
			RunAsUser: ptr.To(int64(1000)),
			FSGroup:   ptr.To(int64(1000)),
		},
		PriorityClassName: "high-priority",
		MinReadySeconds:   5,
		Affinity: &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "node-role.kubernetes.io/infra",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"worker"},
								},
							},
						},
					},
				},
			},
		},
		Tolerations: &[]corev1.Toleration{
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Key:      "node-role.kubernetes.io/infra",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
		},
		EnableMetrics:                 true,
		ImagePullSecrets:              &[]corev1.LocalObjectReference{{Name: "mysecret"}},
		ServiceAccountName:            ptr.To("redis-sa"),
		TerminationGracePeriodSeconds: ptr.To(int64(30)),
		IgnoreAnnotations:             []string{"opstreelabs.in/ignore"},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &rsvb2.RedisSentinel{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual := generateRedisSentinelParams(context.TODO(), input, *input.Spec.Size, nil, input.Spec.Affinity)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}

func Test_generateRedisSentinelContainerParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-sentinel.yaml")
	expected := containerParameters{
		Image:           "quay.io/opstree/redis:v7.0.12",
		ImagePullPolicy: corev1.PullPolicy("IfNotPresent"),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("101m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("101m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:              ptr.To(int64(1000)),
			RunAsGroup:             ptr.To(int64(1000)),
			RunAsNonRoot:           ptr.To(true),
			ReadOnlyRootFilesystem: ptr.To(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"NET_BIND_SERVICE"},
			},
		},
		RedisExporterImage:           "quay.io/opstree/redis-exporter:v1.44.0",
		RedisExporterImagePullPolicy: corev1.PullPolicy("Always"),
		RedisExporterResources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		RedisExporterEnv: &[]corev1.EnvVar{
			{
				Name:  "REDIS_EXPORTER_INCL_SYSTEM_METRICS",
				Value: "true",
			},
			{
				Name: "UI_PROPERTIES_FILE_NAME",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "game-demo",
						},
						Key: "ui_properties_file_name",
					},
				},
			},
			{
				Name: "SECRET_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "mysecret",
						},
						Key: "username",
					},
				},
			},
		},
		Role:            "sentinel",
		EnabledPassword: ptr.To(true),
		SecretName:      ptr.To("redis-secret"),
		SecretKey:       ptr.To("password"),
		TLSConfig: &common.TLSConfig{
			CaKeyFile:   "ca.key",
			CertKeyFile: "tls.crt",
			KeyFile:     "tls.key",
			Secret: corev1.SecretVolumeSource{
				SecretName: "redis-tls-cert",
			},
		},
		AdditionalEnvVariable: &[]corev1.EnvVar{},
		EnvVars: &[]corev1.EnvVar{
			{
				Name:  "CUSTOM_ENV_VAR_1",
				Value: "custom_value_1",
			},
			{
				Name:  "CUSTOM_ENV_VAR_2",
				Value: "custom_value_2",
			},
		},
		Port: ptr.To(26379),
		AdditionalVolume: []corev1.Volume{
			{
				Name: "redis-config",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		AdditionalMountPath: []corev1.VolumeMount{
			{
				Name:        "redis-config",
				ReadOnly:    false,
				MountPath:   "/etc/redis",
				SubPath:     "",
				SubPathExpr: "",
			},
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &rsvb2.RedisSentinel{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual, err := generateRedisSentinelContainerParams(context.TODO(), nil, input, nil, nil, nil)
	require.NoError(t, err)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}

func Test_generateRedisSentinelInitContainerParams(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "redis-sentinel.yaml")
	expected := initContainerParameters{
		Enabled:         ptr.To(true),
		Image:           "quay.io/opstree/redis-operator-restore:latest",
		ImagePullPolicy: corev1.PullPolicy("Always"),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
		Role:      "sentinel",
		Command:   []string{"/bin/bash", "-c", "/app/restore.bash"},
		Arguments: []string{"--restore-from", "redis-sentinel-restore"},
		AdditionalEnvVariable: &[]corev1.EnvVar{
			{
				Name: "CLUSTER_NAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "env-secrets",
						},
						Key: "CLUSTER_NAME",
					},
				},
			},
			{
				Name: "CLUSTER_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "env-secrets",
						},
						Key: "CLUSTER_NAMESPACE",
					},
				},
			},
		},
		AdditionalVolume: []corev1.Volume{
			{
				Name: "redis-config",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		AdditionalMountPath: []corev1.VolumeMount{
			{
				Name:        "redis-config",
				ReadOnly:    false,
				MountPath:   "/etc/redis",
				SubPath:     "",
				SubPathExpr: "",
			},
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}

	input := &rsvb2.RedisSentinel{}
	err = yaml.UnmarshalStrict(data, input)
	if err != nil {
		t.Fatalf("Failed to unmarshal file %s: %v", path, err)
	}

	actual := generateRedisSentinelInitContainerParams(input)
	assert.EqualValues(t, expected, actual, "Expected %+v, got %+v", expected, actual)
}

func Test_getSentinelEnvVariable(t *testing.T) {
	type args struct {
		cr *rsvb2.RedisSentinel
	}
	tests := []struct {
		name string
		args args
		want *[]corev1.EnvVar
	}{
		{
			name: "When RedisSentinelConfig is nil",
			args: args{
				cr: &rsvb2.RedisSentinel{},
			},
			want: &[]corev1.EnvVar{},
		},
		{
			name: "When RedisSentinelConfig is not nil",
			args: args{
				cr: &rsvb2.RedisSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-sentinel",
						Namespace: "redis",
					},
					Spec: rsvb2.RedisSentinelSpec{
						RedisSentinelConfig: &rsvb2.RedisSentinelConfig{
							RedisSentinelConfig: common.RedisSentinelConfig{
								RedisReplicationName:  "redis-replication",
								MasterGroupName:       "master",
								RedisPort:             "6379",
								Quorum:                "2",
								DownAfterMilliseconds: "30000",
								ParallelSyncs:         "1",
								FailoverTimeout:       "180000",
								ResolveHostnames:      "no",
								AnnounceHostnames:     "no",
							},
						},
					},
				},
			},
			want: &[]corev1.EnvVar{
				{
					Name:  "MASTER_GROUP_NAME",
					Value: "master",
				},
				{
					Name:  "PORT",
					Value: "6379",
				},
				{
					Name:  "QUORUM",
					Value: "2",
				},
				{
					Name:  "DOWN_AFTER_MILLISECONDS",
					Value: "30000",
				},
				{
					Name:  "PARALLEL_SYNCS",
					Value: "1",
				},
				{
					Name:  "FAILOVER_TIMEOUT",
					Value: "180000",
				},
				{
					Name:  "RESOLVE_HOSTNAMES",
					Value: "no",
				},
				{
					Name:  "ANNOUNCE_HOSTNAMES",
					Value: "no",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSentinelEnvVariable(tt.args.cr)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSentinelEnvVariable() = %v, want %v", got, tt.want)
			}
		})
	}
}
