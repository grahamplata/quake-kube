package v1alpha1

import (
	"context"
	"testing"
)

func TestQuakeServerValidation(t *testing.T) {
	tests := []struct {
		name    string
		server  *QuakeServer
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid server with agreeEula true",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server with agreeEula false",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: false,
				},
			},
			wantErr: true,
			errMsg:  "spec.agreeEula must be set to true to accept the Quake 3 EULA",
		},
		{
			name: "valid server with gateway enabled and gatewayRef",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					Gateway: &GatewaySpec{
						Enabled: true,
						GatewayRef: &GatewayReference{
							Name:      "main-gateway",
							Namespace: "gateway-system",
						},
						Hostname: "quake.example.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server with gateway enabled but no gatewayRef",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					Gateway: &GatewaySpec{
						Enabled:  true,
						Hostname: "quake.example.com",
					},
				},
			},
			wantErr: true,
			errMsg:  "spec.gateway.gatewayRef is required when gateway is enabled",
		},
		{
			name: "invalid server with gateway enabled and empty gatewayRef name",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					Gateway: &GatewaySpec{
						Enabled: true,
						GatewayRef: &GatewayReference{
							Name: "",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "spec.gateway.gatewayRef.name cannot be empty",
		},
		{
			name: "valid server with templateRef",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					TemplateRef: &TemplateReference{
						Name: "ffa-template",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server with empty templateRef name",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					TemplateRef: &TemplateReference{
						Name: "",
					},
				},
			},
			wantErr: true,
			errMsg:  "spec.templateRef.name cannot be empty when templateRef is specified",
		},
		{
			name: "valid server with gateway disabled (no gatewayRef required)",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					Gateway: &GatewaySpec{
						Enabled:  false,
						Hostname: "quake.example.com",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.server.validateQuakeServer()
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateQuakeServer() expected error but got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("validateQuakeServer() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateQuakeServer() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestQuakeServerDefault(t *testing.T) {
	tests := []struct {
		name         string
		server       *QuakeServer
		expectedType GameType
	}{
		{
			name: "default game type set when empty",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					ServerConfig: &ServerConfigSpec{
						Game: &GameConfigSpec{
							Type: "",
						},
					},
				},
			},
			expectedType: FreeForAll,
		},
		{
			name: "game type not changed when already set",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					ServerConfig: &ServerConfigSpec{
						Game: &GameConfigSpec{
							Type: Tournament,
						},
					},
				},
			},
			expectedType: Tournament,
		},
		{
			name: "no change when serverConfig is nil",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
				},
			},
			expectedType: "",
		},
		{
			name: "no change when game is nil",
			server: &QuakeServer{
				Spec: QuakeServerSpec{
					AgreeEula: true,
					ServerConfig: &ServerConfigSpec{
						FragLimit: intPtr(30),
					},
				},
			},
			expectedType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &QuakeServer{}
			_ = r.Default(context.Background(), tt.server)

			var actualType GameType
			if tt.server.Spec.ServerConfig != nil && tt.server.Spec.ServerConfig.Game != nil {
				actualType = tt.server.Spec.ServerConfig.Game.Type
			}

			if actualType != tt.expectedType {
				t.Errorf("Default() game type = %v, want %v", actualType, tt.expectedType)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
