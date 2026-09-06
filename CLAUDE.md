# CLAUDE.md

Terraform provider for UptimeEye, built on terraform-plugin-framework (NOT the
legacy SDKv2). Companion backend: `../ms-management` (Go/Huma API service).

## Commands

```bash
make build            # compile
make test             # unit tests
make testacc          # acceptance tests (needs UPTIMEEYE_API_KEY, UPTIMEEYE_ENDPOINT)
make generate-client  # re-export OpenAPI spec from ../ms-management, regen internal/apiclient
make docs             # regenerate docs/ via tfplugindocs from schema + examples/
```

## Architecture

- `internal/apiclient/` — GENERATED (oapi-codegen) from `openapi.yaml`; never edit
  by hand, run `make generate-client` after backend API changes. The spec comes
  from `../ms-management` via `go run ./cmd/openapi -v30` (no server/DB needed).
- `internal/provider/` — provider + one file per resource/data source.
  - Shared helpers in `helpers.go`: `apiCallFailed`/`checkStatus`/`isNotFound`
    for API error handling, `stringSliceToList`/`listToStringSlice`, `ptr`/`deref`.
  - Every resource: tfsdk model struct, schema with MarkdownDescription,
    Configure via `clientFromResourceConfigure`, ImportState.
  - Every optional+computed attribute has a schema default so apply leaves no
    unknown values (plan==state must hold after apply).

## API semantics to keep in mind

- Auth: `Authorization: Bearer ue_live_...` org API keys (backend distinguishes
  them from Clerk JWTs by prefix). `/v1/api-keys` itself is JWT-only.
- Monitor: `status` is server-managed (PAUSED via pause/resume endpoints →
  `paused` attribute); steps are sent without ids (server re-creates them);
  `message` in the API DTO is dead — never send/expose it.
- Slack integrations: read endpoint returns only `channel`, never `webhookUrl` —
  Read must keep the state's webhook_url.
- Secure variables: API never returns the value — Read keeps the state value.
- StatusPage sections are upserted as part of the page document (no own API).
- ReportSchedule is a per-page singleton with PUT-upsert; destroy = enabled=false.
- StatusPageDomain has no update API — all attributes RequiresReplace.

## Hosting & Releasing

Primary repo: GitLab (`gitlab.com/launchx/uptimeeye/terraform-provider-uptimeeye`,
CI in .gitlab-ci.yml). The Terraform Registry only publishes from public GitHub
repos, so releases run on a GitHub push-mirror: tag on GitLab → mirror forwards
→ .github/workflows/release.yml (goreleaser + GPG). Registry namespace = GitHub
owner; adjust `main.go` Address + examples if the owner is not `uptimeeye`.
Details in README "Hosting & Releasing".

## Ops vault (company memory)

Strategy, marketing, experiments, decisions, reviews and the inventory of all initiatives live in the LaunchX
**ops vault**: `~/dev/privat/launchx/ops/` (GitLab `launchx/ops`, opened in Obsidian; rules in its `CLAUDE.md`;
clone `git@gitlab.com:launchx/ops.git` if it is missing on this machine). Product hub for this repo: `ops/10-Produkte/UptimeEye/UptimeEye.md`.
- Before non-trivial work, read the product hub and the open levers for this app (`ops/20-Hebel/`).
- New facts that are not code — numbers (with date and source), experiment results, marketing findings, plan
  changes, started or abandoned initiatives — go into the vault, not into loose `.md` files in this repo:
  hub = current state, `20-Hebel/` = experiments, `50-Reviews/` = run reports, `60-Ideen/` = ideas,
  `40-Entscheidungen/` only with `status: vorgeschlagen` (humans decide), `Anlaeufe.md` = initiative status.
- Technical docs (API contracts, runbooks, this file) stay in the repo; link them from the hub.
- Vault edits: `git -C ~/dev/privat/launchx/ops pull --rebase` first, commit as `docs(ops): …`, push only
  when the task explicitly allows it.
