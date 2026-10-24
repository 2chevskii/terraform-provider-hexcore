package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hexcore "github.com/2chevskii/go-hexcore"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestConfiguredAuthenticationAndUserAgent(t *testing.T) {
	for _, explicit := range []bool{true, false} {
		t.Setenv("HEXCORE_TOKEN", "environment-token")
		token := types.StringNull()
		want := "Bearer environment-token"
		if explicit {
			token = types.StringValue("configured-token")
			want = "Bearer configured-token"
		}
		var calls int
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.Header.Get("Authorization") != want {
				t.Error("token precedence incorrect")
			}
			if r.Header.Get("User-Agent") != "terraform-provider-hexcore/test-version Terraform/1.14.7" {
				t.Error("user agent missing versions")
			}
			_, _ = w.Write([]byte(`{"regions":[]}`))
		}))
		p := &hexcoreProvider{version: "test-version"}
		var sr provider.SchemaResponse
		p.Schema(context.Background(), provider.SchemaRequest{}, &sr)
		state := tfsdk.State{Schema: sr.Schema}
		noErrors(t, state.Set(context.Background(), &providerModel{Token: token, Endpoint: types.StringValue(s.URL)}))
		var response provider.ConfigureResponse
		p.Configure(context.Background(), provider.ConfigureRequest{Config: tfsdk.Config{Raw: state.Raw, Schema: state.Schema}, TerraformVersion: "1.14.7"}, &response)
		noErrors(t, response.Diagnostics)
		if calls != 0 {
			t.Fatal("Configure made a request")
		}
		_, err := response.DataSourceData.(*configuredClient).api.Regions.List(context.Background())
		if err != nil {
			t.Fatal("SDK request failed")
		}
		s.Close()
	}
}

func TestCatalogEmptyAndSDKErrors(t *testing.T) {
	for _, kind := range []string{"regions", "sizes", "images", "applications"} {
		for _, status := range []int{200, 403} {
			t.Run(fmt.Sprintf("%s/%d", kind, status), func(t *testing.T) {
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(status)
					if status == 200 {
						_ = json.NewEncoder(w).Encode(map[string]any{kind: []any{}})
					} else {
						_, _ = w.Write([]byte(`{"message":"never-log-catalog-secret"}`))
					}
				}))
				defer s.Close()
				c, err := hexcore.NewClient(hexcore.Config{Token: "offline-only", BaseURL: s.URL})
				if err != nil {
					t.Fatal(err)
				}
				d := &catalogDataSource{kind: kind, client: &configuredClient{api: c}}
				var schema datasource.SchemaResponse
				d.Schema(context.Background(), datasource.SchemaRequest{}, &schema)
				resp := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema, Raw: tftypes.NewValue(schema.Schema.Type().TerraformType(context.Background()), nil)}}
				d.Read(context.Background(), datasource.ReadRequest{}, &resp)
				if resp.Diagnostics.HasError() != (status != 200) {
					t.Fatal("SDK error not propagated")
				}
				if strings.Contains(fmt.Sprint(resp.Diagnostics), "never-log-catalog-secret") {
					t.Fatal("API message leaked")
				}
				if status == 200 {
					var list types.List
					noErrors(t, resp.State.GetAttribute(context.Background(), path.Root(kind), &list))
					if list.IsNull() || list.IsUnknown() || len(list.Elements()) != 0 {
						t.Fatal("empty catalog must be a known empty collection")
					}
				}
			})
		}
	}
}

func TestTimeoutAndUnknownNestedValidation(t *testing.T) {
	for _, d := range []string{"2s", "0s", "-1m"} {
		m := testModel()
		m.Timeouts = timeoutValue(d)
		state := resourceState(t, m)
		var resp resource.ValidateConfigResponse
		(&vdsResource{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: state.Schema, Raw: state.Raw}}, &resp)
		if resp.Diagnostics.HasError() != (d != "2s") {
			t.Fatal("nonpositive timeout accepted")
		}
	}
	m := testModel()
	m.Size = types.StringNull()
	m.CustomSize = types.ObjectValueMust(customSizeTypes, map[string]attr.Value{"vcpu": types.Int64Unknown(), "memory_gb": types.Int64Value(1), "disk_gb": types.Int64Value(10)})
	for _, requireKnown := range []bool{false, true} {
		var d diag.Diagnostics
		validateSizing(m, requireKnown, &d)
		if d.HasError() != requireKnown {
			t.Fatal("unknown custom quantity mishandled")
		}
	}
}

func TestUncertainDestroyIsNotReplayed(t *testing.T) {
	b := newStub(t)
	v := syntheticVDS("123")
	v.Status = str("halted")
	b.servers["123"] = v
	b.uncertainDestroy = true
	r := &vdsResource{client: b.client(t)}
	m := testModel()
	m.ID = types.StringValue("123")
	state := resourceState(t, m)
	resp := resource.DeleteResponse{State: state}
	r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() || b.destroyCalls != 1 || !resp.State.Raw.Equal(state.Raw) {
		t.Fatal("uncertain destroy was replayed or state lost")
	}
}

func TestPartialObservationIsNull(t *testing.T) {
	m := testModel()
	m.observe(&hexcore.VDS{ID: "123"}, false)
	if !m.IPv4.IsNull() || !m.Status.IsNull() || !m.Price.IsNull() || !m.CreatedAt.IsNull() || !m.VCPU.IsNull() || !m.MemoryMB.IsNull() || !m.DiskGB.IsNull() {
		t.Fatal("unavailable partial observations must be null")
	}
	if m.Image.ValueString() != "debian-12" || m.Size.ValueString() != "small" || m.Name.ValueString() != "tf-offline" {
		t.Fatal("creation declarations were overwritten")
	}
}

func TestImportLocalDeclarations(t *testing.T) {
	const base = `{"id":"123","image":"debian-12","region":"test-region","size":"small"`
	for _, suffix := range []string{`,"password_wo_version":0}`, `,"password_wo":"secret"}`, `,"timeouts":{"create":"0s"}}`, `,"timeouts":{"token":"secret"}}`, `,"delete_mode":"automatic"}`} {
		if _, ok := parseImport(base + suffix); ok {
			t.Fatal("invalid or secret import field accepted")
		}
	}
	b := newStub(t)
	b.servers["123"] = syntheticVDS("123")
	r := &vdsResource{client: b.client(t)}
	state := resourceState(t, testModel())
	resp := resource.ImportStateResponse{State: state}
	r.ImportState(context.Background(), resource.ImportStateRequest{ID: base + `,"password_wo_version":3,"delete_mode":"force","timeouts":{"create":"1m","delete":"3m"}}`}, &resp)
	noErrors(t, resp.Diagnostics)
	var m vdsModel
	noErrors(t, resp.State.Get(context.Background(), &m))
	if m.PasswordWOVersion.ValueInt64() != 3 || m.DeleteMode.ValueString() != "force" || m.Timeouts.Attributes()["delete"].(types.String).ValueString() != "3m" || !m.PasswordWO.IsNull() {
		t.Fatal("non-secret local declarations lost")
	}
	if b.createCalls+b.destroyCalls+len(b.shutdowns) != 0 {
		t.Fatal("import performed a mutation")
	}
}
