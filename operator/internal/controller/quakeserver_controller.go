package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	quakekubeiov1alpha1 "github.com/grahamplata/quake-kube/operator/api/v1alpha1"
	"github.com/grahamplata/quake-kube/operator/internal/quakenet"
)

const (
	// Finalizer for QuakeServer cleanup
	quakeServerFinalizer = "quakekube.io/finalizer"

	// Annotation for pausing/resuming the server
	pauseAnnotation = "quakekube.io/paused"

	// Default ports
	httpPort    = 8080
	gamePort    = 27960
	contentPort = 9090

	// Default image
	defaultImage = "ghcr.io/grahamplata/quake-kube:latest"

	// Config paths
	configMountPath = "/config"
	assetsMountPath = "/assets"

	// Status refresh interval for player count updates
	statusRefreshInterval = 30 * time.Second
)

// QuakeServerReconciler reconciles a QuakeServer object
type QuakeServerReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Recorder    record.EventRecorder
	QuakeClient *quakenet.Client
}

//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservertemplates,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=udproutes,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *QuakeServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling QuakeServer", "name", req.Name)

	// Fetch the QuakeServer
	server := &quakekubeiov1alpha1.QuakeServer{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("QuakeServer not found, likely deleted")
			// Decrement active total if possible, or we rely on periodic listing
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get QuakeServer")
		QuakeServerReconciliationTotal.WithLabelValues("failure").Inc()
		return ctrl.Result{}, err
	}

	// Update active total gauge (approximate via current reconcile)
	// For high accuracy, a background lister is better, but this works for basic tracking
	var serverList quakekubeiov1alpha1.QuakeServerList
	if err := r.List(ctx, &serverList); err == nil {
		QuakeServerActiveTotal.Set(float64(len(serverList.Items)))
	}

	// Handle deletion with finalizer
	if !server.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, server)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(server, quakeServerFinalizer) {
		controllerutil.AddFinalizer(server, quakeServerFinalizer)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Resolve template if referenced
	resolvedConfig, err := r.resolveConfig(ctx, server)
	if err != nil {
		logger.Error(err, "Failed to resolve server configuration")
		r.Recorder.Eventf(server, corev1.EventTypeWarning, "TemplateResolutionFailed",
			"Failed to resolve template: %v", err)
		r.setCondition(server, "TemplateResolved", metav1.ConditionFalse, "ResolutionFailed", err.Error())
		if statusErr := r.Status().Update(ctx, server); statusErr != nil {
			logger.Error(statusErr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	// Set template resolved condition
	if server.Spec.TemplateRef != nil {
		server.Status.TemplateRef = server.Spec.TemplateRef.Name
		r.setCondition(server, "TemplateResolved", metav1.ConditionTrue, "TemplateFound",
			fmt.Sprintf("Using template %s", server.Spec.TemplateRef.Name))
		r.Recorder.Eventf(server, corev1.EventTypeNormal, "TemplateResolved",
			"Successfully resolved template %s", server.Spec.TemplateRef.Name)
	}

	// Reconcile ConfigMap
	if err := r.reconcileConfigMap(ctx, server, resolvedConfig); err != nil {
		logger.Error(err, "Failed to reconcile ConfigMap")
		r.Recorder.Eventf(server, corev1.EventTypeWarning, "ConfigMapReconcileFailed",
			"Failed to reconcile ConfigMap: %v", err)
		return ctrl.Result{}, err
	}

	// Reconcile Service
	if err := r.reconcileService(ctx, server); err != nil {
		logger.Error(err, "Failed to reconcile Service")
		r.Recorder.Eventf(server, corev1.EventTypeWarning, "ServiceReconcileFailed",
			"Failed to reconcile Service: %v", err)
		return ctrl.Result{}, err
	}

	// Reconcile Deployment
	if err := r.reconcileDeployment(ctx, server); err != nil {
		logger.Error(err, "Failed to reconcile Deployment")
		r.Recorder.Eventf(server, corev1.EventTypeWarning, "DeploymentReconcileFailed",
			"Failed to reconcile Deployment: %v", err)
		return ctrl.Result{}, err
	}

	// Reconcile Gateway API routes if enabled
	if server.Spec.Gateway != nil && server.Spec.Gateway.Enabled {
		if err := r.reconcileHTTPRoute(ctx, server); err != nil {
			logger.Error(err, "Failed to reconcile HTTPRoute")
			r.Recorder.Eventf(server, corev1.EventTypeWarning, "HTTPRouteReconcileFailed",
				"Failed to reconcile HTTPRoute: %v", err)
			return ctrl.Result{}, err
		}
		if err := r.reconcileUDPRoute(ctx, server); err != nil {
			logger.Error(err, "Failed to reconcile UDPRoute")
			r.Recorder.Eventf(server, corev1.EventTypeWarning, "UDPRouteReconcileFailed",
				"Failed to reconcile UDPRoute: %v", err)
			return ctrl.Result{}, err
		}
	}

	// Update status based on Deployment (including player count from live server)
	serverReady, err := r.updateStatus(ctx, server)
	if err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Record successful reconciliation event
	r.Recorder.Event(server, corev1.EventTypeNormal, "Reconciled", "Successfully reconciled QuakeServer")

	QuakeServerReconciliationTotal.WithLabelValues("success").Inc()
	logger.Info("Successfully reconciled QuakeServer", "name", server.Name)

	// Requeue periodically to refresh player count when server is ready
	if serverReady {
		return ctrl.Result{RequeueAfter: statusRefreshInterval}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileDelete handles cleanup when a QuakeServer is being deleted
func (r *QuakeServerReconciler) reconcileDelete(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling QuakeServer deletion", "name", server.Name)

	// Record deletion event
	r.Recorder.Event(server, corev1.EventTypeNormal, "Deleting", "Cleaning up QuakeServer resources")

	// Owned resources (Deployment, Service, ConfigMap) will be garbage collected
	// due to OwnerReferences - no explicit cleanup needed

	// Remove finalizer
	controllerutil.RemoveFinalizer(server, quakeServerFinalizer)
	if err := r.Update(ctx, server); err != nil {
		return ctrl.Result{}, err
	}

	// Update active total gauge after deletion
	var serverList quakekubeiov1alpha1.QuakeServerList
	if err := r.List(ctx, &serverList); err == nil {
		QuakeServerActiveTotal.Set(float64(len(serverList.Items)))
	}

	// Remove player count metric for this server
	QuakeServerPlayersActive.DeleteLabelValues(server.Name, "default")

	logger.Info("Successfully cleaned up QuakeServer", "name", server.Name)
	return ctrl.Result{}, nil
}

// resolveConfig merges template config with inline config
func (r *QuakeServerReconciler) resolveConfig(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) (*quakekubeiov1alpha1.ServerConfigSpec, error) {
	// Start with inline config or empty
	result := &quakekubeiov1alpha1.ServerConfigSpec{}
	if server.Spec.ServerConfig != nil {
		result = server.Spec.ServerConfig.DeepCopy()
	}

	// If no template reference, return inline config
	if server.Spec.TemplateRef == nil {
		return result, nil
	}

	// Fetch the template
	template := &quakekubeiov1alpha1.QuakeServerTemplate{}
	templateKey := client.ObjectKey{Name: server.Spec.TemplateRef.Name}
	if err := r.Get(ctx, templateKey, template); err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("template %s not found", server.Spec.TemplateRef.Name)
		}
		return nil, err
	}

	// Merge template values with inline overrides (inline takes precedence)
	if template.Spec.ServerConfig != nil {
		result = mergeServerConfig(template.Spec.ServerConfig, result)
	}

	return result, nil
}

// mergeServerConfig merges base config with overrides (overrides take precedence)
func mergeServerConfig(base, override *quakekubeiov1alpha1.ServerConfigSpec) *quakekubeiov1alpha1.ServerConfigSpec {
	if override == nil {
		return base.DeepCopy()
	}
	if base == nil {
		return override.DeepCopy()
	}

	result := base.DeepCopy()

	// Override scalar fields if set
	if override.FragLimit != nil {
		result.FragLimit = override.FragLimit
	}
	if override.TimeLimit != nil {
		result.TimeLimit = override.TimeLimit
	}
	if len(override.Maps) > 0 {
		result.Maps = override.Maps
	}
	if len(override.Commands) > 0 {
		result.Commands = override.Commands
	}

	// Merge nested structs
	if override.Game != nil {
		if result.Game == nil {
			result.Game = &quakekubeiov1alpha1.GameConfigSpec{}
		}
		mergeGameConfig(result.Game, override.Game)
	}
	if override.Server != nil {
		if result.Server == nil {
			result.Server = &quakekubeiov1alpha1.ServerSettings{}
		}
		mergeServerSettings(result.Server, override.Server)
	}
	if override.Bot != nil {
		if result.Bot == nil {
			result.Bot = &quakekubeiov1alpha1.BotConfigSpec{}
		}
		mergeBotConfig(result.Bot, override.Bot)
	}

	return result
}

func mergeGameConfig(base, override *quakekubeiov1alpha1.GameConfigSpec) {
	if override.Type != "" {
		base.Type = override.Type
	}
	if override.MOTD != "" {
		base.MOTD = override.MOTD
	}
	if override.ForceRespawn {
		base.ForceRespawn = override.ForceRespawn
	}
	if override.Inactivity != nil {
		base.Inactivity = override.Inactivity
	}
	if override.QuadFactor != nil {
		base.QuadFactor = override.QuadFactor
	}
	if override.Password != "" {
		base.Password = override.Password
	}
	if override.WeaponRespawn != nil {
		base.WeaponRespawn = override.WeaponRespawn
	}
}

func mergeServerSettings(base, override *quakekubeiov1alpha1.ServerSettings) {
	if override.Hostname != "" {
		base.Hostname = override.Hostname
	}
	if override.MaxClients != nil {
		base.MaxClients = override.MaxClients
	}
	if override.RconPassword != "" {
		base.RconPassword = override.RconPassword
	}
}

func mergeBotConfig(base, override *quakekubeiov1alpha1.BotConfigSpec) {
	if override.MinPlayers != nil {
		base.MinPlayers = override.MinPlayers
	}
	if override.Skill != nil {
		base.Skill = override.Skill
	}
	if override.NoChat {
		base.NoChat = override.NoChat
	}
}

// reconcileConfigMap creates or updates the ConfigMap for server configuration
func (r *QuakeServerReconciler) reconcileConfigMap(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer, config *quakekubeiov1alpha1.ServerConfigSpec) error {
	logger := log.FromContext(ctx)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName(server),
			Namespace: "default", // Cluster-scoped resources need a namespace for child resources
			Labels:    labelsForServer(server),
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		// Set owner reference for garbage collection
		if err := controllerutil.SetControllerReference(server, configMap, r.Scheme); err != nil {
			return err
		}

		// Generate config YAML
		configYAML := generateConfigYAML(config)
		configMap.Data = map[string]string{
			"config.yaml": configYAML,
		}
		return nil
	})

	if err != nil {
		return err
	}

	logger.Info("Reconciled ConfigMap", "name", configMap.Name, "operation", op)
	return nil
}

// generateConfigYAML creates the YAML configuration for the Quake server
func generateConfigYAML(config *quakekubeiov1alpha1.ServerConfigSpec) string {
	if config == nil {
		return "fragLimit: 25\ntimeLimit: 15m\n"
	}

	yaml := ""

	// FragLimit
	fragLimit := 25
	if config.FragLimit != nil {
		fragLimit = *config.FragLimit
	}
	yaml += fmt.Sprintf("fragLimit: %d\n", fragLimit)

	// TimeLimit
	timeLimit := "15m"
	if config.TimeLimit != nil {
		timeLimit = config.TimeLimit.Duration.String()
	}
	yaml += fmt.Sprintf("timeLimit: %s\n", timeLimit)

	// Bot config
	if config.Bot != nil {
		yaml += "bot:\n"
		if config.Bot.MinPlayers != nil {
			yaml += fmt.Sprintf("  minPlayers: %d\n", *config.Bot.MinPlayers)
		}
		if config.Bot.Skill != nil {
			yaml += fmt.Sprintf("  skill: %d\n", *config.Bot.Skill)
		}
		if config.Bot.NoChat {
			yaml += "  noChat: true\n"
		}
	}

	// Game config
	if config.Game != nil {
		yaml += "game:\n"
		if config.Game.Type != "" {
			yaml += fmt.Sprintf("  type: %s\n", config.Game.Type)
		}
		if config.Game.MOTD != "" {
			yaml += fmt.Sprintf("  motd: %q\n", config.Game.MOTD)
		}
		if config.Game.ForceRespawn {
			yaml += "  forceRespawn: true\n"
		}
		if config.Game.Inactivity != nil {
			yaml += fmt.Sprintf("  inactivity: %s\n", config.Game.Inactivity.Duration.String())
		}
		if config.Game.QuadFactor != nil {
			yaml += fmt.Sprintf("  quadFactor: %d\n", *config.Game.QuadFactor)
		}
		if config.Game.Password != "" {
			yaml += fmt.Sprintf("  password: %q\n", config.Game.Password)
		}
		if config.Game.WeaponRespawn != nil {
			yaml += fmt.Sprintf("  weaponRespawn: %d\n", *config.Game.WeaponRespawn)
		}
	}

	// Server config
	if config.Server != nil {
		yaml += "server:\n"
		if config.Server.Hostname != "" {
			yaml += fmt.Sprintf("  hostname: %q\n", config.Server.Hostname)
		}
		if config.Server.MaxClients != nil {
			yaml += fmt.Sprintf("  maxClients: %d\n", *config.Server.MaxClients)
		}
		if config.Server.RconPassword != "" {
			yaml += fmt.Sprintf("  password: %q\n", config.Server.RconPassword)
		}
	}

	// Maps
	if len(config.Maps) > 0 {
		yaml += "maps:\n"
		for _, m := range config.Maps {
			yaml += fmt.Sprintf("  - name: %s\n", m.Name)
			if m.Type != "" {
				yaml += fmt.Sprintf("    type: %s\n", m.Type)
			}
			if m.CaptureLimit != nil {
				yaml += fmt.Sprintf("    captureLimit: %d\n", *m.CaptureLimit)
			}
			if m.FragLimit != nil {
				yaml += fmt.Sprintf("    fragLimit: %d\n", *m.FragLimit)
			}
			if m.TimeLimit != nil {
				yaml += fmt.Sprintf("    timeLimit: %s\n", m.TimeLimit.Duration.String())
			}
		}
	}

	// Commands
	if len(config.Commands) > 0 {
		yaml += "commands:\n"
		for _, cmd := range config.Commands {
			yaml += fmt.Sprintf("  - %q\n", cmd)
		}
	}

	return yaml
}

// reconcileService creates or updates the Service for the QuakeServer
func (r *QuakeServerReconciler) reconcileService(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) error {
	logger := log.FromContext(ctx)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName(server),
			Namespace: "default",
			Labels:    labelsForServer(server),
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := controllerutil.SetControllerReference(server, service, r.Scheme); err != nil {
			return err
		}

		service.Spec = corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selectorLabelsForServer(server),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       httpPort,
					TargetPort: intstr.FromInt(httpPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "game",
					Port:       gamePort,
					TargetPort: intstr.FromInt(gamePort),
					Protocol:   corev1.ProtocolUDP,
				},
				{
					Name:       "content",
					Port:       contentPort,
					TargetPort: intstr.FromInt(contentPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		}
		return nil
	})

	if err != nil {
		return err
	}

	logger.Info("Reconciled Service", "name", service.Name, "operation", op)
	return nil
}

// reconcileDeployment creates or updates the Deployment for the QuakeServer
func (r *QuakeServerReconciler) reconcileDeployment(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) error {
	logger := log.FromContext(ctx)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName(server),
			Namespace: "default",
			Labels:    labelsForServer(server),
		},
	}

	paused := isPaused(server)

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		if err := controllerutil.SetControllerReference(server, deployment, r.Scheme); err != nil {
			return err
		}

		// Determine replica count based on pause state
		replicas := int32(1)
		if paused {
			replicas = 0
			logger.Info("Server is paused, scaling to 0 replicas", "name", server.Name)
		}

		deployment.Spec = appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabelsForServer(server),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labelsForServer(server),
				},
				Spec: r.buildPodSpec(server),
			},
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Record events for deployment state changes
	switch op {
	case controllerutil.OperationResultCreated:
		if paused {
			r.Recorder.Event(server, corev1.EventTypeNormal, "ServerPaused", "Server created in paused state")
		} else {
			r.Recorder.Event(server, corev1.EventTypeNormal, "ServerStarted", "Server deployment created")
		}
	case controllerutil.OperationResultUpdated:
		if paused {
			r.Recorder.Event(server, corev1.EventTypeNormal, "ServerPaused", "Server paused, scaling to 0 replicas")
		}
	}

	logger.Info("Reconciled Deployment", "name", deployment.Name, "operation", op)
	return nil
}

// buildPodSpec creates the pod spec for the QuakeServer deployment
func (r *QuakeServerReconciler) buildPodSpec(server *quakekubeiov1alpha1.QuakeServer) corev1.PodSpec {
	// Build command args
	args := []string{
		"server",
		"--config=/config/config.yaml",
		fmt.Sprintf("--assets-dir=%s", assetsMountPath),
		fmt.Sprintf("--content-server=http://127.0.0.1:%d", contentPort),
	}
	if server.Spec.AgreeEula {
		args = append(args, "--agree-eula")
	}

	// Build container security context with secure defaults
	containerSecurityContext := buildContainerSecurityContext(server.Spec.SecurityContext)

	// Server container
	serverContainer := corev1.Container{
		Name:            "server",
		Image:           defaultImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"q3"},
		Args:            args,
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: httpPort, Protocol: corev1.ProtocolTCP},
			{Name: "game", ContainerPort: gamePort, Protocol: corev1.ProtocolUDP},
		},
		SecurityContext: containerSecurityContext,
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(httpPort),
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       5,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(httpPort),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "config", MountPath: configMountPath, ReadOnly: true},
			{Name: "assets", MountPath: assetsMountPath},
		},
	}

	// Apply resource limits if specified
	if server.Spec.Resources != nil {
		serverContainer.Resources = *server.Spec.Resources
	} else {
		// Default resource limits
		serverContainer.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		}
	}

	// Content server sidecar container
	contentContainer := corev1.Container{
		Name:            "content-server",
		Image:           defaultImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"q3"},
		Args: []string{
			"content",
			fmt.Sprintf("--addr=:%d", contentPort),
			fmt.Sprintf("--assets-dir=%s", assetsMountPath),
		},
		Ports: []corev1.ContainerPort{
			{Name: "content", ContainerPort: contentPort, Protocol: corev1.ProtocolTCP},
		},
		SecurityContext: containerSecurityContext,
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(contentPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(contentPort),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "assets", MountPath: assetsMountPath},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{serverContainer, contentContainer},
		Volumes: []corev1.Volume{
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName(server),
						},
					},
				},
			},
			{
				Name: "assets",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		// Apply default pod security context with secure settings
		SecurityContext: buildPodSecurityContext(server.Spec.PodSecurityContext),
	}

	return podSpec
}

// reconcileHTTPRoute creates or updates the HTTPRoute for browser WebSocket traffic
func (r *QuakeServerReconciler) reconcileHTTPRoute(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) error {
	logger := log.FromContext(ctx)

	if server.Spec.Gateway == nil || server.Spec.Gateway.Hostname == "" {
		logger.Info("Skipping HTTPRoute creation: no hostname specified")
		return nil
	}

	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpRouteName(server),
			Namespace: "default",
			Labels:    labelsForServer(server),
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, httpRoute, func() error {
		if err := controllerutil.SetControllerReference(server, httpRoute, r.Scheme); err != nil {
			return err
		}

		// Build parent references
		var parentRefs []gatewayv1.ParentReference
		if server.Spec.Gateway.GatewayRef != nil {
			namespace := gatewayv1.Namespace(server.Spec.Gateway.GatewayRef.Namespace)
			if server.Spec.Gateway.GatewayRef.Namespace == "" {
				namespace = gatewayv1.Namespace("default")
			}
			parentRefs = []gatewayv1.ParentReference{
				{
					Name:      gatewayv1.ObjectName(server.Spec.Gateway.GatewayRef.Name),
					Namespace: &namespace,
				},
			}
		}

		// Build hostname
		hostname := gatewayv1.Hostname(server.Spec.Gateway.Hostname)

		// Build backend reference to the service
		servicePort := gatewayv1.PortNumber(httpPort)
		httpRoute.Spec = gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Hostnames: []gatewayv1.Hostname{hostname},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: gatewayv1.ObjectName(serviceName(server)),
									Port: &servicePort,
								},
							},
						},
					},
				},
			},
		}
		return nil
	})

	if err != nil {
		return err
	}

	logger.Info("Reconciled HTTPRoute", "name", httpRoute.Name, "operation", op)
	return nil
}

// reconcileUDPRoute creates or updates the UDPRoute for native Quake client traffic
func (r *QuakeServerReconciler) reconcileUDPRoute(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) error {
	logger := log.FromContext(ctx)

	if server.Spec.Gateway == nil || server.Spec.Gateway.GatewayRef == nil {
		logger.Info("Skipping UDPRoute creation: no gateway reference specified")
		return nil
	}

	udpRoute := &gatewayv1alpha2.UDPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      udpRouteName(server),
			Namespace: "default",
			Labels:    labelsForServer(server),
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, udpRoute, func() error {
		if err := controllerutil.SetControllerReference(server, udpRoute, r.Scheme); err != nil {
			return err
		}

		// Build parent references
		namespace := gatewayv1.Namespace(server.Spec.Gateway.GatewayRef.Namespace)
		if server.Spec.Gateway.GatewayRef.Namespace == "" {
			namespace = gatewayv1.Namespace("default")
		}
		parentRefs := []gatewayv1.ParentReference{
			{
				Name:      gatewayv1.ObjectName(server.Spec.Gateway.GatewayRef.Name),
				Namespace: &namespace,
			},
		}

		// Build backend reference to the service
		servicePort := gatewayv1.PortNumber(gamePort)
		udpRoute.Spec = gatewayv1alpha2.UDPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Rules: []gatewayv1alpha2.UDPRouteRule{
				{
					BackendRefs: []gatewayv1alpha2.BackendRef{
						{
							BackendObjectReference: gatewayv1.BackendObjectReference{
								Name: gatewayv1.ObjectName(serviceName(server)),
								Port: &servicePort,
							},
						},
					},
				},
			},
		}
		return nil
	})

	if err != nil {
		return err
	}

	logger.Info("Reconciled UDPRoute", "name", udpRoute.Name, "operation", op)
	return nil
}

// updateStatus updates the QuakeServer status based on the Deployment state
// and queries the live server for player count. Returns true if the server is ready.
func (r *QuakeServerReconciler) updateStatus(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) (bool, error) {
	logger := log.FromContext(ctx)

	// Check if server is paused
	paused := isPaused(server)
	if paused {
		r.setCondition(server, "Paused", metav1.ConditionTrue, "ServerPaused", "Server is paused via annotation")
	} else {
		r.setCondition(server, "Paused", metav1.ConditionFalse, "ServerRunning", "Server is not paused")
	}

	var serverReady bool

	// Get the deployment
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentName(server), Namespace: "default"}, deployment); err != nil {
		if errors.IsNotFound(err) {
			server.Status.Ready = false
			server.Status.Players = 0
			r.setCondition(server, "Ready", metav1.ConditionFalse, "DeploymentNotFound", "Deployment does not exist")
		} else {
			return false, err
		}
	} else {
		// Check deployment readiness
		ready := deployment.Status.ReadyReplicas > 0
		server.Status.Ready = ready

		if paused {
			// When paused, set Ready to false with appropriate message
			server.Status.Ready = false
			server.Status.Players = 0
			r.setCondition(server, "Ready", metav1.ConditionFalse, "ServerPaused", "Server is paused and not accepting connections")
		} else if ready {
			serverReady = true
			r.setCondition(server, "Ready", metav1.ConditionTrue, "ServerReady", "Server is running and accepting connections")

			// Query the live server for player count
			playerCount, err := r.queryPlayerCount(ctx, server)
			if err != nil {
				logger.V(1).Info("Failed to query player count from server", "error", err)
				// Keep previous player count or set to 0 if we can't query
				// Don't fail the reconciliation for this
			} else {
				server.Status.Players = playerCount
				logger.V(1).Info("Updated player count from live server", "players", playerCount)
				// Update Prometheus metric
				QuakeServerPlayersActive.WithLabelValues(server.Name, "default").Set(float64(playerCount))
			}
		} else {
			server.Status.Players = 0
			// Set metric to 0 if server is not ready or paused
			QuakeServerPlayersActive.WithLabelValues(server.Name, "default").Set(0)
			r.setCondition(server, "Ready", metav1.ConditionFalse, "ServerNotReady",
				fmt.Sprintf("Waiting for replicas: %d/%d ready", deployment.Status.ReadyReplicas, *deployment.Spec.Replicas))
		}
	}

	// Set endpoint and game port
	if server.Spec.Gateway != nil && server.Spec.Gateway.Hostname != "" {
		server.Status.Endpoint = fmt.Sprintf("https://%s", server.Spec.Gateway.Hostname)
	}
	server.Status.GamePort = gamePort

	return serverReady, r.Status().Update(ctx, server)
}

// queryPlayerCount queries the Quake server for the current player count
func (r *QuakeServerReconciler) queryPlayerCount(ctx context.Context, server *quakekubeiov1alpha1.QuakeServer) (int, error) {
	logger := log.FromContext(ctx)

	// Skip if no quake client configured
	if r.QuakeClient == nil {
		return 0, fmt.Errorf("quake client not configured")
	}

	// Get the service to find its ClusterIP
	service := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: serviceName(server), Namespace: "default"}, service); err != nil {
		return 0, fmt.Errorf("getting service: %w", err)
	}

	// Build the address to query (ClusterIP:gamePort)
	addr := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, gamePort)
	logger.V(1).Info("Querying Quake server for player count", "address", addr)

	// Query the server
	playerCount, err := r.QuakeClient.GetPlayerCount(addr)
	if err != nil {
		return 0, fmt.Errorf("querying server at %s: %w", addr, err)
	}

	return playerCount, nil
}

// setCondition sets a condition on the QuakeServer status
func (r *QuakeServerReconciler) setCondition(server *quakekubeiov1alpha1.QuakeServer, condType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: server.Generation,
	}
	meta.SetStatusCondition(&server.Status.Conditions, condition)
}

// buildContainerSecurityContext returns a container security context with secure defaults.
// If a custom context is provided, it is used; otherwise, secure defaults are applied.
func buildContainerSecurityContext(custom *corev1.SecurityContext) *corev1.SecurityContext {
	if custom != nil {
		return custom
	}

	// Secure defaults: drop all capabilities, run as non-root, read-only root filesystem
	runAsNonRoot := true
	allowPrivilegeEscalation := false
	readOnlyRootFilesystem := true

	return &corev1.SecurityContext{
		RunAsNonRoot:             &runAsNonRoot,
		AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

// buildPodSecurityContext returns a pod security context with secure defaults.
// If a custom context is provided, it is used; otherwise, secure defaults are applied.
func buildPodSecurityContext(custom *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	if custom != nil {
		return custom
	}

	// Secure defaults: run as non-root user 1000 with fsGroup 1000
	runAsNonRoot := true
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)

	return &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
		RunAsGroup:   &runAsGroup,
		FSGroup:      &fsGroup,
	}
}

// Helper functions for resource naming
func configMapName(server *quakekubeiov1alpha1.QuakeServer) string {
	return fmt.Sprintf("%s-config", server.Name)
}

func serviceName(server *quakekubeiov1alpha1.QuakeServer) string {
	return server.Name
}

func deploymentName(server *quakekubeiov1alpha1.QuakeServer) string {
	return server.Name
}

func httpRouteName(server *quakekubeiov1alpha1.QuakeServer) string {
	return fmt.Sprintf("%s-http", server.Name)
}

func udpRouteName(server *quakekubeiov1alpha1.QuakeServer) string {
	return fmt.Sprintf("%s-udp", server.Name)
}

func labelsForServer(server *quakekubeiov1alpha1.QuakeServer) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "quake-server",
		"app.kubernetes.io/instance":   server.Name,
		"app.kubernetes.io/managed-by": "quake-operator",
	}
}

func selectorLabelsForServer(server *quakekubeiov1alpha1.QuakeServer) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "quake-server",
		"app.kubernetes.io/instance": server.Name,
	}
}

// isPaused returns true if the server has the pause annotation set to "true"
func isPaused(server *quakekubeiov1alpha1.QuakeServer) bool {
	if server.Annotations == nil {
		return false
	}
	return server.Annotations[pauseAnnotation] == "true"
}

// SetupWithManager sets up the controller with the Manager.
// It watches QuakeServer resources and also watches QuakeServerTemplates,
// triggering reconciliation of all QuakeServers that reference a changed template.
func (r *QuakeServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quakekubeiov1alpha1.QuakeServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Owns(&gatewayv1alpha2.UDPRoute{}).
		// Watch QuakeServerTemplates and enqueue all QuakeServers that reference them
		Watches(
			&quakekubeiov1alpha1.QuakeServerTemplate{},
			handler.EnqueueRequestsFromMapFunc(r.findServersForTemplate),
		).
		Complete(r)
}

// findServersForTemplate returns reconcile requests for all QuakeServers
// that reference the given template. This enables template changes to
// propagate to all dependent servers.
func (r *QuakeServerReconciler) findServersForTemplate(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)
	template := obj.(*quakekubeiov1alpha1.QuakeServerTemplate)

	// Find all QuakeServers referencing this template using the index
	serverList := &quakekubeiov1alpha1.QuakeServerList{}
	if err := r.List(ctx, serverList, client.MatchingFields{
		TemplateRefIndexField: template.Name,
	}); err != nil {
		logger.Error(err, "Failed to list QuakeServers for template", "template", template.Name)
		return nil
	}

	// Build reconcile requests for each dependent server
	requests := make([]reconcile.Request, len(serverList.Items))
	for i, server := range serverList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name: server.Name,
			},
		}
	}

	if len(requests) > 0 {
		logger.Info("Template changed, re-reconciling dependent servers",
			"template", template.Name,
			"serverCount", len(requests))
	}

	return requests
}
