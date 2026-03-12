# dbq

A simple CLI for executing SQL queries against Databricks.

## Installation

```
go install github.com/mdub/dbq@latest
```

Or build from source:

```
go build -o dbq .
```

## Usage

### Authentication

First, authenticate to your Databricks workspace:

```
dbq --host block-lakehouse-staging login
```

This opens a browser for OAuth login. Tokens are cached and automatically refreshed.

If your credentials expire during interactive use, `dbq` will automatically re-authenticate (opening a browser). This auto-login is enabled by default when running in a terminal, and disabled when output is piped or in scripts. Override with `--auto-login` or `--no-auto-login`.

### Running queries

```
dbq --host block-lakehouse-staging --warehouse ventana-warehouse sql "SELECT 1"
```

Or use environment variables:

```
export DATABRICKS_HOST=block-lakehouse-staging
export DBQ_WAREHOUSE=ventana-warehouse

dbq sql "SELECT current_date()"
```

Read SQL from a file:

```
dbq sql @query.sql
```

Read SQL from stdin:

```
echo "SELECT 1" | dbq sql
```

### Output formats

```
dbq sql -f json "SELECT 1"   # JSON array (default)
dbq sql -f csv "SELECT 1"    # CSV
dbq sql -f raw "SELECT 1"    # Full response with metadata
```

### List warehouses

```
dbq warehouses
```

## Configuration

| Flag | Environment | Description |
|------|-------------|-------------|
| `--host`, `-H` | `DATABRICKS_HOST` | Databricks workspace (required) |
| `--warehouse`, `-w` | `DBQ_WAREHOUSE` | SQL warehouse ID or name (default: "Serverless Starter Warehouse") |
| `--auto-login` | | Force auto re-authentication on auth failure |
| `--no-auto-login` | | Disable auto re-authentication |
| `--debug` | | Enable debug output |

Host can be specified as:
- Simple name: `block-lakehouse-staging` (expands to `block-lakehouse-staging.cloud.databricks.com`)
- Full hostname: `block-lakehouse-staging.cloud.databricks.com`
- Full URL: `https://block-lakehouse-staging.cloud.databricks.com`

Warehouse can be specified by ID or name. Names are resolved via the API.

## How it works

`dbq` uses the [Databricks SDK for Go](https://github.com/databricks/databricks-sdk-go):

- **Authentication**: OAuth U2M (user-to-machine) flow via the SDK's `credentials/u2m` package. Tokens are cached in `~/.databricks/token-cache.json` and automatically refreshed.
- **Query execution**: Statement Execution API via the SDK's `service/sql` package.

No configuration files required - just environment variables or command-line flags.
