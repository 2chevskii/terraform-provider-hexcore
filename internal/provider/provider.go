package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	hexcore "github.com/2chevskii/go-hexcore"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type hexcoreProvider struct {
	version      string
	pollInterval time.Duration
}

type providerModel struct {
	Token    types.String `tfsdk:"token"`
	Endpoint types.String `tfsdk:"endpoint"`
}

// configuredClient is immutable after Configure; independent resources may poll concurrently.
type configuredClient struct {
	api          *hexcore.Client
	pollInterval time.Duration
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &hexcoreProvider{version: version, pollInterval: 5 * time.Second} }
}

func (p *hexcoreProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName, resp.Version = "hexcore", p.version
}

func (p *hexcoreProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage HexCore VDS and discover catalogs. Requires Terraform >= 1.11. Provider publication and live lifecycle verification are separate milestones.",
		Attributes: map[string]schema.Attribute{
			"token":    schema.StringAttribute{Optional: true, Sensitive: true, Description: "Raw API token without a Bearer prefix. Null uses HEXCORE_TOKEN; empty or unknown configured values never fall back.", Validators: []validator.String{tokenValidator{}}},
			"endpoint": schema.StringAttribute{Optional: true, Description: "API base URL for development/testing. Null uses the SDK default https://api.hexcore.ru/. No credentials, query, or fragment.", Validators: []validator.String{endpointValidator{}}},
		},
	}
}

func (p *hexcoreProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("token"), "Unknown API token", "The configured token must be known before the provider can perform operations.")
	}
	if data.Endpoint.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"), "Unknown API endpoint", "The configured endpoint must be known before the provider can perform operations.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	token := data.Token.ValueString()
	if data.Token.IsNull() {
		token = os.Getenv("HEXCORE_TOKEN")
	}
	if !validToken(token) {
		resp.Diagnostics.AddAttributeError(path.Root("token"), "Invalid API token", "Set token or HEXCORE_TOKEN to a nonempty raw token without whitespace or a Bearer prefix.")
		return
	}
	endpoint := hexcore.DefaultBaseURL
	if !data.Endpoint.IsNull() {
		endpoint = data.Endpoint.ValueString()
	}
	if !validEndpoint(endpoint) {
		resp.Diagnostics.AddAttributeError(path.Root("endpoint"), "Invalid API endpoint", "Use an absolute HTTP(S) URL without credentials, query, or fragment.")
		return
	}
	client, err := hexcore.NewClient(hexcore.Config{Token: token, BaseURL: endpoint, UserAgent: fmt.Sprintf("terraform-provider-hexcore/%s Terraform/%s", p.version, req.TerraformVersion)})
	if err != nil {
		resp.Diagnostics.AddError("Cannot configure HexCore", "The SDK rejected the provider configuration. Check token and endpoint formatting.")
		return
	}
	configured := &configuredClient{api: client, pollInterval: p.pollInterval}
	resp.ResourceData, resp.DataSourceData = configured, configured
}

func (p *hexcoreProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewVDSResource}
}

func (p *hexcoreProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource { return &catalogDataSource{kind: "regions"} },
		func() datasource.DataSource { return &catalogDataSource{kind: "sizes"} },
		func() datasource.DataSource { return &catalogDataSource{kind: "images"} },
		func() datasource.DataSource { return &catalogDataSource{kind: "applications"} },
		func() datasource.DataSource { return &vdsDataSource{} },
	}
}

func getConfigured(data any, diags *diag.Diagnostics) *configuredClient {
	if data == nil {
		return nil
	}
	c, ok := data.(*configuredClient)
	if !ok || c == nil || c.api == nil {
		diags.AddError("Unexpected provider data", "The resource or data source received an unexpected configured client type. Report this provider defect.")
		return nil
	}
	return c
}
