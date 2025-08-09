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
- Docker (for container operations)
- kubectl (for Kubernetes operations)
- ORAS CLI (for OCI artifact operations)

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

Download the latest release from the [releases page](https://github.com/dynamoai/dynactl/releases) and extract the binary to your PATH.

## Global Options

These options can be used with any dynactl command:

- `--verbose, -v`: Increase output verbosity (can be used multiple times)
- `--help, -h`: Display help information for the command

## Commands

### `dynactl artifacts`

Process artifacts for deployment and upgrade operations.

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

### `dynactl cluster`

Handle cluster status and validation.

#### `dynactl cluster check --namespace <namespace>`

Performs comprehensive cluster validation including:

- **Kubernetes Version**: Checks compatibility with required version
- **Resource Availability**: Validates CPU and memory requirements (32+ vCPU, 128+ GB memory)
- **Namespace RBAC**: Verifies permissions for deployments, PVCs, services, configmaps, and secrets
- **Cluster RBAC**: Checks cluster-level permissions for CRD creation
- **Storage Capacity**: Assesses available storage and usage

**Example:**
```bash
$ dynactl cluster check --namespace my-namespace
Checking cluster status for namespace: my-namespace

✓ Kubernetes version: 1.24.6 (compatible)
✓ Available resources: 24/24 CPU cores, 96/96 GB memory (allocatable/total)
✓ RBAC permissions: all required permissions available
✓ Cluster RBAC permissions: all required cluster permissions available
✓ Storage capacity: adequate storage capacity (45.2% used)

✓ Cluster check completed successfully
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