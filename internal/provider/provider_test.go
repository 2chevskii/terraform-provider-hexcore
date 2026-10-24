package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	hexcore "github.com/2chevskii/go-hexcore"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProviderConfigure(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name            string
		token, endpoint types.String
		env             string
		wantError       bool
	}{
		{"environment", types.StringNull(), types.StringNull(), "environment-token", false},
		{"explicit precedence", types.StringValue("configured-token"), types.StringNull(), "invalid env token", false},
		{"missing", types.StringNull(), types.StringNull(), "", true},
		{"explicit empty", types.StringValue(""), types.StringNull(), "environment-token", true},
		{"unknown token", types.StringUnknown(), types.StringNull(), "environment-token", true},
		{"bearer", types.StringValue("Bearer secret"), types.StringNull(), "environment-token", true},
		{"whitespace", types.StringValue("secret\n"), types.StringNull(), "", true},
		{"unknown endpoint", types.StringValue("configured-token"), types.StringUnknown(), "", true},
		{"empty endpoint", types.StringValue("configured-token"), types.StringValue(""), "", true},
		{"endpoint credentials", types.StringValue("configured-token"), types.StringValue("https://user:secret@example.invalid/"), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HEXCORE_TOKEN", tc.env)
			p := &hexcoreProvider{version: "test"}
			var s provider.SchemaResponse
			p.Schema(ctx, provider.SchemaRequest{}, &s)
			state := tfsdk.State{Schema: s.Schema}
			noErrors(t, state.Set(ctx, &providerModel{Token: tc.token, Endpoint: tc.endpoint}))
			var resp provider.ConfigureResponse
			p.Configure(ctx, provider.ConfigureRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: state.Raw}, TerraformVersion: "1.14.7"}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("diagnostics: %v", resp.Diagnostics)
			}
			if !tc.wantError && (resp.ResourceData == nil || resp.ResourceData != resp.DataSourceData) {
				t.Fatal("client not shared")
			}
			for _, secret := range []string{"configured-token", "environment-token", "user:secret"} {
				if strings.Contains(fmt.Sprint(resp.Diagnostics), secret) {
					t.Fatal("secret in diagnostic")
				}
			}
		})
	}
	// An unreachable local endpoint still configures without making any request.
	p := &hexcoreProvider{version: "test"}
	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	noErrors(t, state.Set(ctx, &providerModel{Token: types.StringValue("configured-token"), Endpoint: types.StringValue("http://127.0.0.1:1")}))
	var resp provider.ConfigureResponse
	p.Configure(ctx, provider.ConfigureRequest{Config: tfsdk.Config{Schema: state.Schema, Raw: state.Raw}}, &resp)
	noErrors(t, resp.Diagnostics)
}

func TestConfigureInjection(t *testing.T) {
	for _, data := range []any{nil, "wrong", (*configuredClient)(nil), &configuredClient{}} {
		var rr resource.ConfigureResponse
		(&vdsResource{}).Configure(context.Background(), resource.ConfigureRequest{ProviderData: data}, &rr)
		var dr datasource.ConfigureResponse
		(&vdsDataSource{}).Configure(context.Background(), datasource.ConfigureRequest{ProviderData: data}, &dr)
		var cr datasource.ConfigureResponse
		(&catalogDataSource{}).Configure(context.Background(), datasource.ConfigureRequest{ProviderData: data}, &cr)
		if (data != nil) != rr.Diagnostics.HasError() || rr.Diagnostics.HasError() != dr.Diagnostics.HasError() || dr.Diagnostics.HasError() != cr.Diagnostics.HasError() {
			t.Fatal("injection diagnostics incorrect")
		}
	}
}

func TestSafeError(t *testing.T) {
	secret := "never-print-this-secret"
	for _, err := range []error{
		&hexcore.APIError{StatusCode: 403, Message: secret, Path: secret, Headers: http.Header{"Authorization": {secret}}},
		&hexcore.ActionError{Action: &hexcore.Action{ID: secret, Message: &secret}},
		errors.New(secret), fmt.Errorf("%s: %w", secret, context.Canceled), fmt.Errorf("%s: %w", secret, context.DeadlineExceeded),
	} {
		if got := safeError(err); got == "" || strings.Contains(got, secret) {
			t.Fatal("unsafe or empty diagnostic")
		}
	}
}

func TestTokenValidation(t *testing.T) {
	for _, v := range []types.String{types.StringNull(), types.StringUnknown(), types.StringValue("ok"), types.StringValue("Bearer secret"), types.StringValue("")} {
		var resp validator.StringResponse
		tokenValidator{}.ValidateString(context.Background(), validator.StringRequest{ConfigValue: v}, &resp)
		if resp.Diagnostics.HasError() != (!v.IsNull() && !v.IsUnknown() && !validToken(v.ValueString())) {
			t.Fatal("validation differs")
		}
	}
	var diags diag.Diagnostics
	if c := getConfigured(nil, &diags); c != nil || diags.HasError() {
		t.Fatal("nil validation data should be harmless")
	}
}
