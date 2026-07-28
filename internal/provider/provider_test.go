package provider

import (
	"context"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestProviderSchemas instantiates every resource and data source and runs the
// framework's schema implementation validation (catches e.g. computed
// attributes without defaults, invalid plan modifier combinations).
func TestProviderSchemas(t *testing.T) {
	ctx := context.Background()
	p := New("test")()

	provSchema := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, provSchema)
	if provSchema.Diagnostics.HasError() {
		t.Errorf("provider schema: %v", provSchema.Diagnostics)
	}

	for _, newResource := range p.Resources(ctx) {
		r := newResource()

		metaResp := &fwresource.MetadataResponse{}
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "uptimeeye"}, metaResp)

		schemaResp := &fwresource.SchemaResponse{}
		r.Schema(ctx, fwresource.SchemaRequest{}, schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Errorf("%s schema: %v", metaResp.TypeName, schemaResp.Diagnostics)
			continue
		}
		if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Errorf("%s schema implementation: %v", metaResp.TypeName, diags)
		}
	}

	for _, newDataSource := range p.DataSources(ctx) {
		d := newDataSource()

		metaResp := &fwdatasource.MetadataResponse{}
		d.Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "uptimeeye"}, metaResp)

		schemaResp := &fwdatasource.SchemaResponse{}
		d.Schema(ctx, fwdatasource.SchemaRequest{}, schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Errorf("%s schema: %v", metaResp.TypeName, schemaResp.Diagnostics)
			continue
		}
		if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Errorf("%s schema implementation: %v", metaResp.TypeName, diags)
		}
	}
}
