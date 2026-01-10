package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QuakeServerTemplateSpec defines the desired state of QuakeServerTemplate
// Templates provide reusable server configurations that can be referenced by QuakeServer resources.
type QuakeServerTemplateSpec struct {
	// ServerConfig contains the template server configuration.
	// These values serve as defaults and can be overridden by QuakeServer resources.
	// +optional
	ServerConfig *ServerConfigSpec `json:"serverConfig,omitempty"`
}

// QuakeServerTemplateStatus defines the observed state of QuakeServerTemplate
type QuakeServerTemplateStatus struct {
	// UsedBy lists the QuakeServer resources currently using this template.
	// +optional
	UsedBy []string `json:"usedBy,omitempty"`

	// Conditions represent the latest available observations of the template's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Game Type",type="string",JSONPath=".spec.serverConfig.game.type",description="Default game type"
//+kubebuilder:printcolumn:name="Max Clients",type="integer",JSONPath=".spec.serverConfig.server.maxClients",description="Default max clients"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// QuakeServerTemplate is the Schema for the quakeservertemplates API
// It provides reusable server configurations that can be referenced by QuakeServer resources.
type QuakeServerTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuakeServerTemplateSpec   `json:"spec,omitempty"`
	Status QuakeServerTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QuakeServerTemplateList contains a list of QuakeServerTemplate
type QuakeServerTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuakeServerTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QuakeServerTemplate{}, &QuakeServerTemplateList{})
}
