/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2enode

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubefeatures "k8s.io/kubernetes/pkg/features"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enetwork "k8s.io/kubernetes/test/e2e/framework/network"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epodoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	"k8s.io/kubernetes/test/e2e/network/common"
	imageutils "k8s.io/kubernetes/test/utils/image"
	netutils "k8s.io/utils/net"
	"k8s.io/utils/ptr"
)

var _ = common.SIGDescribe("DualStack Host IP [NodeFeature:PodHostIPs] [Feature:PodHostIPs]", func() {
	f := framework.NewDefaultFramework("dualstack")

	ginkgo.Context("when creating a Pod, it has no PodHostIPs feature", func() {
		tempSetCurrentKubeletConfig(f, func(ctx context.Context, initialConfig *kubeletconfig.KubeletConfiguration) {
			initialConfig.FeatureGates = map[string]bool{
				string(kubefeatures.PodHostIPs): false,
			}
		})
		ginkgo.It("should create pod, add host ips is empty", func(ctx context.Context) {

			podName := "pod-dualstack-host-ips"

			pod := genPodHostIPs(podName+string(uuid.NewUUID()), false)
			ginkgo.By("submitting the pod to kubernetes")
			podClient := e2epod.NewPodClient(f)
			p := podClient.CreateSync(ctx, pod)

			gomega.Expect(p.Status.HostIP).ShouldNot(gomega.BeEquivalentTo(""))
			gomega.Expect(p.Status.HostIPs).Should(gomega.BeNil())

			ginkgo.By("deleting the pod")
			err := podClient.Delete(ctx, pod.Name, *metav1.NewDeleteOptions(1))
			framework.ExpectNoError(err, "failed to delete pod")
		})

		ginkgo.It("should create pod with hostNetwork, add host ips is empty", func(ctx context.Context) {

			podName := "pod-dualstack-host-ips"

			pod := genPodHostIPs(podName+string(uuid.NewUUID()), true)
			ginkgo.By("submitting the pod to kubernetes")
			podClient := e2epod.NewPodClient(f)
			p := podClient.CreateSync(ctx, pod)

			gomega.Expect(p.Status.HostIP).ShouldNot(gomega.BeEquivalentTo(""))
			gomega.Expect(p.Status.HostIPs).Should(gomega.BeNil())

			ginkgo.By("deleting the pod")
			err := podClient.Delete(ctx, pod.Name, *metav1.NewDeleteOptions(1))
			framework.ExpectNoError(err, "failed to delete pod")
		})
	})

	ginkgo.Context("when creating a Pod, it has PodHostIPs feature", func() {
		tempSetCurrentKubeletConfig(f, func(ctx context.Context, initialConfig *kubeletconfig.KubeletConfiguration) {
			initialConfig.FeatureGates = map[string]bool{
				string(kubefeatures.PodHostIPs): true,
			}
		})
		ginkgo.It("should create pod, add ipv6 and ipv4 ip to host ips", func(ctx context.Context) {

			podName := "pod-dualstack-host-ips"

			pod := genPodHostIPs(podName+string(uuid.NewUUID()), false)

			ginkgo.By("submitting the pod to kubernetes")
			podClient := e2epod.NewPodClient(f)
			p := podClient.CreateSync(ctx, pod)

			gomega.Expect(p.Status.HostIP).ShouldNot(gomega.BeEquivalentTo(""))
			gomega.Expect(p.Status.HostIPs).ShouldNot(gomega.BeNil())

			// validate first ip in HostIPs is same as HostIP
			gomega.Expect(p.Status.HostIP).Should(gomega.Equal(p.Status.HostIPs[0].IP))
			if len(p.Status.HostIPs) > 1 {
				// assert 2 host ips belong to different families
				if netutils.IsIPv4String(p.Status.HostIPs[0].IP) == netutils.IsIPv4String(p.Status.HostIPs[1].IP) {
					framework.Failf("both internalIPs %s and %s belong to the same families", p.Status.HostIPs[0].IP, p.Status.HostIPs[1].IP)
				}
			}

			nodeList, err := e2enode.GetReadySchedulableNodes(ctx, f.ClientSet)
			framework.ExpectNoError(err)
			for _, node := range nodeList.Items {
				if node.Name == p.Spec.NodeName {
					got := sets.New[string]()
					for _, hostIP := range p.Status.HostIPs {
						got.Insert(hostIP.IP)
					}

					want := sets.New[string]()
					for _, address := range node.Status.Addresses {
						if address.Type == v1.NodeInternalIP {
							want.Insert(address.Address)
						}
					}
					if !got.Equal(want) {
						framework.Failf("got %v, want %v", got, want)
					}
					break
				}
			}

			ginkgo.By("deleting the pod")
			err = podClient.Delete(ctx, pod.Name, *metav1.NewDeleteOptions(1))
			framework.ExpectNoError(err, "failed to delete pod")
		})

		ginkgo.It("should create pod with hostNetwork, add ipv6 and ipv4 ip to host ips", func(ctx context.Context) {

			podName := "pod-dualstack-host-ips"

			pod := genPodHostIPs(podName+string(uuid.NewUUID()), true)

			ginkgo.By("submitting the pod to kubernetes")
			podClient := e2epod.NewPodClient(f)
			p := podClient.CreateSync(ctx, pod)

			gomega.Expect(p.Status.HostIP).ShouldNot(gomega.BeEquivalentTo(""))
			gomega.Expect(p.Status.HostIPs).ShouldNot(gomega.BeNil())

			// validate first ip in HostIPs is same as HostIP
			gomega.Expect(p.Status.HostIP).Should(gomega.Equal(p.Status.HostIPs[0].IP))
			if len(p.Status.HostIPs) > 1 {
				// assert 2 host ips belong to different families
				if netutils.IsIPv4String(p.Status.HostIPs[0].IP) == netutils.IsIPv4String(p.Status.HostIPs[1].IP) {
					framework.Failf("both internalIPs %s and %s belong to the same families", p.Status.HostIPs[0].IP, p.Status.HostIPs[1].IP)
				}
			}

			nodeList, err := e2enode.GetReadySchedulableNodes(ctx, f.ClientSet)
			framework.ExpectNoError(err)
			for _, node := range nodeList.Items {
				if node.Name == p.Spec.NodeName {
					got := sets.New[string]()
					for _, hostIP := range p.Status.HostIPs {
						got.Insert(hostIP.IP)
					}

					want := sets.New[string]()
					for _, address := range node.Status.Addresses {
						if address.Type == v1.NodeInternalIP {
							want.Insert(address.Address)
						}
					}
					if !got.Equal(want) {
						framework.Failf("got %v, want %v", got, want)
					}
					break
				}
			}

			ginkgo.By("deleting the pod")
			err = podClient.Delete(ctx, pod.Name, *metav1.NewDeleteOptions(1))
			framework.ExpectNoError(err, "failed to delete pod")
		})

		ginkgo.It("should provide hostIPs as an env var", func(ctx context.Context) {
			podName := "downward-api-" + string(uuid.NewUUID())
			env := []v1.EnvVar{
				{
					Name: "HOST_IPS",
					ValueFrom: &v1.EnvVarSource{
						FieldRef: &v1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "status.hostIPs",
						},
					},
				},
			}

			expectations := []string{
				fmt.Sprintf("HOST_IPS=%v|%v", e2enetwork.RegexIPv4, e2enetwork.RegexIPv6),
			}

			testDownwardAPI(ctx, f, podName, env, expectations)
		})

		ginkgo.It("should able upgrade and rollback", func(ctx context.Context) {
			podName := "pod-dualstack-host-ips"

			pod := genPodHostIPs(podName+string(uuid.NewUUID()), false)

			ginkgo.By("submitting the pod to kubernetes")
			podClient := e2epod.NewPodClient(f)
			p := podClient.CreateSync(ctx, pod)

			gomega.Expect(p.Status.HostIPs).ShouldNot(gomega.BeNil())

			ginkgo.By("Disable PodHostIPs feature")
			cfg, err := getCurrentKubeletConfig(ctx)
			framework.ExpectNoError(err)

			newCfg := cfg.DeepCopy()
			newCfg.FeatureGates = map[string]bool{
				string(kubefeatures.PodHostIPs): false,
			}

			updateKubeletConfig(ctx, f, newCfg, true)

			gomega.Expect(p.Status.HostIPs).ShouldNot(gomega.BeNil())

			ginkgo.By("deleting the pod")
			err = podClient.Delete(ctx, pod.Name, *metav1.NewDeleteOptions(1))
			framework.ExpectNoError(err, "failed to delete pod")

			ginkgo.By("recreate pod")
			pod = genPodHostIPs(podName+string(uuid.NewUUID()), false)
			p = podClient.CreateSync(ctx, pod)
			// Feature PodHostIPs is disabled, HostIPs should be nil
			gomega.Expect(p.Status.HostIPs).Should(gomega.BeNil())

			newCfg.FeatureGates = map[string]bool{
				string(kubefeatures.PodHostIPs): true,
			}

			updateKubeletConfig(ctx, f, newCfg, true)

			p, err = podClient.Get(ctx, pod.Name, metav1.GetOptions{})
			framework.ExpectNoError(err)
			// Feature PodHostIPs is enabled, HostIPs should not be nil
			gomega.Expect(p.Status.HostIPs).ShouldNot(gomega.BeNil())

			ginkgo.By("deleting the pod")
			err = podClient.Delete(ctx, pod.Name, *metav1.NewDeleteOptions(1))
			framework.ExpectNoError(err, "failed to delete pod")
		})

	})
})

func genPodHostIPs(podName string, hostNetwork bool) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"test": "dualstack-host-ips"},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: imageutils.GetE2EImage(imageutils.Agnhost),
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
						},
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](1000),
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			HostNetwork:   hostNetwork,
		},
	}
}

func testDownwardAPI(ctx context.Context, f *framework.Framework, podName string, env []v1.EnvVar, expectations []string) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"name": podName},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "dapi-container",
					Image:   imageutils.GetE2EImage(imageutils.BusyBox),
					Command: []string{"sh", "-c", "env"},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("250m"),
							v1.ResourceMemory: resource.MustParse("32Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1250m"),
							v1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
					Env: env,
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
						},
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](1000),
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	e2epodoutput.TestContainerOutputRegexp(ctx, f, "downward api env vars", pod, 0, expectations)
}
