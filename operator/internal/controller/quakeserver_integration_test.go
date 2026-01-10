package controller

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	quakekubeiov1alpha1 "github.com/grahamplata/quake-kube/operator/api/v1alpha1"
)

var _ = Describe("QuakeServer Controller Integration", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a basic QuakeServer", func() {
		const serverName = "test-basic-server"

		BeforeEach(func() {
			// Ensure clean state before each test
			server := &quakekubeiov1alpha1.QuakeServer{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server)
			if err == nil {
				Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}
		})

		AfterEach(func() {
			// Cleanup after test
			server := &quakekubeiov1alpha1.QuakeServer{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server)
			if err == nil {
				Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
			}
		})

		It("Should create a QuakeServer resource successfully", func() {
			ctx := context.Background()

			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: serverName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
					ServerConfig: &quakekubeiov1alpha1.ServerConfigSpec{
						FragLimit: intPtr(30),
						Game: &quakekubeiov1alpha1.GameConfigSpec{
							Type: quakekubeiov1alpha1.FreeForAll,
							MOTD: "Test Server",
						},
						Server: &quakekubeiov1alpha1.ServerSettings{
							Hostname:   "Test Quake Server",
							MaxClients: intPtr(16),
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			// Verify the resource was created
			createdServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, createdServer)
			}, timeout, interval).Should(Succeed())

			Expect(createdServer.Spec.AgreeEula).To(BeTrue())
			Expect(createdServer.Spec.ServerConfig).NotTo(BeNil())
			Expect(*createdServer.Spec.ServerConfig.FragLimit).To(Equal(30))
			Expect(createdServer.Spec.ServerConfig.Game.Type).To(Equal(quakekubeiov1alpha1.FreeForAll))
		})
	})

	Context("When creating a QuakeServer with a template reference", func() {
		const (
			templateName = "test-template"
			serverName   = "test-templated-server"
		)

		BeforeEach(func() {
			// Clean up any existing resources
			server := &quakekubeiov1alpha1.QuakeServer{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server); err == nil {
				Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}

			template := &quakekubeiov1alpha1.QuakeServerTemplate{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: templateName}, template); err == nil {
				Expect(k8sClient.Delete(context.Background(), template)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), types.NamespacedName{Name: templateName}, template)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}
		})

		AfterEach(func() {
			// Cleanup
			server := &quakekubeiov1alpha1.QuakeServer{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server); err == nil {
				Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
			}
			template := &quakekubeiov1alpha1.QuakeServerTemplate{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: templateName}, template); err == nil {
				Expect(k8sClient.Delete(context.Background(), template)).To(Succeed())
			}
		})

		It("Should create a QuakeServerTemplate and QuakeServer that references it", func() {
			ctx := context.Background()

			// Create template first
			template := &quakekubeiov1alpha1.QuakeServerTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: templateName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerTemplateSpec{
					ServerConfig: &quakekubeiov1alpha1.ServerConfigSpec{
						FragLimit: intPtr(25),
						Game: &quakekubeiov1alpha1.GameConfigSpec{
							Type: quakekubeiov1alpha1.FreeForAll,
							MOTD: "Template Server",
						},
						Bot: &quakekubeiov1alpha1.BotConfigSpec{
							MinPlayers: intPtr(4),
							Skill:      intPtr(3),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			// Create server referencing the template
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: serverName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
					TemplateRef: &quakekubeiov1alpha1.TemplateReference{
						Name: templateName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			// Verify both resources exist
			createdTemplate := &quakekubeiov1alpha1.QuakeServerTemplate{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: templateName}, createdTemplate)
			}, timeout, interval).Should(Succeed())

			createdServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, createdServer)
			}, timeout, interval).Should(Succeed())

			Expect(createdServer.Spec.TemplateRef).NotTo(BeNil())
			Expect(createdServer.Spec.TemplateRef.Name).To(Equal(templateName))
		})
	})

	Context("When configuring maps", func() {
		const serverName = "test-maps-server"

		AfterEach(func() {
			server := &quakekubeiov1alpha1.QuakeServer{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server); err == nil {
				Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
			}
		})

		It("Should support map configuration with overrides", func() {
			ctx := context.Background()

			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: serverName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
					ServerConfig: &quakekubeiov1alpha1.ServerConfigSpec{
						Maps: []quakekubeiov1alpha1.MapSpec{
							{
								Name: "q3dm7",
								Type: quakekubeiov1alpha1.FreeForAll,
							},
							{
								Name:      "q3ctf1",
								Type:      quakekubeiov1alpha1.CaptureTheFlag,
								FragLimit: intPtr(50),
							},
							{
								Name:         "q3ctf2",
								Type:         quakekubeiov1alpha1.CaptureTheFlag,
								CaptureLimit: intPtr(8),
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			createdServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, createdServer)
			}, timeout, interval).Should(Succeed())

			Expect(createdServer.Spec.ServerConfig.Maps).To(HaveLen(3))
			Expect(createdServer.Spec.ServerConfig.Maps[0].Name).To(Equal("q3dm7"))
			Expect(createdServer.Spec.ServerConfig.Maps[1].Name).To(Equal("q3ctf1"))
			Expect(*createdServer.Spec.ServerConfig.Maps[1].FragLimit).To(Equal(50))
			Expect(*createdServer.Spec.ServerConfig.Maps[2].CaptureLimit).To(Equal(8))
		})
	})

	Context("When configuring gateway", func() {
		const serverName = "test-gateway-server"

		AfterEach(func() {
			server := &quakekubeiov1alpha1.QuakeServer{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server); err == nil {
				Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
			}
		})

		It("Should support gateway configuration", func() {
			ctx := context.Background()

			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: serverName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
					Gateway: &quakekubeiov1alpha1.GatewaySpec{
						Enabled: true,
						GatewayRef: &quakekubeiov1alpha1.GatewayReference{
							Name:      "main-gateway",
							Namespace: "gateway-system",
						},
						Hostname: "server1.quake.example.com",
					},
				},
			}

			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			createdServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, createdServer)
			}, timeout, interval).Should(Succeed())

			Expect(createdServer.Spec.Gateway).NotTo(BeNil())
			Expect(createdServer.Spec.Gateway.Enabled).To(BeTrue())
			Expect(createdServer.Spec.Gateway.GatewayRef.Name).To(Equal("main-gateway"))
			Expect(createdServer.Spec.Gateway.Hostname).To(Equal("server1.quake.example.com"))
		})
	})

	Context("When setting pause annotation", func() {
		const serverName = "test-pause-server"

		AfterEach(func() {
			server := &quakekubeiov1alpha1.QuakeServer{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, server); err == nil {
				Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
			}
		})

		It("Should recognize pause annotation", func() {
			ctx := context.Background()

			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: serverName,
					Annotations: map[string]string{
						"quakekube.io/paused": "true",
					},
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
				},
			}

			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			createdServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, createdServer)
			}, timeout, interval).Should(Succeed())

			Expect(createdServer.Annotations["quakekube.io/paused"]).To(Equal("true"))
			Expect(isPaused(createdServer)).To(BeTrue())
		})

		It("Should detect unpause when annotation removed", func() {
			ctx := context.Background()

			// First create with pause
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: serverName,
					Annotations: map[string]string{
						"quakekube.io/paused": "true",
					},
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
				},
			}

			Expect(k8sClient.Create(ctx, server)).To(Succeed())

			createdServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, createdServer)
			}, timeout, interval).Should(Succeed())

			Expect(isPaused(createdServer)).To(BeTrue())

			// Now remove the annotation
			delete(createdServer.Annotations, "quakekube.io/paused")
			Expect(k8sClient.Update(ctx, createdServer)).To(Succeed())

			updatedServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: serverName}, updatedServer)
				if err != nil {
					return false
				}
				return !isPaused(updatedServer)
			}, timeout, interval).Should(BeTrue())
		})
	})
})

var _ = Describe("QuakeServerTemplate Controller Integration", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a QuakeServerTemplate", func() {
		const templateName = "test-ffa-template"

		AfterEach(func() {
			template := &quakekubeiov1alpha1.QuakeServerTemplate{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: templateName}, template); err == nil {
				Expect(k8sClient.Delete(context.Background(), template)).To(Succeed())
			}
		})

		It("Should create a QuakeServerTemplate resource successfully", func() {
			ctx := context.Background()

			template := &quakekubeiov1alpha1.QuakeServerTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: templateName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerTemplateSpec{
					ServerConfig: &quakekubeiov1alpha1.ServerConfigSpec{
						FragLimit: intPtr(25),
						Game: &quakekubeiov1alpha1.GameConfigSpec{
							Type: quakekubeiov1alpha1.FreeForAll,
							MOTD: "Standard FFA Server",
						},
						Server: &quakekubeiov1alpha1.ServerSettings{
							MaxClients: intPtr(12),
						},
						Bot: &quakekubeiov1alpha1.BotConfigSpec{
							MinPlayers: intPtr(3),
							Skill:      intPtr(3),
						},
						Maps: []quakekubeiov1alpha1.MapSpec{
							{Name: "q3dm7", Type: quakekubeiov1alpha1.FreeForAll},
							{Name: "q3dm17", Type: quakekubeiov1alpha1.FreeForAll},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			createdTemplate := &quakekubeiov1alpha1.QuakeServerTemplate{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: templateName}, createdTemplate)
			}, timeout, interval).Should(Succeed())

			Expect(createdTemplate.Spec.ServerConfig).NotTo(BeNil())
			Expect(*createdTemplate.Spec.ServerConfig.FragLimit).To(Equal(25))
			Expect(createdTemplate.Spec.ServerConfig.Game.Type).To(Equal(quakekubeiov1alpha1.FreeForAll))
			Expect(createdTemplate.Spec.ServerConfig.Maps).To(HaveLen(2))
		})
	})

	Context("When creating different template types", func() {
		It("Should support CTF template configuration", func() {
			ctx := context.Background()
			templateName := "test-ctf-template"

			template := &quakekubeiov1alpha1.QuakeServerTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: templateName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerTemplateSpec{
					ServerConfig: &quakekubeiov1alpha1.ServerConfigSpec{
						Game: &quakekubeiov1alpha1.GameConfigSpec{
							Type: quakekubeiov1alpha1.CaptureTheFlag,
							MOTD: "CTF Server",
						},
						Maps: []quakekubeiov1alpha1.MapSpec{
							{Name: "q3ctf1", Type: quakekubeiov1alpha1.CaptureTheFlag, CaptureLimit: intPtr(8)},
							{Name: "q3ctf2", Type: quakekubeiov1alpha1.CaptureTheFlag, CaptureLimit: intPtr(8)},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			createdTemplate := &quakekubeiov1alpha1.QuakeServerTemplate{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: templateName}, createdTemplate)
			}, timeout, interval).Should(Succeed())

			Expect(createdTemplate.Spec.ServerConfig.Game.Type).To(Equal(quakekubeiov1alpha1.CaptureTheFlag))
			Expect(createdTemplate.Spec.ServerConfig.Maps).To(HaveLen(2))
			Expect(*createdTemplate.Spec.ServerConfig.Maps[0].CaptureLimit).To(Equal(8))

			// Cleanup
			Expect(k8sClient.Delete(ctx, template)).To(Succeed())
		})
	})
})

var _ = Describe("QuakeServer Reconciler Unit Tests", func() {
	Context("Reconciler helper functions", func() {
		It("Should generate correct resource names", func() {
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-server",
				},
			}

			Expect(configMapName(server)).To(Equal("my-server-config"))
			Expect(serviceName(server)).To(Equal("my-server"))
			Expect(deploymentName(server)).To(Equal("my-server"))
			Expect(httpRouteName(server)).To(Equal("my-server-http"))
			Expect(udpRouteName(server)).To(Equal("my-server-udp"))
		})

		It("Should generate correct labels", func() {
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-server",
				},
			}

			labels := labelsForServer(server)
			Expect(labels["app.kubernetes.io/name"]).To(Equal("quake-server"))
			Expect(labels["app.kubernetes.io/instance"]).To(Equal("my-server"))
			Expect(labels["app.kubernetes.io/managed-by"]).To(Equal("quake-operator"))
		})

		It("Should generate correct selector labels", func() {
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-server",
				},
			}

			selectorLabels := selectorLabelsForServer(server)
			Expect(selectorLabels).To(HaveLen(2))
			Expect(selectorLabels["app.kubernetes.io/name"]).To(Equal("quake-server"))
			Expect(selectorLabels["app.kubernetes.io/instance"]).To(Equal("my-server"))
		})
	})

	Context("Config YAML generation", func() {
		It("Should generate default config when nil", func() {
			yaml := generateConfigYAML(nil)
			Expect(yaml).To(ContainSubstring("fragLimit: 25"))
			Expect(yaml).To(ContainSubstring("timeLimit: 15m"))
		})

		It("Should generate config with custom frag limit", func() {
			config := &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(50),
			}
			yaml := generateConfigYAML(config)
			Expect(yaml).To(ContainSubstring("fragLimit: 50"))
		})

		It("Should generate config with bot settings", func() {
			config := &quakekubeiov1alpha1.ServerConfigSpec{
				Bot: &quakekubeiov1alpha1.BotConfigSpec{
					MinPlayers: intPtr(4),
					Skill:      intPtr(3),
					NoChat:     true,
				},
			}
			yaml := generateConfigYAML(config)
			Expect(yaml).To(ContainSubstring("bot:"))
			Expect(yaml).To(ContainSubstring("minPlayers: 4"))
			Expect(yaml).To(ContainSubstring("skill: 3"))
			Expect(yaml).To(ContainSubstring("noChat: true"))
		})
	})

	Context("Security context building", func() {
		It("Should build secure container defaults", func() {
			ctx := buildContainerSecurityContext(nil)
			Expect(ctx).NotTo(BeNil())
			Expect(*ctx.RunAsNonRoot).To(BeTrue())
			Expect(*ctx.AllowPrivilegeEscalation).To(BeFalse())
			Expect(*ctx.ReadOnlyRootFilesystem).To(BeTrue())
			Expect(ctx.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
		})

		It("Should respect custom container security context", func() {
			custom := &corev1.SecurityContext{
				RunAsNonRoot: boolPtr(false),
			}
			ctx := buildContainerSecurityContext(custom)
			Expect(*ctx.RunAsNonRoot).To(BeFalse())
		})

		It("Should build secure pod defaults", func() {
			ctx := buildPodSecurityContext(nil)
			Expect(ctx).NotTo(BeNil())
			Expect(*ctx.RunAsNonRoot).To(BeTrue())
			Expect(*ctx.RunAsUser).To(Equal(int64(1000)))
			Expect(*ctx.RunAsGroup).To(Equal(int64(1000)))
			Expect(*ctx.FSGroup).To(Equal(int64(1000)))
		})

		It("Should respect custom pod security context", func() {
			custom := &corev1.PodSecurityContext{
				RunAsUser: int64Ptr(2000),
			}
			ctx := buildPodSecurityContext(custom)
			Expect(*ctx.RunAsUser).To(Equal(int64(2000)))
		})
	})

	Context("Config merging", func() {
		It("Should handle nil base config", func() {
			override := &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(30),
			}
			result := mergeServerConfig(nil, override)
			Expect(*result.FragLimit).To(Equal(30))
		})

		It("Should handle nil override config", func() {
			base := &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(25),
			}
			result := mergeServerConfig(base, nil)
			Expect(*result.FragLimit).To(Equal(25))
		})

		It("Should merge with override taking precedence", func() {
			base := &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(25),
				Game: &quakekubeiov1alpha1.GameConfigSpec{
					Type: quakekubeiov1alpha1.FreeForAll,
					MOTD: "Base MOTD",
				},
			}
			override := &quakekubeiov1alpha1.ServerConfigSpec{
				FragLimit: intPtr(50),
				Game: &quakekubeiov1alpha1.GameConfigSpec{
					MOTD: "Override MOTD",
				},
			}
			result := mergeServerConfig(base, override)
			Expect(*result.FragLimit).To(Equal(50))
			Expect(result.Game.Type).To(Equal(quakekubeiov1alpha1.FreeForAll)) // Preserved from base
			Expect(result.Game.MOTD).To(Equal("Override MOTD"))
		})
	})
})

var _ = Describe("Resource Builder Functions", func() {
	Context("When building pod spec", func() {
		It("Should build pod spec with correct defaults", func() {
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-server",
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
				},
			}

			reconciler := &QuakeServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			podSpec := reconciler.buildPodSpec(server)

			// Verify two containers
			Expect(podSpec.Containers).To(HaveLen(2))

			// Verify server container
			serverContainer := podSpec.Containers[0]
			Expect(serverContainer.Name).To(Equal("server"))
			Expect(serverContainer.Image).To(Equal(defaultImage))
			Expect(serverContainer.Command).To(Equal([]string{"q3"}))
			Expect(serverContainer.Args).To(ContainElement("--agree-eula"))

			// Verify content-server container
			contentContainer := podSpec.Containers[1]
			Expect(contentContainer.Name).To(Equal("content-server"))
			Expect(contentContainer.Image).To(Equal(defaultImage))

			// Verify volumes
			Expect(podSpec.Volumes).To(HaveLen(2))
			volumeNames := []string{}
			for _, v := range podSpec.Volumes {
				volumeNames = append(volumeNames, v.Name)
			}
			Expect(volumeNames).To(ContainElements("config", "assets"))
		})

		It("Should use custom resources when specified", func() {
			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-server",
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			}

			reconciler := &QuakeServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			podSpec := reconciler.buildPodSpec(server)
			serverContainer := podSpec.Containers[0]

			Expect(serverContainer.Resources.Limits.Cpu().String()).To(Equal("1"))
			Expect(serverContainer.Resources.Limits.Memory().String()).To(Equal("1Gi"))
		})
	})
})

// Describe tests that don't use envtest
var _ = Describe("isPaused Function", func() {
	It("Should return false for nil annotations", func() {
		server := &quakekubeiov1alpha1.QuakeServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		Expect(isPaused(server)).To(BeFalse())
	})

	It("Should return false for empty annotations", func() {
		server := &quakekubeiov1alpha1.QuakeServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{},
			},
		}
		Expect(isPaused(server)).To(BeFalse())
	})

	It("Should return true when paused annotation is true", func() {
		server := &quakekubeiov1alpha1.QuakeServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					"quakekube.io/paused": "true",
				},
			},
		}
		Expect(isPaused(server)).To(BeTrue())
	})

	It("Should return false when paused annotation is false", func() {
		server := &quakekubeiov1alpha1.QuakeServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					"quakekube.io/paused": "false",
				},
			},
		}
		Expect(isPaused(server)).To(BeFalse())
	})

	It("Should return false for invalid annotation value", func() {
		server := &quakekubeiov1alpha1.QuakeServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					"quakekube.io/paused": "yes",
				},
			},
		}
		Expect(isPaused(server)).To(BeFalse())
	})
})

// Test game types
var _ = Describe("Game Types", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	It("Should support all game types", func() {
		// Map game types to valid DNS names (lowercase)
		gameTypeTests := []struct {
			gameType quakekubeiov1alpha1.GameType
			name     string
		}{
			{quakekubeiov1alpha1.FreeForAll, "ffa"},
			{quakekubeiov1alpha1.Tournament, "tournament"},
			{quakekubeiov1alpha1.SinglePlayer, "sp"},
			{quakekubeiov1alpha1.TeamDeathmatch, "tdm"},
			{quakekubeiov1alpha1.CaptureTheFlag, "ctf"},
		}

		for _, tt := range gameTypeTests {
			serverName := "test-game-type-" + tt.name

			// Clean up first
			existing := &quakekubeiov1alpha1.QuakeServer{}
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, existing); err == nil {
				Expect(k8sClient.Delete(context.Background(), existing)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, existing)
					return errors.IsNotFound(err)
				}, timeout, interval).Should(BeTrue())
			}

			server := &quakekubeiov1alpha1.QuakeServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: serverName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerSpec{
					AgreeEula: true,
					ServerConfig: &quakekubeiov1alpha1.ServerConfigSpec{
						Game: &quakekubeiov1alpha1.GameConfigSpec{
							Type: tt.gameType,
						},
					},
				},
			}

			Expect(k8sClient.Create(context.Background(), server)).To(Succeed(), "Failed to create server for game type: %s", tt.gameType)

			createdServer := &quakekubeiov1alpha1.QuakeServer{}
			Eventually(func() error {
				return k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, createdServer)
			}, timeout, interval).Should(Succeed())

			Expect(createdServer.Spec.ServerConfig.Game.Type).To(Equal(tt.gameType))

			// Cleanup
			Expect(k8sClient.Delete(context.Background(), server)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: serverName}, existing)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		}
	})
})

// Test template index functionality
var _ = Describe("Template Index", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When servers reference a template", func() {
		const (
			templateName = "index-test-template"
			serverName1  = "index-test-server-1"
			serverName2  = "index-test-server-2"
		)

		AfterEach(func() {
			ctx := context.Background()

			for _, name := range []string{serverName1, serverName2} {
				server := &quakekubeiov1alpha1.QuakeServer{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, server); err == nil {
					Expect(k8sClient.Delete(ctx, server)).To(Succeed())
				}
			}

			template := &quakekubeiov1alpha1.QuakeServerTemplate{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: templateName}, template); err == nil {
				Expect(k8sClient.Delete(ctx, template)).To(Succeed())
			}
		})

		It("Should find servers by template reference using index", func() {
			ctx := context.Background()

			// Create template
			template := &quakekubeiov1alpha1.QuakeServerTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: templateName,
				},
				Spec: quakekubeiov1alpha1.QuakeServerTemplateSpec{
					ServerConfig: &quakekubeiov1alpha1.ServerConfigSpec{
						FragLimit: intPtr(25),
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			// Create servers referencing the template
			for _, name := range []string{serverName1, serverName2} {
				server := &quakekubeiov1alpha1.QuakeServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
					Spec: quakekubeiov1alpha1.QuakeServerSpec{
						AgreeEula: true,
						TemplateRef: &quakekubeiov1alpha1.TemplateReference{
							Name: templateName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, server)).To(Succeed())
			}

			// Verify servers exist and reference the template
			for _, name := range []string{serverName1, serverName2} {
				server := &quakekubeiov1alpha1.QuakeServer{}
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: name}, server)
				}, timeout, interval).Should(Succeed())
				Expect(server.Spec.TemplateRef.Name).To(Equal(templateName))
			}

			// Use the index to find servers
			serverList := &quakekubeiov1alpha1.QuakeServerList{}
			Eventually(func() int {
				err := k8sClient.List(ctx, serverList)
				if err != nil {
					return 0
				}
				count := 0
				for _, s := range serverList.Items {
					if s.Spec.TemplateRef != nil && s.Spec.TemplateRef.Name == templateName {
						count++
					}
				}
				return count
			}, timeout, interval).Should(Equal(2))
		})
	})
})
