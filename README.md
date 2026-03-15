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

- **Configuration**: No files required - just command-line flags and a couple of environment variables.
- **Authentication**: OAuth U2M (user-to-machine) flow. Tokens are cached and automatically refreshed.
- **Results**: fetched in Arrow format, minimizing serialization overhead and preserving type fidelity.
