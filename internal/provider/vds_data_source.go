package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type vdsDataSource struct{ client *configuredClient }

func (d *vdsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vds"
}
func (d *vdsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = getConfigured(req.ProviderData, &resp.Diagnostics)
}
func (d *vdsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	a := map[string]schema.Attribute{"id": schema.StringAttribute{Required: true, Description: "Positive decimal VDS ID as a string.", Validators: []validator.String{idValidator{}}}}
	for _, name := range []string{"name", "region", "ipv4", "status", "price", "created_at", "next_bill_date"} {
		a[name] = schema.StringAttribute{Computed: true, Description: observationDescription(name)}
	}
	for _, name := range []string{"vcpu", "memory_mb", "disk_gb"} {
		a[name] = schema.Int64Attribute{Computed: true, Description: observationDescription(name)}
	}
	resp.Schema = schema.Schema{Description: "Read one VDS by ID, reconciled with the complete account VDS list. Missing or indeterminate objects return errors.", Attributes: a}
}
func (d *vdsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client unavailable", "Configure the provider before reading a VDS.")
		return
	}
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if id.IsUnknown() || id.IsNull() || !idPattern.MatchString(id.ValueString()) {
		resp.Diagnostics.AddError("Invalid VDS ID", "A known positive decimal ID string is required.")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	o, err := observeVDS(ctx, d.client.api, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Cannot read VDS", safeError(err)+" Presence is indeterminate.")
		return
	}
	if o.presence != present {
		resp.Diagnostics.AddError("VDS not found", "The successful complete account listing did not contain the requested ID.")
		return
	}
	values := observedValues(o.vds)
	values["id"] = id
	for k, v := range values {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(k), v)...)
	}
}
