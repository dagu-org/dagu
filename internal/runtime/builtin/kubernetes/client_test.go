// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package kubernetes

import (
	"context"
	"path/filepath"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForPodUsesCurrentState(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-pod",
			Namespace: "default",
			Labels: map[string]string{
				"job-name": "job-1",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	client := &Client{
		clientset: fake.NewSimpleClientset(pod),
		namespace: "default",
	}
	client.setJobName("job-1")

	podName, err := client.WaitForPod(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "job-pod", podName)
}

func TestWaitForPodReturnsSchedulingFailure(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-pod",
			Namespace: "default",
			Labels: map[string]string{
				"job-name": "job-1",
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Type:    corev1.PodScheduled,
				Status:  corev1.ConditionFalse,
				Message: "0/3 nodes are available",
			}},
		},
	}

	client := &Client{
		clientset: fake.NewSimpleClientset(pod),
		namespace: "default",
	}
	client.setJobName("job-1")

	_, err := client.WaitForPod(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unschedulable")
}

func TestWaitForCompletionUsesCurrentState(t *testing.T) {
	tests := []struct {
		name    string
		job     *batchv1.Job
		wantErr string
	}{
		{
			name: "Completed",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{{
						Type:   batchv1.JobComplete,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		{
			name: "Failed",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "job-1", Namespace: "default"},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{{
						Type:    batchv1.JobFailed,
						Status:  corev1.ConditionTrue,
						Message: "container exited 1",
					}},
				},
			},
			wantErr: "container exited 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				clientset: fake.NewSimpleClientset(tt.job),
				namespace: "default",
			}
			client.setJobName("job-1")

			err := client.WaitForCompletion(context.Background())
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDeleteJobIgnoresNotFound(t *testing.T) {
	client := &Client{
		clientset: fake.NewSimpleClientset(),
		namespace: "default",
	}
	client.setJobName("job-1")

	err := client.DeleteJob(context.Background())
	require.NoError(t, err)
	assert.Empty(t, client.GetJobName())
}

func TestBuildRESTConfigFailsFastForExplicitKubeconfig(t *testing.T) {
	cfg, err := buildRESTConfig(&Config{
		Kubeconfig: "/definitely/missing/kubeconfig",
	})

	require.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig error")
}

func TestBuildRESTConfigFailsFastForExplicitContext(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "config")

	raw := clientcmdapi.NewConfig()
	raw.Clusters["cluster-1"] = &clientcmdapi.Cluster{Server: "https://example.com"}
	raw.AuthInfos["user-1"] = &clientcmdapi.AuthInfo{}
	raw.Contexts["ctx-1"] = &clientcmdapi.Context{
		Cluster:  "cluster-1",
		AuthInfo: "user-1",
	}
	raw.CurrentContext = "ctx-1"
	require.NoError(t, clientcmd.WriteToFile(*raw, kubeconfigPath))

	cfg, err := buildRESTConfig(&Config{
		Kubeconfig: kubeconfigPath,
		Context:    "missing-context",
	})

	require.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubeconfig error")
	assert.Contains(t, err.Error(), "context")
}
