default: build

.PHONY: build install test testacc generate-spec generate-client docs lint

build:
	go build ./...

install:
	go install .

test:
	go test ./... -v $(TESTARGS)

# Acceptance tests run against a real UptimeEye management API.
# Requires UPTIMEEYE_API_KEY and (for a local backend) UPTIMEEYE_ENDPOINT.
testacc:
	TF_ACC=1 go test ./internal/provider/ -v -timeout 120m $(TESTARGS)

# Re-export the OpenAPI spec from the ms-management source tree (no running
# server needed) and regenerate the Go client from it.
generate-spec:
	cd ../ms-management && go run ./cmd/openapi -v30 > ../terraform-provider-uptimeeye/openapi.yaml

generate-client: generate-spec
	mkdir -p internal/apiclient
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 -config oapi-codegen.yaml openapi.yaml

# Generate registry documentation from schema + examples/ into docs/.
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name uptimeeye

lint:
	go vet ./...
