# dynactl

A Go-based tool to manage customer's DevOps operations on Dynamo AI deployment and maintenance.

## Features

- **Artifact Management**: Pull container images, ML models, and Helm charts from OCI registries
- **Cluster Validation**: Comprehensive Kubernetes cluster health and permission checks
- **Enhanced Progress Reporting**: Detailed progress information with file sizes and timing
- **Structured Codebase**: Modular, maintainable code with clear separation of concerns
- **Cross-Platform**: Works on macOS, Linux, and Windows

## Installation

### Prerequisites

- Go 1.21 or higher
- Docker (for container operations and as the default credential store)
- kubectl (for Kubernetes operations)
- (Optional) ORAS CLI if you prefer managing registry logins with `oras login`

### From Source

```bash
# Clone the repository
git clone https://github.com/dynamoai/dynactl.git
cd dynactl

# Build the binary
make build

# Install the binary (optional)
sudo mv bin/dynactl /usr/local/bin/
```

### From Binary

Download the latest release from the [releases page](https://github.com/dynamofl/dynactl/releases) and extract the binary to your PATH.

## Global Options

These options can be used with any dynactl command:

- `--verbose, -v`: Increase output verbosity (can be used multiple times)
- `--help, -h`: Display help information for the command

## Commands

### `dynactl artifacts`

Process artifacts for deployment and upgrade operations.

#### Registry Authentication

`dynactl` uses the embedded ORAS SDK and automatically reads credentials from:

- Your Docker/Podman credential store (e.g., `docker login artifacts.dynamo.ai`)
- Any existing ORAS CLI login
- Credentials saved via `dynactl registry login`

To store credentials directly through the CLI:
```bash
echo "super-secret-password" | dynactl registry login artifacts.dynamo.ai -u robot$jenkins-ci --password-stdin
```

You can also provide identity or access tokens with `--identity-token` or `--access-token`.

#### `dynactl artifacts pull --file <filename>`

Pulls artifacts from a local manifest JSON file.

**Example:**
```bash
$ dynactl artifacts pull --file testdata/sample.manifest.json --output-dir ./artifacts
=== Loading Manifest from File ===
Manifest file: testdata/sample.manifest.json
Output directory: ./artifacts

=== Loading Manifest and Pulling Artifacts ===
Manifest loaded successfully:
  Customer: Test Customer (test-customer-123)
  Release Version: 3.22.2
  Onboarding Date: 2024-01-15
  License Expiry: 2025-01-15T10:00:00Z
  Max Users: 100

Artifacts found in manifest:
  Container Images: 2
  ML Models: 2
  Helm Charts: 2

=== Starting Artifact Pull Process ===
Total artifacts to pull: 6
Output directory: ./artifacts
Components breakdown:
  - Container Images: 2
  - ML Models: 2
  - Helm Charts: 2

------------------------------------------------------------
Pulling artifact 1/6: dynamoai-api (containerImage)
------------------------------------------------------------
📦 Pulling container image...
  Reference: artifacts.dynamo.ai/dynamoai/3.22.2/images/dynamoai-api:latest
  Downloading image layers...
  Saving image to: ./artifacts/dynamoai-api.tar
  Image saved: 245.67 MB
✅ Successfully pulled dynamoai-api in 45.2s

=== Pull Summary ===
Total time: 2m15s
Successful: 6
Failed: 0

🎉 Successfully completed all operations!
Total artifacts pulled: 6
All files saved to: ./artifacts
```

#### `dynactl artifacts pull --url <oci_uri>`

Pulls a manifest file from an OCI registry and then pulls all artifacts listed in the manifest.

You can limit which artifact categories are downloaded by providing any combination of:

- `--images` – container images only
- `--models` – ML model archives only
- `--charts` – Helm charts only

If none of these flags are supplied all artifact types are pulled (backwards compatible).

**Example:**
```bash
$ dynactl artifacts pull --url artifacts.dynamo.ai/dynamoai/manifest:3.22.2
=== Pulling Manifest from URL ===
URL: artifacts.dynamo.ai/dynamoai/manifest:3.22.2
Output directory: ./artifacts

✅ Successfully pulled manifest from artifacts.dynamo.ai/dynamoai/manifest:3.22.2 to ./artifacts

=== Loading Manifest and Pulling Artifacts ===
Manifest loaded successfully:
  Customer: dynamoai (1f4a8e7e-6c5d-4636-91f0-bf9e72de92c2)
  Release Version: 3.22.2
  Onboarding Date: 2025-06-24T08:13:22.673045+00:00

Artifacts found in manifest:
  Container Images: 17
  ML Models: 1
  Helm Charts: 3

[Progress continues with detailed artifact pulling...]
```

#### `dynactl artifacts mirror`

Pulls artifacts into a local cache and then pushes selected types to a target registry.

- Requires either `--url` or `--file` to locate the manifest.
- Requires `--target-registry` to define where artifacts are pushed.
- Honors the same `--images`, `--models`, and `--charts` filters as `pull`. By default only container images are mirrored, and at present models/charts are not pushed.
- Use `--cache-dir` to reuse an existing workspace or `--keep-cache` to retain the temporary cache that dynactl creates.

**Example:**
```bash
$ dynactl artifacts mirror \
    --url artifacts.dynamo.ai/customer/3.23.2/manifests:3.23.2 \
    --target-registry customer.registry.example.com \
    --images
```

**Manifest File Format:**
```json
{
  "customer_id": "test-customer-123",
  "customer_name": "Test Customer",
  "release_version": "3.22.2",
  "onboarding_date": "2024-01-15",
  "license_generated_at": "2024-01-15T10:00:00Z",
  "license_expiry": "2025-01-15T10:00:00Z",
  "max_users": 100,
  "spoc": {
    "name": "John Doe",
    "email": "john.doe@testcustomer.com"
  },
  "artifacts": {
    "charts_root": "oci://artifacts.dynamo.ai/dynamoai/3.22.2/charts",
    "images_root": "oci://artifacts.dynamo.ai/dynamoai/3.22.2/images",
    "models_root": "oci://artifacts.dynamo.ai/dynamoai/3.22.2/models"
  },
  "images": [
    "oci://artifacts.dynamo.ai/dynamoai/3.22.2/images/dynamoai-api:latest",
    "oci://artifacts.dynamo.ai/dynamoai/3.22.2/images/dynamoai-web:latest"
  ],
  "models": [
    "oci://artifacts.dynamo.ai/dynamoai/3.22.2/models/text-generation-model:latest",
    "oci://artifacts.dynamo.ai/dynamoai/3.22.2/models/image-classification-model:latest"
  ],
  "charts": [
    {
      "name": "dynamoai-base",
      "version": "1.1.2",
      "appVersion": "3.22.2",
      "filename": "dynamoai-base-1.1.2.tgz",
      "harbor_path": "oci://artifacts.dynamo.ai/dynamoai/3.22.2/charts/dynamoai-base-1.1.2.tgz",
      "sha256": "abc123def456",
      "size_bytes": 1048576
    }
  ]
}
```

### `dynactl registry login`

Manage credentials used when pulling artifacts from private registries.

```bash
$ dynactl registry login artifacts.dynamo.ai -u robot$jenkins-ci --password-stdin
```

- Credentials are written to `~/.dynactl/credentials.json` with `0600` permissions.
- `--password`, `--password-stdin`, `--identity-token`, and `--access-token` are supported.
- Stored credentials are used alongside Docker/ORAS credentials when pulling manifests, container images, ML models, and Helm charts.

### `dynactl cluster`

Handle cluster status and validation.

#### `dynactl cluster all check --namespace <namespace>`

Runs all available cluster checks:

- **Kubernetes Version**: Checks compatibility with required version
- **Node Resources**: Aggregated CPU and memory across ready nodes
- **Namespace Permissions**: Uses authorization API (SelfSubjectAccessReview) to validate create permissions for deployments, PVCs, services, configmaps, secrets
- **Cluster Permissions**: Uses authorization API to validate permission to create CRDs
- **StorageClasses**: Checks for common database-compatible provisioners
- **Storage Capacity**: Assesses available storage and usage

**Example:**
```bash
$ dynactl cluster all check --namespace my-namespace
```

#### `dynactl cluster node check`

Checks node readiness and aggregated CPU/memory resources. No namespace required.

**Features:**
- **Node Status**: Reports ready/not-ready nodes
- **Resource Capacity**: Shows allocatable vs total CPU and memory for each node
- **Resource Usage**: Displays percentage of CPU, memory, and GPU requests/limits for each node
- **Instance Types**: Lists AWS instance types for each node

**Example:**
```bash
$ dynactl cluster node check
```

### Release Automation

Releases are generated automatically when changes land on `main`:

- Update the `version` constant in `cmd/dynactl/main.go` as part of your PR.
- After the PR is merged, the `Release` GitHub Actions workflow builds binaries for Linux, macOS, and Windows and publishes a `v<version>` GitHub release (creating the tag if needed).
- No manual packaging or `gh release` commands are required.

**Default Output:**
```bash
$ dynactl cluster node check
Checking node resources...
Name | Type | CPU Alloc/Total | Mem Alloc/Total | CPU %Req | CPU %Limits | Mem %Req | Mem %Limits | GPU Alloc/Total
-----|------|----------------|-------------------|-----------|-------------|-----------|-------------|----------------
ip-192-168-6-2.ec2.internal | c5a.xlarge | 3/4 | 6/7 GB | 0.8% | 0.0% | 1.8% | 11.2% | -
ip-192-168-58-120.ec2.internal | c5a.xlarge | 3/4 | 6/7 GB | 5.9% | 12.8% | 43.6% | 96.9% | -
ip-192-168-252-75.ec2.internal | g5.2xlarge | 7/8 | 30/30 GB | 50.9% | 50.6% | 80.3% | 82.4% | 8/10
ip-192-168-61-169.ec2.internal | m5.large | 1/2 | 6/7 GB | 94.8% | 191.7% | 27.4% | 62.7% | -
ip-192-168-40-124.ec2.internal | t3a.xlarge | 3/4 | 14/15 GB | 54.3% | 107.1% | 30.1% | 63.8% | -
```

*Note: Output is sorted alphabetically by instance type for easy comparison across node types.*

**Verbose Output** (with `-v 2`):
```bash
$ dynactl cluster node check -v 2
DEBUG: Starting dynactl with verbosity level 2
Checking node resources...
INFO: Checking resources on 24 nodes...
Name | Type | CPU Alloc/Total | Mem Alloc/Total | CPU %Req | CPU %Limits | Mem %Req | Mem %Limits | GPU Alloc/Total
-----|------|----------------|-------------------|-----------|-------------|-----------|-------------|----------------
ip-192-168-252-75.ec2.internal | g5.2xlarge | 7/8 | 30/30 GB | 50.9% | 50.6% | 80.3% | 82.4% | 8/10
```

#### `dynactl cluster permission check --namespace <namespace>`

Checks permissions in a namespace and at cluster level using the authorization API.

**Example:**
```bash
$ dynactl cluster permission check --namespace my-namespace
```

#### `dynactl cluster storage check`

Checks StorageClasses for database compatibility and storage capacity.

**Example:**
```bash
$ dynactl cluster storage check
```

### `dynactl guard models list -n <namespace> [--output json]`

List deployments in a namespace with per-container resource requests and limits for CPU, memory, and GPUs (`nvidia.com/gpu`).

**Example:**
```bash
$ dynactl guard models list -n my-namespace
Namespace: my-namespace
Deployment / Container                          Requests (cpu/mem/gpu)         Limits (cpu/mem/gpu)
----------------------------------------------------------------------------------------------
guard-api guard-container                        250m/256Mi/-                   500m/512Mi/-
guard-worker worker                              500m/1Gi/1                     1/2Gi/1
```

JSON output:
```bash
$ dynactl guard models list -n my-namespace --output json
[
  {
    "Name": "guard-api",
    "Containers": [
      {
        "Name": "guard-container",
        "RequestsCPU": "250m",
        "RequestsMemory": "256Mi",
        "RequestsGPU": "0",
        "LimitsCPU": "500m",
        "LimitsMemory": "512Mi",
        "LimitsGPU": "0"
      }
    ]
  }
]
```

## Future Work

The following features are planned for future releases:

### Configuration Management (`dynactl config`)
- Getting and setting configuration values
- Managing registry credentials
- Cloud provider configuration
- Cluster context management

### Advanced Artifact Operations
- **`dynactl artifacts mirror`**: Mirror artifacts between registries
- **`dynactl artifacts export`**: Export artifacts to compressed archives
- **`dynactl artifacts import`**: Import artifacts from archives to registries

### Service Validation (`dynactl validate`)
- API endpoint connectivity and response time checks
- Core service health checks
- Authentication and authorization functionality validation
- Data processing pipeline verification
- External dependency integration validation
- Basic functionality smoke tests

## Development

### Building

```bash
# Build the binary
make build

# Run tests
make test

# Run linter
make lint

# Clean build artifacts
make clean
```

### Project Structure

```
dynactl/
├── cmd/
│   └── dynactl/
│       ├── main.go           # Main entry point
│       └── main_test.go      # Tests for main command
├── pkg/
│   ├── commands/             # Command implementations
│   │   ├── artifacts.go      # Artifacts command logic
│   │   ├── artifacts_test.go # Artifacts command tests
│   │   └── cluster.go        # Cluster command logic
│   └── utils/                # Utility functions
│       ├── artifacts.go      # Manifest and component logic
│       ├── artifact_pullers.go # Artifact pulling operations
│       ├── kubernetes.go     # Kubernetes utilities
│       ├── logging.go        # Logging utilities
│       └── logging_test.go   # Tests for logging
├── testdata/                 # Test manifest files
├── examples/                 # Example files
├── bin/                      # Built binaries
├── Makefile                  # Build automation
├── go.mod                    # Go module definition
├── go.sum                    # Go module checksums
└── README.md                 # This file
```

### Key Features

- **Modular Architecture**: Clear separation between commands, utilities, and business logic
- **Enhanced Progress Reporting**: Detailed progress information with timing and file sizes
- **Comprehensive Error Handling**: Graceful error handling with detailed error messages
- **Cross-Platform Support**: Works on macOS, Linux, and Windows
- **Authentication Support**: Uses Docker credentials for registry authentication
- **Test Coverage**: Comprehensive test suite for all major functionality

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite: `make test`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
