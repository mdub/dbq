# ADR 0001: No configuration file

## Status

Accepted

## Context

Most CLI tools eventually grow a configuration file (`~/.toolrc`, `config.yaml`,
etc.) for persisting user preferences. We need to decide whether `dbq` should
support one.

## Options considered

- **Config file** (e.g. `~/.dbq.yaml`): convenient for users who always target
  the same workspace, but adds complexity — file discovery, parsing, merging
  with flags, and debugging "why is it doing that?" when the config is stale or
  forgotten.
- **Environment variables only**: simple and transparent. The user's shell
  profile or a wrapper script can set defaults. Easy to inspect (`env | grep
  DBQ`) and override per-invocation.
- **Flags only**: maximally explicit, but tedious for repeated use.

## Decision

`dbq` does not support a configuration file. Behaviour is driven by CLI flags
and parameters, with environment variables as the only implicit configuration.

## Consequences

- Behaviour is predictable — there's no hidden config file that might vary
  between machines or get out of sync.
- Users who want persistent defaults can set environment variables in their
  shell profile, or write a shell alias.
- Debugging is straightforward: flags and environment variables are all there
  is to check.
- If a user needs different settings for different workspaces, they use
  explicit flags or per-directory environment setup (e.g. `direnv`), rather
  than a config file with profiles.
