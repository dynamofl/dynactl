# dynactl
A Go-based tool to manage customer's DevOps operations on Dynamo AI deployment and maintenance.

# Installation

## From Source

```bash
# Clone the repository
git clone https://github.com/dynamoai/dynactl.git
cd dynactl

# Build the binary
go build -o dynactl ./cmd/dynactl

# Install the binary (optional)
sudo mv dynactl /usr/local/bin/
```

## From Binary

Download the latest release from the [releases page](https://github.com/dynamoai/dynactl/releases) and extract the binary to your PATH.

# Global Options

These options can be used with any dynactl command:

- `--verbose, -v`: Increase output verbosity (can be used multiple times)
- `--help, -h`: Display help information for the command

# `dynactl config` (Future Work)

Configuration management functionality is planned for future releases. This will include:
- Getting and setting configuration values
- Managing registry credentials
- Cloud provider configuration
- Cluster context management

# `dynactl artifacts`

This command processes the artifacts for the deployment and upgrade.

## `dynactl artifacts pull --file <filename>`

- Reads a manifest JSON file from the local filesystem.
- Parses artifact list from `images`, `models`, and `charts` arrays.
- For each artifact: Pulls using appropriate tool (`docker pull` for container images, `helm pull` for Helm charts, `oras pull` for ML models) based on type. Saves to `--output-dir`. Handles authentication via Docker config/environment variables.

**Example:**
```
$ dynactl artifacts pull --file examples/example.manifest.json --output-dir ./artifacts
Successfully pulled 3 artifacts to ./artifacts
```

**Manifest File Format:**
```json
{
  "customer_id": "abc123",
  "customer_name": "paypal",
  "release_version": "3.22.4",
  "onboarding_date": "2024-07-10",
  "license_generated_at": "2024-07-11",
  "license_expiry": "2025-12-31T00:00:00Z",
  "max_users": 25,
  "spoc": {
    "name": "John Doe",
    "email": "john.doe@example.com"
  },
  "images": [
    {
      "name": "dynamoai-operator",
      "tag": "latest",
      "path": "oci://artifacts.dynamo.ai/paypal/dynamoai-operator"
    }
  ],
  "models": [
    {
      "name": "base-model",
      "tag": "latest",
      "path": "artifacts.dynamo.ai/paypal/sentence-transformers/all-minilm-l6-v2"
    }
  ],
  "charts": [
    {
      "name": "dynamoai-base",
      "version": "1.0.0",
      "appVersion": "3.21.2",
      "path": "oci://artifacts.dynamo.ai/intact-helm-charts/intact--dynamoai-base"
    }
  ]
}
```

## Future Work

The following artifacts commands are planned for future releases:

### `dynactl artifacts mirror --manifest-uri <oci_uri> --target-registry <registry_url>`

- Fetches manifest.
- Pulls each artifact from Harbor.
- Re-tags and pushes each artifact to `-target-registry`. Handles auth for both source and target. Respects proxies.

### `dynactl artifacts export --manifest-uri <oci_uri> --archive-file <path.tar.gz>`

- Fetches manifest.
- Pulls all artifacts to a temporary local cache.
- Packages the manifest and all artifacts into a single compressed tarball.

### `dynactl artifacts import --archive-file <path.tar.gz> --target-registry <registry_url>`

- Extracts the archive.
- Reads the manifest.
- Pushes all artifacts from the local cache to the `-target-registry`. Handles auth for target.

# `dynactl cluster`

This command handles the cluster status.

## `dynactl cluster check --namespace <namespace>`

- If <namespace> doesn't exist, dynactl will create it.

- Checks the cluster status for the deployment, including:
  - Kubernetes version compatibility
  - Available vCPU, memory resources (more than 32 vCPU, 128 GB memory)
  - Required RBAC permissions in the namespace
    - Create deployment
    - Create PVC
    - Create service
    - Create configmap
    - Create secret
  - Cluster RBAC permissions
    - Create CRD

**Example:**
```
$ dynactl cluster check --namespace my-namespace
✓ Kubernetes version: 1.24.6 (compatible)
✓ Available resources: 24/24 CPU cores, 96/96 GB memory (allocatable/total)
✓ RBAC permissions: all required permissions available
✓ Cluster RBAC permissions: all required cluster permissions available
✓ Storage capacity: adequate storage capacity (45.2% used)
```

# `dynactl validate` (Future Work)

Service validation functionality is planned for future releases. This will include:
- API endpoint connectivity and response time checks
- Core service health checks
- Authentication and authorization functionality validation
- Data processing pipeline verification
- External dependency integration validation
- Basic functionality smoke tests

# Development

## Prerequisites

- Go 1.21 or higher
- Docker (for container operations)
- kubectl (for Kubernetes operations)

## Building

```bash
# Build the binary
go build -o dynactl ./cmd/dynactl

# Run tests
go test ./...

# Run linter
go vet ./...
```

## Project Structure

```
dynactl/
├── cmd/
│   └── dynactl/
│       ├── main.go       # Main entry point
│       └── main_test.go  # Tests for main command
├── pkg/
│   ├── commands/         # Command implementations
│   │   ├── artifacts.go  # Artifacts command
│   │   └── cluster.go    # Cluster command
│   └── utils/            # Utility functions
│       ├── artifacts.go  # Artifact processing utilities
│       ├── kubernetes.go # Kubernetes utilities
│       ├── logging.go    # Logging utilities
│       └── logging_test.go # Tests for logging
├── examples/             # Example manifest files
├── go.mod
├── go.sum
└── README.md
```