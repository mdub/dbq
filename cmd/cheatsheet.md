# dbq cheatsheet

`dbq` is a CLI for executing SQL queries against Databricks.

## Selecting a workspace

The only required config is the Databricks workspace, which can be specified via
`--workspace` flag, or `DBQ_WORKSPACE` environment variable.

For example:

    dbq --workspace my-workspace sql "SELECT current_date()"

or:

    export DBQ_WORKSPACE=my-workspace
    dbq sql "SELECT current_date()"

The workspace can be specified in several ways. The following are all equivalent:

    my-workspace                              # expanded to my-workspace.cloud.databricks.com
    my-workspace.cloud.databricks.com         # used as-is
    https://my-workspace.cloud.databricks.com # used as-is

## Authentication

`dbq` uses OAuth (user-to-machine) for authentication. On first use, it
opens a browser for you to log in. Tokens are cached locally and refreshed automatically.

If your token expires during interactive use, `dbq` will re-authenticate automatically.
Use `--no-auto-login` to disable this (e.g. in scripts that should fail fast).

To authenticate explicitly:

    dbq auth login

## Running queries

    dbq sql "SELECT current_date()"
    dbq sql "SELECT * FROM catalog.schema.table LIMIT 10"

Read SQL from a file:

    dbq sql @query.sql

Read SQL from stdin:

    echo "SELECT 1" | dbq sql

## Output formats

    dbq sql -f jsonl "..."     # JSONL, one object per line (default)
    dbq sql -f json "..."      # JSON array
    dbq sql -f csv "..."       # CSV with header row
    dbq sql -f parquet "..."   # Apache Parquet
    dbq sql -f arrows "..."    # Arrow IPC streaming format
    dbq sql -f arrow "..."     # Arrow IPC file format

## Writing to a file

Use `--output` / `-o` to write results directly to a file.
The format is automatically derived from the file extension, e.g.

    dbq sql -o results.parquet "SELECT * FROM t"

## Piping and post-processing

JSONL output works well with `jq`:

    dbq sql "SELECT name, age FROM people" | jq .name

CSV output works well with `mlr` (Miller) and standard tools:

    dbq sql -f csv "SELECT * FROM t" | mlr --csv sort-by name

## Async queries

Long-running queries can be run asynchronously:

    # start a query
    QUERY_ID=$(dbq sql --async "SELECT * FROM big_table")

    # wait for completion
    dbq query wait $QUERY_ID

    # fetch results
    dbq query results -o results.parquet $QUERY_ID

## Selecting a warehouse

Queries run on a SQL warehouse. By default, `dbq` uses "Serverless Starter
Warehouse". To use a different one, specify it by name or ID:

    dbq --warehouse my-warehouse sql "SELECT 1"

    # or via environment variable:
    export DBQ_WAREHOUSE=my-warehouse

To see available warehouses:

    dbq warehouses
