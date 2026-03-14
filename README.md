# dbq

A simple CLI for executing SQL queries against Databricks.

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
