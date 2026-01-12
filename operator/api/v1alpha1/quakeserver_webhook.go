package v1alpha1

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var quakeserverlog = logf.Log.WithName("quakeserver-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *QuakeServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(r).
		WithDefaulter(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-quakekube-io-v1alpha1-quakeserver,mutating=true,failurePolicy=fail,sideEffects=None,groups=quakekube.io,resources=quakeservers,verbs=create;update,versions=v1alpha1,name=mquakeserver.kb.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &QuakeServer{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (r *QuakeServer) Default(ctx context.Context, obj runtime.Object) error {
	server := obj.(*QuakeServer)
	quakeserverlog.Info("default", "name", server.Name)

	// Set default game type if not specified
	if server.Spec.ServerConfig != nil && server.Spec.ServerConfig.Game != nil {
		if server.Spec.ServerConfig.Game.Type == "" {
			server.Spec.ServerConfig.Game.Type = FreeForAll
		}
	}

	return nil
}

//+kubebuilder:webhook:path=/validate-quakekube-io-v1alpha1-quakeserver,mutating=false,failurePolicy=fail,sideEffects=None,groups=quakekube.io,resources=quakeservers,verbs=create;update,versions=v1alpha1,name=vquakeserver.kb.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &QuakeServer{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *QuakeServer) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	server := obj.(*QuakeServer)
	quakeserverlog.Info("validate create", "name", server.Name)

	return nil, server.validateQuakeServer()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *QuakeServer) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	server := newObj.(*QuakeServer)
	quakeserverlog.Info("validate update", "name", server.Name)

	return nil, server.validateQuakeServer()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *QuakeServer) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	server := obj.(*QuakeServer)
	quakeserverlog.Info("validate delete", "name", server.Name)

	// No validation needed on delete
	return nil, nil
}

// validateQuakeServer validates the QuakeServer spec
func (r *QuakeServer) validateQuakeServer() error {
	var allErrs []string

	// Validate agreeEula is true
	if !r.Spec.AgreeEula {
		allErrs = append(allErrs, "spec.agreeEula must be set to true to accept the Quake 3 EULA")
	}

	// Validate templateRef if specified
	if r.Spec.TemplateRef != nil && r.Spec.TemplateRef.Name == "" {
		allErrs = append(allErrs, "spec.templateRef.name cannot be empty when templateRef is specified")
	}

	// Validate gateway configuration
	if r.Spec.Gateway != nil && r.Spec.Gateway.Enabled {
		if r.Spec.Gateway.GatewayRef == nil {
			allErrs = append(allErrs, "spec.gateway.gatewayRef is required when gateway is enabled")
		} else if r.Spec.Gateway.GatewayRef.Name == "" {
			allErrs = append(allErrs, "spec.gateway.gatewayRef.name cannot be empty")
		}
	}

	if len(allErrs) > 0 {
		return &ValidationError{Errors: allErrs}
	}
	return nil
}

// ValidationError represents validation errors
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	result := "validation failed:"
	for _, err := range e.Errors {
		result += "\n  - " + err
	}
	return result
}
