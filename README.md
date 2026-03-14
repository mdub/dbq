# dbq

A CLI for executing serverless SQL queries on Databricks.

```bash
dbq sql "
  SELECT stuff FROM the.lakehouse WHERE patience = 0
"
```

## Features

- Quick.
- Simple.
- JSON, CSV, or Parquet output.
- Preserves type information; no "everything is a string" nonsense.

## Installing it

On Mac, with `brew`:

```
brew install mdub/brews/dbq 
```

Elsewhere, with `go install`:

```
go install github.com/mdub/dbq@latest
```

## Using it

Select a workspace:

```bash
export DBQ_WORKSPACE=my-workspace
```

Run a query:

```bash
dbq sql "SELECT current_timestamp(), current_user()"
```

View a [cheatsheet](cmd/cheatsheet.md):

```bash
dbq cheatsheet
```

## How it works

`dbq` uses the [Databricks SDK for Go](https://github.com/databricks/databricks-sdk-go):

- **Authentication**: OAuth U2M (user-to-machine) flow via the SDK's `credentials/u2m` package. Tokens are cached in `~/.databricks/token-cache.json` and automatically refreshed.
- **Query execution**: Statement Execution API via the SDK's `service/sql` package.

No configuration files required - just environment variables or command-line flags.
