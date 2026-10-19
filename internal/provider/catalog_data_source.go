package provider

import (
	"context"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type catalogDataSource struct {
	kind   string
	client *configuredClient
}

func (d *catalogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.kind
}
func (d *catalogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = getConfigured(req.ProviderData, &resp.Diagnostics)
}

func catalogAttributes(kind string) map[string]schema.Attribute {
	a := map[string]schema.Attribute{
		"slug": schema.StringAttribute{Computed: true, Description: "Catalog slug."},
	}
	switch kind {
	case "regions":
		a["name"] = schema.StringAttribute{Computed: true, Description: "Display name."}
		a["available"] = schema.BoolAttribute{Computed: true, Description: "Whether the region currently accepts creation. Unavailable regions remain visible."}
	case "sizes":
		a["vcpu"] = schema.Int64Attribute{Computed: true, Description: "Virtual CPU count."}
		a["memory_mb"] = schema.Int64Attribute{Computed: true, Description: "Memory in MB."}
		a["disk_gb"] = schema.Int64Attribute{Computed: true, Description: "Disk in GB."}
		a["month_price"] = schema.StringAttribute{Computed: true, Description: "Monthly price as decimal text in rubles."}
		a["hour_price"] = schema.StringAttribute{Computed: true, Description: "Hourly price as decimal text in rubles."}
	default:
		a["id"] = schema.StringAttribute{Computed: true, Description: "Image ID as a string."}
		a["name"] = schema.StringAttribute{Computed: true, Description: "Display name."}
		a["version"] = schema.StringAttribute{Computed: true, Description: "Image version text."}
		a["min_disk_gb"] = schema.Int64Attribute{Computed: true, Description: "Minimum disk size in GB."}
	}
	return a
}

func (d *catalogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Read the HexCore " + d.kind + " catalog. Results are ordered by slug, then ID where available.", Attributes: map[string]schema.Attribute{
		d.kind: schema.ListNestedAttribute{Computed: true, Description: "Catalog entries in stable order.", NestedObject: schema.NestedAttributeObject{Attributes: catalogAttributes(d.kind)}},
	}}
}

func (d *catalogDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Client unavailable", "Configure the provider before reading catalogs.")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	values := []map[string]attr.Value{}
	var err error
	switch d.kind {
	case "regions":
		items, e := d.client.api.Regions.List(ctx)
		err = e
		for _, v := range items {
			values = append(values, map[string]attr.Value{"slug": types.StringValue(v.Slug), "name": types.StringValue(v.Name), "available": types.BoolValue(v.Available)})
		}
	case "sizes":
		items, e := d.client.api.Sizes.List(ctx)
		err = e
		for _, v := range items {
			values = append(values, map[string]attr.Value{"slug": types.StringValue(v.Slug), "vcpu": types.Int64Value(v.VCPU), "memory_mb": types.Int64Value(v.Memory), "disk_gb": types.Int64Value(v.Disk), "month_price": types.StringValue(v.MonthPrice), "hour_price": types.StringValue(v.HourPrice)})
		}
	case "images":
		items, e := d.client.api.Images.List(ctx)
		err = e
		for _, v := range items {
			values = append(values, imageValues(v.ID, v.Name, v.Version, v.Slug, v.MinDiskSize))
		}
	case "applications":
		items, e := d.client.api.Applications.List(ctx)
		err = e
		for _, v := range items {
			values = append(values, imageValues(v.ID, v.Name, v.Version, v.Slug, v.MinDiskSize))
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Cannot read catalog", safeError(err))
		return
	}
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i]["slug"].(types.String).ValueString(), values[j]["slug"].(types.String).ValueString()
		if a != b {
			return a < b
		}
		if values[i]["id"] != nil {
			return values[i]["id"].(types.String).ValueString() < values[j]["id"].(types.String).ValueString()
		}
		return false
	})
	attrs := map[string]attr.Type{}
	for k, v := range catalogAttributes(d.kind) {
		attrs[k] = v.GetType()
	}
	objects := make([]attr.Value, len(values))
	for i, v := range values {
		object, diags := types.ObjectValue(attrs, v)
		resp.Diagnostics.Append(diags...)
		objects[i] = object
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: attrs}, objects)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(d.kind), list)...)
}

func imageValues(id, name, version, slug string, disk int64) map[string]attr.Value {
	return map[string]attr.Value{"id": types.StringValue(id), "name": types.StringValue(name), "version": types.StringValue(version), "slug": types.StringValue(slug), "min_disk_gb": types.Int64Value(disk)}
}
