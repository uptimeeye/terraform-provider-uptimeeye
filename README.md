# Terraform Provider for UptimeEye

Manage [UptimeEye](https://uptimeeye.com) monitors, scheduled tasks (heartbeats),
notification channels, status pages and variables declaratively with Terraform
(or OpenTofu).

```hcl
terraform {
  required_providers {
    uptimeeye = {
      source = "uptimeeye/uptimeeye"
    }
  }
}

provider "uptimeeye" {
  # or set UPTIMEEYE_API_KEY
  api_key = var.uptimeeye_api_key
}

resource "uptimeeye_notification_channel" "oncall" {
  name = "On-Call"
}

resource "uptimeeye_monitor" "api" {
  name                    = "API Healthcheck"
  notification_channel_id = uptimeeye_notification_channel.oncall.id
  locations               = ["eu-central-1"]

  steps = [{
    type = "http"
    request = {
      url = "https://api.example.com/health"
    }
    asserts = [{
      type     = "body"
      property = "statusCode"
      operator = "is"
      expected = "200"
    }]
  }]

  options = {
    tick_every = 60
    timeout    = 5000
  }
}
```

See [examples/](examples/) for a complete setup and [docs/](docs/) for the
generated per-resource documentation.

## Authentication

The provider authenticates with an organization API key (`ue_live_...`),
created in the UptimeEye app under **Settings → API Keys**. Supply it via the
`api_key` provider attribute or the `UPTIMEEYE_API_KEY` environment variable.

## Resources

| Resource | Notes |
|---|---|
| `uptimeeye_monitor` | HTTP monitors incl. multi-step checks; `paused` maps to the pause/resume API |
| `uptimeeye_scheduled_task` | Heartbeat/cron monitoring (`cron_task` or `simple_task`) |
| `uptimeeye_notification_channel` | Groups integrations, referenced by monitors/tasks |
| `uptimeeye_notification_integration` | One typed block per integration (slack, teams, discord, email, webhook, telegram, pagerduty) |
| `uptimeeye_variable` | Variables for monitor steps; `secure` values are write-only on the API |
| `uptimeeye_status_page` | Status page incl. sections (managed as one atomic document) |
| `uptimeeye_status_page_domain` | Custom domain; changes force replacement (no update API) |
| `uptimeeye_report_schedule` | Scheduled uptime report per status page; destroy disables it |

Data sources: `uptimeeye_locations`, `uptimeeye_notification_channel`.

All resources support `terraform import`; nested resources use composite IDs
(`<channel_id>/<integration_id>`, `<status_page_id>/<domain_id>`).

## Development

```sh
make build            # compile
make test             # unit tests
make generate-client  # re-export OpenAPI spec from ../ms-management and regenerate internal/apiclient
make docs             # regenerate docs/ from schema + examples/
```

To run a locally built provider against real configs, add a
[dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides)
to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "uptimeeye/uptimeeye" = "/path/to/go/bin"
  }
  direct {}
}
```

### Acceptance tests

Acceptance tests run against a real management API:

```sh
UPTIMEEYE_ENDPOINT=http://localhost:4444 UPTIMEEYE_API_KEY=ue_live_... make testacc
```

For a local backend start `ms-management` (`make dev` there) and create an API
key for a test organization.

## Hosting & Releasing

Primary development happens on GitLab
(`gitlab.com/launchx/uptimeeye/terraform-provider-uptimeeye`, CI via
`.gitlab-ci.yml`). The public **Terraform Registry can only publish providers
from public GitHub repositories**, so releases go through a GitHub mirror:

1. Create a public GitHub repo `terraform-provider-uptimeeye` under the chosen
   owner. The Registry derives the provider namespace from that owner:
   `github.com/<owner>/terraform-provider-uptimeeye` → `source = "<owner>/uptimeeye"`.
   The address in `main.go` and the examples assume `uptimeeye` — adjust if the
   owner differs.
2. In GitLab, configure push mirroring (Settings → Repository → Mirroring
   repositories) to that GitHub repo, including tags.
3. One-time setup on the GitHub repo: create a GPG signing key, add its public
   key in the Terraform Registry, set the repo secrets `GPG_PRIVATE_KEY` and
   `PASSPHRASE`, and register the provider in the Registry.
4. Release: `git tag v0.1.0 && git push origin --tags` on GitLab — the mirror
   forwards the tag to GitHub, where `.github/workflows/release.yml` builds,
   signs and publishes via goreleaser; the Registry picks the release up
   automatically.
