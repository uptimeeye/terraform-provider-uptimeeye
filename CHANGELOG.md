# Changelog

## Unreleased

- Regenerate the API client from the current UptimeEye API.
- Validate at plan time what the API now rejects: names of monitors,
  scheduled tasks, notification channels and status pages of 1 to 80
  characters, status page slugs (3 to 63 of `a-z`, `0-9`, `-`), variable
  keys (`^[A-Za-z_][A-Za-z0-9_]{0,63}$`) and values (at most 255
  characters), https webhook URLs for Teams, Discord and webhook
  integrations, and the ranges of the monitor options and the scheduled
  task grace period.
- `uptimeeye_monitor`: steps of type `tcp` (`host:port`).
- Examples: monitor `timeout` in seconds (was `5000`), a `statusCode`
  assertion, and `interval_type = "minute"` for simple scheduled tasks.

## v0.1.0 (2026-07-28)

Initial release.

- Resources: `uptimeeye_monitor`, `uptimeeye_scheduled_task`,
  `uptimeeye_notification_channel`, `uptimeeye_notification_integration`,
  `uptimeeye_variable`, `uptimeeye_status_page`,
  `uptimeeye_status_page_domain`, `uptimeeye_report_schedule`
- Data sources: `uptimeeye_locations`, `uptimeeye_notification_channel`
- `terraform import` support for all resources (composite IDs for
  integrations and status page domains)
