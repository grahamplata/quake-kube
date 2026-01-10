# QuakeKube Operator

A Kubernetes Operator for managing Quake 3 game servers. Built with [kubebuilder](https://book.kubebuilder.io/), this operator provides a declarative way to deploy and manage Quake 3 servers in Kubernetes clusters.

## Features

- **Declarative Server Management**: Define Quake servers as Kubernetes resources
- **Reusable Templates**: Create `QuakeServerTemplate` resources for shared configurations
- **Gateway API Integration**: Expose servers via HTTPRoute (browser) and UDPRoute (native clients)
- **Validation Webhooks**: Automatic validation with clear error messages
- **Secure Defaults**: Non-root containers, dropped capabilities, read-only filesystem
- **High Availability**: Leader election support for multi-replica deployments
- **Pause/Resume**: Control server state via annotation

## Installation

### Prerequisites

- Kubernetes v1.25+
- [cert-manager](https://cert-manager.io/) (for webhook TLS)
- [Gateway API CRDs](https://gateway-api.sigs.k8s.io/) (optional, for Gateway features)

### Using Helm

```bash
# Install with default values
helm install quake-operator ../helm/quake-kube-operator \
  -n quake-system --create-namespace

# Install with custom values
helm install quake-operator ../helm/quake-kube-operator \
  -n quake-system --create-namespace \
  --set replicaCount=2 \
  --set image.tag=v0.1.0

# Disable webhooks (for development)
helm install quake-operator ../helm/quake-kube-operator \
  -n quake-system --create-namespace \
  --set webhook.enabled=false
```

### Using Make (Development)

```bash
# Install CRDs
make install

# Run locally (outside cluster)
make run

# Deploy to cluster
make deploy IMG=<your-registry>/quake-operator:tag
```

## Custom Resource Definitions

### QuakeServer

The primary resource for deploying a Quake 3 server.

```yaml
apiVersion: quakekube.io/v1alpha1
kind: QuakeServer
metadata:
  name: my-server
spec:
  # Required: Accept the Quake 3 EULA
  agreeEula: true

  # Optional: Reference a template for base configuration
  templateRef:
    name: ffa-standard

  # Server configuration (overrides template values)
  serverConfig:
    fragLimit: 25
    timeLimit: "15m"

    game:
      type: FreeForAll          # FreeForAll, Tournament, TeamDeathmatch, CaptureTheFlag
      motd: "Welcome!"
      forceRespawn: false
      inactivity: "10m"
      quadFactor: 3

    server:
      hostname: "My Quake Server"
      maxClients: 12
      rconPassword: "secret"

    bot:
      minPlayers: 4             # Bots fill remaining slots
      skill: 3                  # 1-5 skill level

    maps:
      - name: q3dm7
      - name: q3dm17
      - name: q3tourney2

    commands:                   # Extra server commands
      - "set sv_allowDownload 1"

  # Gateway API configuration
  gateway:
    enabled: true
    gatewayRef:
      name: main-gateway
      namespace: gateway-system
    hostname: "quake.example.com"

  # Resource limits
  resources:
    limits:
      cpu: "1"
      memory: "512Mi"
    requests:
      cpu: "100m"
      memory: "128Mi"
```

### QuakeServerTemplate

Reusable configurations that can be referenced by multiple QuakeServer resources.

```yaml
apiVersion: quakekube.io/v1alpha1
kind: QuakeServerTemplate
metadata:
  name: ffa-standard
spec:
  serverConfig:
    fragLimit: 25
    timeLimit: "15m"
    game:
      type: FreeForAll
      motd: "Standard FFA Server"
    server:
      maxClients: 16
    bot:
      minPlayers: 4
      skill: 3
    maps:
      - name: q3dm7
      - name: q3dm17
```

Templates support all the same `serverConfig` fields as QuakeServer. When a QuakeServer references a template, inline values override template values (strategic merge).

## Operations

### Viewing Servers

```bash
# List all servers
kubectl get quakeservers

# Get detailed status
kubectl describe quakeserver my-server

# View server events
kubectl get events --field-selector involvedObject.kind=QuakeServer
```

### Pause/Resume

Pause a server (scales deployment to 0 replicas):

```bash
# Pause
kubectl annotate quakeserver my-server quakekube.io/paused=true

# Resume
kubectl annotate quakeserver my-server quakekube.io/paused-
```

### Template Management

```bash
# List templates
kubectl get quakeservertemplates

# View which servers use a template
kubectl describe quakeservertemplate ffa-standard
# Status.usedBy shows dependent servers
```

## Game Types

| Type | Description |
|------|-------------|
| `FreeForAll` | Every player for themselves |
| `Tournament` | 1v1 duel mode |
| `TeamDeathmatch` | Team-based deathmatch |
| `CaptureTheFlag` | Capture the enemy flag |
| `SinglePlayer` | Single player mode |

## Architecture

The operator creates the following resources for each QuakeServer:

- **Deployment**: Runs the game server with a content-server sidecar
- **ConfigMap**: Server configuration (`config.yaml`)
- **Service**: ClusterIP with ports 8080 (HTTP), 27960 (UDP), 9090 (content)
- **HTTPRoute** (optional): Browser WebSocket access via Gateway API
- **UDPRoute** (optional): Native client access via Gateway API

### Security

By default, containers run with:

- `runAsNonRoot: true`
- `runAsUser: 1000`
- `allowPrivilegeEscalation: false`
- `readOnlyRootFilesystem: true`
- `capabilities.drop: ["ALL"]`

These can be customized via `spec.securityContext` and `spec.podSecurityContext`.

## Development

### Running Tests

```bash
# Unit tests
make test

# With coverage
go test -v -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

### Generating Manifests

```bash
# Regenerate CRDs after modifying types
make manifests

# Regenerate deepcopy functions
make generate
```

### Linting

```bash
golangci-lint run
```

## Uninstallation

```bash
# Using Helm
helm uninstall quake-operator -n quake-system

# Using Make
make undeploy
make uninstall  # Removes CRDs
```

## License

Copyright 2026. Licensed under the Apache License, Version 2.0.
