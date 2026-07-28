resource "uptimeeye_variable" "api_token" {
  key    = "API_TOKEN"
  value  = var.api_token
  secure = true
}
