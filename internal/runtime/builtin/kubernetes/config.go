// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package kubernetes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	ErrImageRequired            = errors.New("kubernetes executor requires an image")
	ErrInvalidCleanupPolicy     = errors.New("kubernetes executor cleanup_policy must be either delete or keep")
	ErrInvalidImagePullPolicy   = errors.New("kubernetes executor image_pull_policy must be one of Always, IfNotPresent, or Never")
	ErrNegativeActiveDeadline   = errors.New("kubernetes executor active_deadline must be >= 0")
	ErrNegativeBackoffLimit     = errors.New("kubernetes executor backoff_limit must be >= 0")
	ErrNegativeTTLAfterFinished = errors.New("kubernetes executor ttl_after_finished must be >= 0")
	ErrInvalidVolumeSource      = errors.New("kubernetes executor volume must define exactly one source")
)

const (
	cleanupPolicyDelete = "delete"
	cleanupPolicyKeep   = "keep"
)

// Config holds the configuration for creating a Kubernetes Job.
type Config struct {
	// Kubeconfig is the path to the kubeconfig file.
	// If empty, uses default discovery (KUBECONFIG env, ~/.kube/config, in-cluster).
	Kubeconfig string `mapstructure:"kubeconfig"`
	// Context is the kubeconfig context to use. If empty, uses current-context.
	Context string `mapstructure:"context"`

	// Namespace is the Kubernetes namespace. Default: "default".
	Namespace string `mapstructure:"namespace"`
	// Image is the container image to use (required).
	Image string `mapstructure:"image"`
	// ImagePullPolicy is the image pull policy (Always, IfNotPresent, Never).
	ImagePullPolicy string `mapstructure:"image_pull_policy"`
	// ImagePullSecrets are the names of secrets for pulling images from private registries.
	ImagePullSecrets []string `mapstructure:"image_pull_secrets"`

	// WorkingDir is the working directory inside the container.
	WorkingDir string `mapstructure:"working_dir"`
	// Env specifies environment variables for the container.
	Env []EnvVar `mapstructure:"env"`
	// EnvFrom specifies sources to populate environment variables.
	EnvFrom []EnvFromSource `mapstructure:"env_from"`

	// Resources specifies CPU/memory requests and limits.
	Resources *ResourceRequirements `mapstructure:"resources"`
	// ServiceAccount is the service account to use for the pod.
	ServiceAccount string `mapstructure:"service_account"`
	// NodeSelector constrains which nodes the pod can run on.
	NodeSelector map[string]string `mapstructure:"node_selector"`
	// Tolerations allow the pod to schedule onto nodes with matching taints.
	Tolerations []Toleration `mapstructure:"tolerations"`

	// Labels are applied to the Job and Pod.
	Labels map[string]string `mapstructure:"labels"`
	// Annotations are applied to the Job and Pod.
	Annotations map[string]string `mapstructure:"annotations"`

	// Volumes defines volumes available to the pod.
	Volumes []Volume `mapstructure:"volumes"`
	// VolumeMounts defines mount points for volumes in the container.
	VolumeMounts []VolumeMount `mapstructure:"volume_mounts"`

	// ActiveDeadlineSeconds is the Kubernetes-native timeout for the Job in seconds.
	ActiveDeadlineSeconds *int64 `mapstructure:"active_deadline"`
	// BackoffLimit is the number of retries before considering the Job failed. Default: 0.
	BackoffLimit *int32 `mapstructure:"backoff_limit"`
	// TTLSecondsAfterFinished controls automatic cleanup by Kubernetes.
	TTLSecondsAfterFinished *int32 `mapstructure:"ttl_after_finished"`

	// CleanupPolicy controls whether to delete the Job after completion.
	// "delete" (default) or "keep".
	CleanupPolicy string `mapstructure:"cleanup_policy"`
}

// EnvVar represents a Kubernetes environment variable.
type EnvVar struct {
	Name      string        `mapstructure:"name"`
	Value     string        `mapstructure:"value"`
	ValueFrom *EnvVarSource `mapstructure:"value_from"`
}

// EnvVarSource represents a source for an environment variable's value.
type EnvVarSource struct {
	SecretKeyRef    *KeySelector `mapstructure:"secret_key_ref"`
	ConfigMapKeyRef *KeySelector `mapstructure:"config_map_key_ref"`
	FieldRef        *FieldRef    `mapstructure:"field_ref"`
}

// KeySelector selects a key from a ConfigMap or Secret.
type KeySelector struct {
	Name string `mapstructure:"name"`
	Key  string `mapstructure:"key"`
}

// FieldRef selects a field of the pod.
type FieldRef struct {
	FieldPath string `mapstructure:"field_path"`
}

// EnvFromSource represents a source to populate environment variables.
type EnvFromSource struct {
	ConfigMapRef *EnvFromRef `mapstructure:"config_map_ref"`
	SecretRef    *EnvFromRef `mapstructure:"secret_ref"`
	Prefix       string      `mapstructure:"prefix"`
}

// EnvFromRef references a ConfigMap or Secret for envFrom.
type EnvFromRef struct {
	Name string `mapstructure:"name"`
}

// ResourceRequirements specifies CPU and memory requests and limits.
type ResourceRequirements struct {
	Requests map[string]string `mapstructure:"requests"`
	Limits   map[string]string `mapstructure:"limits"`
}

// Toleration represents a Kubernetes toleration.
type Toleration struct {
	Key      string `mapstructure:"key"`
	Operator string `mapstructure:"operator"`
	Value    string `mapstructure:"value"`
	Effect   string `mapstructure:"effect"`
}

// Volume defines a volume available to the pod.
type Volume struct {
	Name                  string     `mapstructure:"name"`
	EmptyDir              *EmptyDir  `mapstructure:"empty_dir"`
	HostPath              *HostPath  `mapstructure:"host_path"`
	ConfigMap             *ConfigMap `mapstructure:"config_map"`
	Secret                *SecretVol `mapstructure:"secret"`
	PersistentVolumeClaim *PVCVol    `mapstructure:"persistent_volume_claim"`
}

// EmptyDir represents an emptyDir volume source.
type EmptyDir struct {
	Medium    string `mapstructure:"medium"`
	SizeLimit string `mapstructure:"size_limit"`
}

// HostPath represents a hostPath volume source.
type HostPath struct {
	Path string `mapstructure:"path"`
	Type string `mapstructure:"type"`
}

// ConfigMap represents a configMap volume source.
type ConfigMap struct {
	Name string `mapstructure:"name"`
}

// SecretVol represents a secret volume source.
type SecretVol struct {
	SecretName string `mapstructure:"secret_name"`
}

// PVCVol represents a persistentVolumeClaim volume source.
type PVCVol struct {
	ClaimName string `mapstructure:"claim_name"`
	ReadOnly  bool   `mapstructure:"read_only"`
}

// VolumeMount defines a mount point for a volume in a container.
type VolumeMount struct {
	Name      string `mapstructure:"name"`
	MountPath string `mapstructure:"mount_path"`
	SubPath   string `mapstructure:"sub_path"`
	ReadOnly  bool   `mapstructure:"read_only"`
}

// LoadConfigFromMap decodes a config map into a Config struct and applies defaults.
func LoadConfigFromMap(data map[string]any) (*Config, error) {
	cfg := &Config{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           cfg,
		WeaklyTypedInput: true,
		TagName:          "mapstructure",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}
	if err := decoder.Decode(data); err != nil {
		return nil, fmt.Errorf("failed to decode kubernetes config: %w", err)
	}

	normalizeConfig(cfg)
	applyDefaults(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func normalizeConfig(cfg *Config) {
	cfg.Kubeconfig = strings.TrimSpace(cfg.Kubeconfig)
	cfg.Context = strings.TrimSpace(cfg.Context)
	cfg.Namespace = strings.TrimSpace(cfg.Namespace)
	cfg.Image = strings.TrimSpace(cfg.Image)
	cfg.ImagePullPolicy = strings.TrimSpace(cfg.ImagePullPolicy)
	cfg.CleanupPolicy = strings.ToLower(strings.TrimSpace(cfg.CleanupPolicy))
}

func applyDefaults(cfg *Config) {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.CleanupPolicy == "" {
		cfg.CleanupPolicy = cleanupPolicyDelete
	}
	if cfg.BackoffLimit == nil {
		zero := int32(0)
		cfg.BackoffLimit = &zero
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Image == "" {
		return ErrImageRequired
	}
	if _, err := cfg.toK8sImagePullPolicy(); err != nil {
		return err
	}
	switch cfg.CleanupPolicy {
	case cleanupPolicyDelete, cleanupPolicyKeep:
	default:
		return ErrInvalidCleanupPolicy
	}
	if cfg.ActiveDeadlineSeconds != nil && *cfg.ActiveDeadlineSeconds < 0 {
		return ErrNegativeActiveDeadline
	}
	if cfg.BackoffLimit != nil && *cfg.BackoffLimit < 0 {
		return ErrNegativeBackoffLimit
	}
	if cfg.TTLSecondsAfterFinished != nil && *cfg.TTLSecondsAfterFinished < 0 {
		return ErrNegativeTTLAfterFinished
	}
	if _, err := cfg.toK8sResourceRequirements(); err != nil {
		return err
	}
	if _, err := cfg.toK8sVolumes(); err != nil {
		return err
	}
	return nil
}

// toK8sEnvVars converts config EnvVars to Kubernetes API types.
func (cfg *Config) toK8sEnvVars() []corev1.EnvVar {
	if len(cfg.Env) == 0 {
		return nil
	}
	envVars := make([]corev1.EnvVar, 0, len(cfg.Env))
	for _, e := range cfg.Env {
		ev := corev1.EnvVar{
			Name:  e.Name,
			Value: e.Value,
		}
		if e.ValueFrom != nil {
			ev.ValueFrom = &corev1.EnvVarSource{}
			if e.ValueFrom.SecretKeyRef != nil {
				ev.ValueFrom.SecretKeyRef = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: e.ValueFrom.SecretKeyRef.Name},
					Key:                  e.ValueFrom.SecretKeyRef.Key,
				}
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				ev.ValueFrom.ConfigMapKeyRef = &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: e.ValueFrom.ConfigMapKeyRef.Name},
					Key:                  e.ValueFrom.ConfigMapKeyRef.Key,
				}
			}
			if e.ValueFrom.FieldRef != nil {
				ev.ValueFrom.FieldRef = &corev1.ObjectFieldSelector{
					FieldPath: e.ValueFrom.FieldRef.FieldPath,
				}
			}
		}
		envVars = append(envVars, ev)
	}
	return envVars
}

// toK8sEnvFromSources converts config EnvFromSources to Kubernetes API types.
func (cfg *Config) toK8sEnvFromSources() []corev1.EnvFromSource {
	if len(cfg.EnvFrom) == 0 {
		return nil
	}
	sources := make([]corev1.EnvFromSource, 0, len(cfg.EnvFrom))
	for _, ef := range cfg.EnvFrom {
		s := corev1.EnvFromSource{
			Prefix: ef.Prefix,
		}
		if ef.ConfigMapRef != nil {
			s.ConfigMapRef = &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: ef.ConfigMapRef.Name},
			}
		}
		if ef.SecretRef != nil {
			s.SecretRef = &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: ef.SecretRef.Name},
			}
		}
		sources = append(sources, s)
	}
	return sources
}

// toK8sResourceRequirements converts config resources to Kubernetes API types.
func (cfg *Config) toK8sResourceRequirements() (corev1.ResourceRequirements, error) {
	reqs := corev1.ResourceRequirements{}
	if cfg.Resources == nil {
		return reqs, nil
	}
	if len(cfg.Resources.Requests) > 0 {
		reqs.Requests = make(corev1.ResourceList)
		for k, v := range cfg.Resources.Requests {
			qty, err := parseQuantity(fmt.Sprintf("resources.requests.%s", k), v)
			if err != nil {
				return corev1.ResourceRequirements{}, err
			}
			reqs.Requests[corev1.ResourceName(k)] = qty
		}
	}
	if len(cfg.Resources.Limits) > 0 {
		reqs.Limits = make(corev1.ResourceList)
		for k, v := range cfg.Resources.Limits {
			qty, err := parseQuantity(fmt.Sprintf("resources.limits.%s", k), v)
			if err != nil {
				return corev1.ResourceRequirements{}, err
			}
			reqs.Limits[corev1.ResourceName(k)] = qty
		}
	}
	return reqs, nil
}

// toK8sTolerations converts config tolerations to Kubernetes API types.
func (cfg *Config) toK8sTolerations() []corev1.Toleration {
	if len(cfg.Tolerations) == 0 {
		return nil
	}
	tols := make([]corev1.Toleration, 0, len(cfg.Tolerations))
	for _, t := range cfg.Tolerations {
		tols = append(tols, corev1.Toleration{
			Key:      t.Key,
			Operator: corev1.TolerationOperator(t.Operator),
			Value:    t.Value,
			Effect:   corev1.TaintEffect(t.Effect),
		})
	}
	return tols
}

// toK8sVolumes converts config volumes to Kubernetes API types.
func (cfg *Config) toK8sVolumes() ([]corev1.Volume, error) {
	if len(cfg.Volumes) == 0 {
		return nil, nil
	}
	vols := make([]corev1.Volume, 0, len(cfg.Volumes))
	for _, v := range cfg.Volumes {
		if countVolumeSources(v) != 1 {
			return nil, fmt.Errorf("volume %q: %w", v.Name, ErrInvalidVolumeSource)
		}

		vol := corev1.Volume{Name: v.Name}
		switch {
		case v.EmptyDir != nil:
			emptyDir := &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMedium(v.EmptyDir.Medium),
			}
			if v.EmptyDir.SizeLimit != "" {
				qty, err := parseQuantity(fmt.Sprintf("volumes[%s].empty_dir.size_limit", v.Name), v.EmptyDir.SizeLimit)
				if err != nil {
					return nil, err
				}
				emptyDir.SizeLimit = &qty
			}
			vol.VolumeSource = corev1.VolumeSource{EmptyDir: emptyDir}
		case v.HostPath != nil:
			hostPathType := corev1.HostPathUnset
			if v.HostPath.Type != "" {
				hostPathType = corev1.HostPathType(v.HostPath.Type)
			}
			vol.VolumeSource = corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: v.HostPath.Path,
					Type: &hostPathType,
				},
			}
		case v.ConfigMap != nil:
			vol.VolumeSource = corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: v.ConfigMap.Name},
				},
			}
		case v.Secret != nil:
			vol.VolumeSource = corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: v.Secret.SecretName,
				},
			}
		case v.PersistentVolumeClaim != nil:
			vol.VolumeSource = corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: v.PersistentVolumeClaim.ClaimName,
					ReadOnly:  v.PersistentVolumeClaim.ReadOnly,
				},
			}
		}
		vols = append(vols, vol)
	}
	return vols, nil
}

// toK8sVolumeMounts converts config volume mounts to Kubernetes API types.
func (cfg *Config) toK8sVolumeMounts() []corev1.VolumeMount {
	if len(cfg.VolumeMounts) == 0 {
		return nil
	}
	mounts := make([]corev1.VolumeMount, 0, len(cfg.VolumeMounts))
	for _, m := range cfg.VolumeMounts {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      m.Name,
			MountPath: m.MountPath,
			SubPath:   m.SubPath,
			ReadOnly:  m.ReadOnly,
		})
	}
	return mounts
}

// toK8sImagePullPolicy converts config image pull policy to Kubernetes API type.
func (cfg *Config) toK8sImagePullPolicy() (corev1.PullPolicy, error) {
	switch strings.ToLower(cfg.ImagePullPolicy) {
	case "":
		return "", nil
	case "always":
		return corev1.PullAlways, nil
	case "never":
		return corev1.PullNever, nil
	case "ifnotpresent":
		return corev1.PullIfNotPresent, nil
	default:
		return "", ErrInvalidImagePullPolicy
	}
}

// toK8sImagePullSecrets converts config image pull secret names to Kubernetes API types.
func (cfg *Config) toK8sImagePullSecrets() []corev1.LocalObjectReference {
	if len(cfg.ImagePullSecrets) == 0 {
		return nil
	}
	refs := make([]corev1.LocalObjectReference, 0, len(cfg.ImagePullSecrets))
	for _, name := range cfg.ImagePullSecrets {
		refs = append(refs, corev1.LocalObjectReference{Name: name})
	}
	return refs
}

func countVolumeSources(v Volume) int {
	count := 0
	if v.EmptyDir != nil {
		count++
	}
	if v.HostPath != nil {
		count++
	}
	if v.ConfigMap != nil {
		count++
	}
	if v.Secret != nil {
		count++
	}
	if v.PersistentVolumeClaim != nil {
		count++
	}
	return count
}

func parseQuantity(field, value string) (resource.Quantity, error) {
	qty, err := resource.ParseQuantity(value)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("%s: invalid quantity %q: %w", field, value, err)
	}
	return qty, nil
}

func init() {
	core.RegisterExecutorConfigSchema("kubernetes", configSchema)
	core.RegisterExecutorConfigSchema("k8s", configSchema)
}

var configSchema = &jsonschema.Schema{
	Type:     "object",
	Required: []string{"image"},
	Properties: map[string]*jsonschema.Schema{
		"image":              {Type: "string", Description: "Container image (required)"},
		"namespace":          {Type: "string", Description: "Kubernetes namespace (default: default)"},
		"kubeconfig":         {Type: "string", Description: "Path to kubeconfig file"},
		"context":            {Type: "string", Description: "Kubeconfig context name"},
		"service_account":    {Type: "string", Description: "Service account for the pod"},
		"working_dir":        {Type: "string", Description: "Working directory inside the container"},
		"image_pull_policy":  {Type: "string", Description: "Image pull policy (Always, IfNotPresent, Never)"},
		"image_pull_secrets": {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Secret names for private registries"},
		"env":                {Type: "array", Items: &jsonschema.Schema{Type: "object"}, Description: "Environment variables"},
		"env_from":           {Type: "array", Items: &jsonschema.Schema{Type: "object"}, Description: "Environment variable sources"},
		"resources":          {Type: "object", Description: "CPU/memory requests and limits"},
		"node_selector":      {Type: "object", Description: "Node selector constraints"},
		"tolerations":        {Type: "array", Items: &jsonschema.Schema{Type: "object"}, Description: "Pod tolerations"},
		"labels":             {Type: "object", Description: "Labels for Job and Pod"},
		"annotations":        {Type: "object", Description: "Annotations for Job and Pod"},
		"volumes":            {Type: "array", Items: &jsonschema.Schema{Type: "object"}, Description: "Pod volumes"},
		"volume_mounts":      {Type: "array", Items: &jsonschema.Schema{Type: "object"}, Description: "Container volume mounts"},
		"active_deadline":    {Type: "integer", Description: "Job timeout in seconds"},
		"backoff_limit":      {Type: "integer", Description: "Number of retries (default: 0)"},
		"ttl_after_finished": {Type: "integer", Description: "TTL for automatic cleanup in seconds"},
		"cleanup_policy":     {Type: "string", Description: "delete (default) or keep"},
	},
}
