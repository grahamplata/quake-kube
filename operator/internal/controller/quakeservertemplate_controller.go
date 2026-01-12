package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	quakekubeiov1alpha1 "github.com/grahamplata/quake-kube/operator/api/v1alpha1"
)

// TemplateRefIndexField is the index field name for QuakeServer.spec.templateRef
const TemplateRefIndexField = ".spec.templateRef"

// QuakeServerTemplateReconciler reconciles a QuakeServerTemplate object
type QuakeServerTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservertemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservertemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservertemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=quakekube.io,resources=quakeservers,verbs=get;list;watch

// Reconcile handles QuakeServerTemplate reconciliation.
// When a template is updated, it finds all QuakeServers referencing this template
// and updates the template's status with the list of dependent servers.
func (r *QuakeServerTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling QuakeServerTemplate", "name", req.Name)

	// Fetch the QuakeServerTemplate
	template := &quakekubeiov1alpha1.QuakeServerTemplate{}
	if err := r.Get(ctx, req.NamespacedName, template); err != nil {
		// Template was deleted, nothing to do
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Find all QuakeServers referencing this template
	serverList := &quakekubeiov1alpha1.QuakeServerList{}
	if err := r.List(ctx, serverList, client.MatchingFields{
		TemplateRefIndexField: template.Name,
	}); err != nil {
		logger.Error(err, "Failed to list QuakeServers using template")
		return ctrl.Result{}, err
	}

	// Build the list of server names using this template
	usedBy := make([]string, 0, len(serverList.Items))
	for _, server := range serverList.Items {
		usedBy = append(usedBy, server.Name)
	}

	// Update status.usedBy
	template.Status.UsedBy = usedBy

	// Update condition
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "TemplateReady",
		Message:            fmt.Sprintf("Template is ready, used by %d server(s)", len(usedBy)),
		ObservedGeneration: template.Generation,
	}
	meta.SetStatusCondition(&template.Status.Conditions, condition)

	// Update the template status
	if err := r.Status().Update(ctx, template); err != nil {
		logger.Error(err, "Failed to update QuakeServerTemplate status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled QuakeServerTemplate",
		"name", template.Name,
		"usedBy", len(usedBy))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
// It creates an index on QuakeServer.spec.templateRef to enable efficient lookups
// and watches both QuakeServerTemplates and QuakeServers that reference templates.
func (r *QuakeServerTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create an index on QuakeServer.spec.templateRef for efficient lookups
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&quakekubeiov1alpha1.QuakeServer{},
		TemplateRefIndexField,
		func(obj client.Object) []string {
			server := obj.(*quakekubeiov1alpha1.QuakeServer)
			if server.Spec.TemplateRef == nil {
				return nil
			}
			return []string{server.Spec.TemplateRef.Name}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&quakekubeiov1alpha1.QuakeServerTemplate{}).
		// Watch QuakeServers and enqueue their referenced template for reconciliation
		Watches(
			&quakekubeiov1alpha1.QuakeServer{},
			handler.EnqueueRequestsFromMapFunc(r.findTemplateForServer),
		).
		Complete(r)
}

// findTemplateForServer returns a reconcile request for the template referenced by a QuakeServer.
// This enables the template controller to update its status.usedBy when servers are created/deleted.
func (r *QuakeServerTemplateReconciler) findTemplateForServer(ctx context.Context, obj client.Object) []reconcile.Request {
	server := obj.(*quakekubeiov1alpha1.QuakeServer)
	if server.Spec.TemplateRef == nil {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: client.ObjectKey{
				Name: server.Spec.TemplateRef.Name,
			},
		},
	}
}
