provider "uptimeeye" {
  # Create the key in the UptimeEye app under Settings → API Keys.
  # Can also be provided via the UPTIMEEYE_API_KEY environment variable.
  api_key = var.uptimeeye_api_key
}
