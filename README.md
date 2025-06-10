# dynactl
A Python based tool to manage customer's DevOps operations on Dynamo AI deployment and maintenance.

# Installation

```
pip install -e .
```

# Global Options

These options can be used with any dynactl command:

- `--verbose, -v`: Increase output verbosity (can be used multiple times)
- `--config-file <path>`: Specify a custom config file location (default: ~/.dynactl/config)
- `--help, -h`: Display help information for the command

# `dynactl config`

This command gets and sets the configuration for dynactl. It's saved into the .dynactl/config file.

## `dynactl config get <key>`

- Fetches the value of the `key` in configuration.
- Returns `Invalid key` error message if the key is not supported.
- Returns `The value is unset` error message upon error.

**Example:**
```
$ dynactl config get cloud
$ [cloud]: aws
```

## `dynactl config set <key> <value>`

- Sets the value of the `key` to `value` in configuration.
- Returns `Invalid key` error message if the key is not supported.
- Returns `Invalid value` error message if the value is not supported.

**Example:**
```
$ dynactl config set cloud aws
$ Updated property [cloud]: aws
```

# `dynactl artifacts`

This command processes the artifacts for the deployment and upgrade.

## `dynactl artifacts pull --manifest-uri <oci_uri> --output-dir <path>`

- Fetches manifest JSON from `<oci_uri>`.
- Parses artifact list.
- For each artifact: Pulls using appropriate tool (`docker pull`, `helm pull oci://...`, `oras pull`) based on URI/type. Saves to `-output-dir`. Handles authentication via Docker config/environment variables. Respects `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`.

**Example:**
```
$ dynactl artifacts pull --manifest-uri registry.example.com/manifests/dynamo:v1.0.0 --output-dir ./artifacts
Successfully pulled 15 artifacts to ./artifacts
```

## `dynactl artifacts mirror --manifest-uri <oci_uri> --target-registry <registry_url>`

- Fetches manifest.
- Pulls each artifact from Harbor.
- Re-tags and pushes each artifact to `-target-registry`. Handles auth for both source and target. Respects proxies.

**Example:**
```
$ dynactl artifacts mirror --manifest-uri registry.example.com/manifests/dynamo:v1.0.0 --target-registry internal-registry.company.com
Successfully mirrored 15 artifacts to internal-registry.company.com
```

## `dynactl artifacts export --manifest-uri <oci_uri> --archive-file <path.tar.gz>`

- Fetches manifest.
- Pulls all artifacts to a temporary local cache.
- Packages the manifest and all artifacts into a single compressed tarball.

**Example:**
```
$ dynactl artifacts export --manifest-uri registry.example.com/manifests/dynamo:v1.0.0 --archive-file dynamo-artifacts.tar.gz
Successfully exported 15 artifacts to dynamo-artifacts.tar.gz (2.3GB)
```

## `dynactl artifacts import --archive-file <path.tar.gz> --target-registry <registry_url>`

- Extracts the archive.
- Reads the manifest.
- Pushes all artifacts from the local cache to the `-target-registry`. Handles auth for target.

**Example:**
```
$ dynactl artifacts import --archive-file dynamo-artifacts.tar.gz --target-registry internal-registry.company.com
Successfully imported 15 artifacts to internal-registry.company.com
```

# `dynactl cluster`

This command handles the cluster status.

## `dynactl cluster check`

- Checks the cluster status for the deployment, including:
  - Available CPU, memory, and storage resources
  - Required RBAC permissions
  - Network connectivity and DNS resolution
  - Kubernetes version compatibility
  - Required CRDs and admission controllers

**Example:**
```
$ dynactl cluster check
✓ Kubernetes version: 1.24.6 (compatible)
✓ Available resources: sufficient (8/16 CPU cores, 24/32GB memory)
✓ RBAC permissions: all required permissions available
✓ Network connectivity: all tests passed
! Warning: Limited storage capacity (80% used)
```

# `dynactl validate`

Validates that the deployed Dynamo AI service functions correctly by performing the following checks:
- API endpoint connectivity and response time
- Core service health checks
- Authentication and authorization functionality
- Data processing pipeline verification
- External dependency integration validation
- Basic functionality smoke tests

**Example:**
```
$ dynactl validate
Performing validation of Dynamo AI deployment...
✓ API endpoints: all accessible (avg response: 126ms)
✓ Core services: all healthy
✓ Auth subsystem: working correctly
✓ Data processing: validated
✓ External integrations: connected
✓ Smoke tests: passed
Validation successful: Dynamo AI is functioning correctly
```