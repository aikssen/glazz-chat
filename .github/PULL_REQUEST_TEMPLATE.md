# Summary

<!-- What changes and why. Link the issue this resolves. -->

Closes #

## Type of change

- [ ] Bug fix
- [ ] Feature
- [ ] Documentation
- [ ] Refactor with no behavior change
- [ ] Test or tooling
- [ ] Dependency update

## Checks run

<!-- Confirm what you actually ran, not what should pass. -->

- [ ] `pnpm check`
- [ ] `pnpm test:integration`
- [ ] `pnpm e2e`
- [ ] `pnpm e2e:visual`
- [ ] `go test -race` on the affected packages
- [ ] `pnpm contracts:lint`
- [ ] Not applicable

## Boundaries touched

- [ ] OpenAPI or AsyncAPI contract (regenerated with `pnpm contracts:generate`)
- [ ] Database schema (new forward migration, no edits to applied ones)
- [ ] SQL queries (regenerated with `pnpm db:generate`)
- [ ] Authentication, sessions, or ownership
- [ ] Quotas or rate limits
- [ ] Realtime lifecycle, ordering, or reconnect
- [ ] None of the above

## Evidence

<!--
For behavior changes, name the test that fails without this change and passes with
it. For a concurrency, shutdown, or quota change, name the test that drives the
dangerous path. For a visual change, attach before and after.
-->

## Confirmations

- [ ] The change fits the current milestone in `TASKS.md`
- [ ] Documentation affected by this change is updated in this pull request
- [ ] No secrets, credentials, real hostnames, LAN addresses, or local paths added
- [ ] No prompt or response content added to logs, metrics, traces, or audit records
- [ ] Any new dependency is justified against `docs/dependency-policy.md`
