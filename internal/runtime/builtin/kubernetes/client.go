// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package kubernetes

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset and manages Job lifecycle.
type Client struct {
	clientset kubernetes.Interface
	cfg       *Config
	jobName   string
	namespace string
	mu        sync.Mutex
}

// NewClient creates a Kubernetes client using the given config.
// It discovers the kubeconfig in this order:
// 1. Explicit kubeconfig path from config
// 2. KUBECONFIG env / ~/.kube/config (default loading rules)
// 3. In-cluster config
func NewClient(cfg *Config) (*Client, error) {
	restCfg, err := buildRESTConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		clientset: clientset,
		cfg:       cfg,
		namespace: cfg.Namespace,
	}, nil
}

func buildRESTConfig(cfg *Config) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cfg.Kubeconfig != "" {
		loadingRules.ExplicitPath = cfg.Kubeconfig
	}

	overrides := &clientcmd.ConfigOverrides{}
	if cfg.Context != "" {
		overrides.CurrentContext = cfg.Context
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	restCfg, err := kubeConfig.ClientConfig()
	if err != nil {
		// Fall back to in-cluster config
		restCfg, inClusterErr := rest.InClusterConfig()
		if inClusterErr != nil {
			return nil, fmt.Errorf("kubeconfig error: %w; in-cluster error: %w", err, inClusterErr)
		}
		return restCfg, nil
	}
	return restCfg, nil
}

// CreateJob creates a Kubernetes Job from the config.
// The command parameter overrides the container command if non-empty.
func (c *Client) CreateJob(ctx context.Context, stepName string, command []string) error {
	job := c.buildJob(stepName, command)

	created, err := c.clientset.BatchV1().Jobs(c.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes job: %w", err)
	}

	c.mu.Lock()
	c.jobName = created.Name
	c.mu.Unlock()

	return nil
}

func (c *Client) buildJob(stepName string, command []string) *batchv1.Job {
	// Build container spec
	container := corev1.Container{
		Name:            "step",
		Image:           c.cfg.Image,
		ImagePullPolicy: c.cfg.toK8sImagePullPolicy(),
		Env:             c.cfg.toK8sEnvVars(),
		EnvFrom:         c.cfg.toK8sEnvFromSources(),
		Resources:       c.cfg.toK8sResourceRequirements(),
		VolumeMounts:    c.cfg.toK8sVolumeMounts(),
	}

	if c.cfg.WorkingDir != "" {
		container.WorkingDir = c.cfg.WorkingDir
	}

	if len(command) > 0 {
		container.Command = command
	}

	// Generate a safe name prefix from step name
	namePrefix := sanitizeName(stepName) + "-"

	// Build pod spec
	podSpec := corev1.PodSpec{
		Containers:       []corev1.Container{container},
		RestartPolicy:    corev1.RestartPolicyNever,
		Volumes:          c.cfg.toK8sVolumes(),
		ImagePullSecrets: c.cfg.toK8sImagePullSecrets(),
	}

	if c.cfg.ServiceAccount != "" {
		podSpec.ServiceAccountName = c.cfg.ServiceAccount
	}
	if len(c.cfg.NodeSelector) > 0 {
		podSpec.NodeSelector = c.cfg.NodeSelector
	}
	if tols := c.cfg.toK8sTolerations(); len(tols) > 0 {
		podSpec.Tolerations = tols
	}

	// Build Job spec
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix,
			Namespace:    c.namespace,
			Labels:       c.cfg.Labels,
			Annotations:  c.cfg.Annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: c.cfg.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      c.cfg.Labels,
					Annotations: c.cfg.Annotations,
				},
				Spec: podSpec,
			},
		},
	}

	if c.cfg.ActiveDeadlineSeconds != nil {
		job.Spec.ActiveDeadlineSeconds = c.cfg.ActiveDeadlineSeconds
	}
	if c.cfg.TTLSecondsAfterFinished != nil {
		job.Spec.TTLSecondsAfterFinished = c.cfg.TTLSecondsAfterFinished
	}

	return job
}

// WaitForPod watches for a Pod created by the Job and returns its name
// once it reaches Running, Succeeded, or Failed state.
func (c *Client) WaitForPod(ctx context.Context) (string, error) {
	c.mu.Lock()
	jobName := c.jobName
	c.mu.Unlock()

	watcher, err := c.clientset.CoreV1().Pods(c.namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + jobName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to watch pods: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return "", fmt.Errorf("pod watcher closed unexpectedly")
			}
			if event.Type == watch.Error {
				return "", fmt.Errorf("watch error: %v", event.Object)
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			switch pod.Status.Phase {
			case corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed:
				return pod.Name, nil
			case corev1.PodPending:
				// Check for scheduling or image pull errors
				if reason := getPodFailureReason(pod); reason != "" {
					return "", fmt.Errorf("pod scheduling failed: %s", reason)
				}
			}
		}
	}
}

// StreamLogs streams logs from the given pod to the stdout writer.
// Kubernetes merges stdout and stderr into a single log stream.
func (c *Client) StreamLogs(ctx context.Context, podName string, stdout io.Writer) error {
	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow:    true,
		Container: "step",
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream: %w", err)
	}
	defer stream.Close()

	if _, err := io.Copy(stdout, stream); err != nil {
		// Context cancellation during streaming is expected during Kill
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("log streaming error: %w", err)
	}
	return nil
}

// WaitForCompletion watches the Job until it completes or fails.
func (c *Client) WaitForCompletion(ctx context.Context) error {
	c.mu.Lock()
	jobName := c.jobName
	c.mu.Unlock()

	watcher, err := c.clientset.BatchV1().Jobs(c.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + jobName,
	})
	if err != nil {
		return fmt.Errorf("failed to watch job: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("job watcher closed unexpectedly")
			}
			if event.Type == watch.Error {
				return fmt.Errorf("job watch error: %v", event.Object)
			}
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
					return nil
				}
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					return fmt.Errorf("job failed: %s", cond.Message)
				}
			}
		}
	}
}

// GetExitCode retrieves the exit code from the terminated container in the pod.
func (c *Client) GetExitCode(ctx context.Context, podName string) (int, error) {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return -1, fmt.Errorf("failed to get pod: %w", err)
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == "step" && cs.State.Terminated != nil {
			return int(cs.State.Terminated.ExitCode), nil
		}
	}

	return -1, fmt.Errorf("container exit code not available")
}

// DeleteJob deletes the Job and its Pods using background propagation.
func (c *Client) DeleteJob(ctx context.Context) error {
	c.mu.Lock()
	jobName := c.jobName
	c.mu.Unlock()

	if jobName == "" {
		return nil
	}

	propagation := metav1.DeletePropagationBackground
	err := c.clientset.BatchV1().Jobs(c.namespace).Delete(ctx, jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil {
		return fmt.Errorf("failed to delete job %s: %w", jobName, err)
	}
	return nil
}

// GetJobName returns the name of the created Job.
func (c *Client) GetJobName() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.jobName
}

// getPodFailureReason checks for known failure conditions in pod status.
func getPodFailureReason(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			switch cs.State.Waiting.Reason {
			case "ImagePullBackOff", "ErrImagePull", "InvalidImageName":
				return fmt.Sprintf("%s: %s", cs.State.Waiting.Reason, cs.State.Waiting.Message)
			}
		}
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			return fmt.Sprintf("unschedulable: %s", cond.Message)
		}
	}
	return ""
}

// sanitizeName converts a step name to a valid Kubernetes resource name prefix.
// Kubernetes names must be lowercase, alphanumeric, or '-', and start with a letter.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == '_' || r == ' ' {
			b.WriteRune('-')
		}
	}
	result := b.String()
	// Trim leading non-alpha characters
	for len(result) > 0 && (result[0] < 'a' || result[0] > 'z') {
		result = result[1:]
	}
	// Trim trailing hyphens
	result = strings.TrimRight(result, "-")
	if result == "" {
		result = "dagu-k8s"
	}
	// Kubernetes GenerateName adds a suffix, keep prefix short
	const maxLen = 50
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}

// waitForPodTermination polls the pod until its container terminates.
// This is used after log streaming completes to ensure we can read the exit code.
func (c *Client) waitForPodTermination(ctx context.Context, podName string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod status: %w", err)
			}
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				return nil
			}
		}
	}
}
