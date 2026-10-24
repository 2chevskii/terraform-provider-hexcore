package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCatalogs(t *testing.T) {
	for _, kind := range []string{"regions", "sizes", "images", "applications"} {
		t.Run(kind, func(t *testing.T) {
			b := newStub(t)
			d := &catalogDataSource{kind: kind, client: b.client(t)}
			var sr datasource.SchemaResponse
			d.Schema(context.Background(), datasource.SchemaRequest{}, &sr)
			resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(context.Background()), nil)}}
			d.Read(context.Background(), datasource.ReadRequest{}, &resp)
			noErrors(t, resp.Diagnostics)
			var list types.List
			noErrors(t, resp.State.GetAttribute(context.Background(), path.Root(kind), &list))
			if len(list.Elements()) == 0 {
				t.Fatal("catalog empty")
			}
			a := list.Elements()[0].(types.Object).Attributes()
			if kind == "sizes" && (a["slug"].(types.String).ValueString() != "small" || a["month_price"].(types.String).ValueString() != "12.3400") {
				t.Fatal("ordering or decimal text lost")
			}
			if kind == "regions" && len(list.Elements()) != 2 {
				t.Fatal("unavailable region filtered")
			}
			if kind == "images" && a["id"].(types.String).ValueString() != "999999999999999999999999" {
				t.Fatal("ID precision lost")
			}
		})
	}
}

func TestVDSDataSourceAbsenceAndErrors(t *testing.T) {
	for _, tc := range []struct {
		exists    bool
		status    int
		wantError bool
	}{{true, 0, false}, {false, 0, true}, {true, 403, true}} {
		b := newStub(t)
		if tc.exists {
			b.servers["123"] = syntheticVDS("123")
		}
		b.listStatus = tc.status
		d := &vdsDataSource{client: b.client(t)}
		var s datasource.SchemaResponse
		d.Schema(context.Background(), datasource.SchemaRequest{}, &s)
		state := tfsdk.State{Schema: s.Schema, Raw: tftypes.NewValue(s.Schema.Type().TerraformType(context.Background()), nil)}
		noErrors(t, state.SetAttribute(context.Background(), path.Root("id"), types.StringValue("123")))
		resp := datasource.ReadResponse{State: state}
		d.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: state.Raw}}, &resp)
		if resp.Diagnostics.HasError() != tc.wantError {
			t.Fatalf("%v", resp.Diagnostics)
		}
		if !tc.wantError && !strings.Contains(resp.State.Raw.String(), "Running") {
			t.Fatal("List status missing")
		}
		if tc.wantError && resp.State.Raw.IsNull() {
			t.Fatal("data source used managed absence semantics")
		}
	}
}
