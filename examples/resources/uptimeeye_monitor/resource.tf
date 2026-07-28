resource "uptimeeye_monitor" "api" {
  name                    = "API Healthcheck"
  notification_channel_id = uptimeeye_notification_channel.oncall.id
  locations               = ["eu-central-1"]

  steps = [{
    name = "GET /health"
    type = "http"
    request = {
      url    = "https://api.example.com/health"
      method = "GET"
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
