package provider

import (
	"context"
	"fmt"
	"math"
	"time"

	hexcore "github.com/2chevskii/go-hexcore"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type vdsResource struct{ client *configuredClient }

var (
	_ resource.ResourceWithConfigure      = &vdsResource{}
	_ resource.ResourceWithValidateConfig = &vdsResource{}
	_ resource.ResourceWithImportState    = &vdsResource{}
)

func NewVDSResource() resource.Resource { return &vdsResource{} }
func (r *vdsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vds"
}
func (r *vdsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = getConfigured(req.ProviderData, &resp.Diagnostics)
}
func (r *vdsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	slugValidators := []validator.String{stringvalidator.RegexMatches(slugPattern, "Use a nonempty catalog slug without whitespace.")}
	a := map[string]schema.Attribute{
		"id":     schema.StringAttribute{Computed: true, Description: "Stable VDS ID as a string.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":   schema.StringAttribute{Required: true, Description: "VDS name. Changes replace the server. Limited to 45 UTF-8 bytes by conservative provider policy; the API was observed truncating at 45 characters and Unicode boundaries are unverified.", Validators: []validator.String{nameValidator{}}, PlanModifiers: replace},
		"region": schema.StringAttribute{Required: true, Description: "Region slug. Changes replace the server.", Validators: slugValidators, PlanModifiers: replace},
		"image":  schema.StringAttribute{Required: true, Description: "Image/application slug used for creation. Unreadable through the SDK; preserved on refresh. Changes replace the server.", Validators: slugValidators, PlanModifiers: replace},
		"size":   schema.StringAttribute{Optional: true, Description: "Standard size slug; exactly one of size and custom_size is required. Unreadable through the SDK; preserved on refresh. Changes replace the server.", Validators: slugValidators, PlanModifiers: replace},
		"custom_size": schema.SingleNestedAttribute{Optional: true, Description: "Custom sizing instead of a standard size. Changes replace the server.", PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()}, Attributes: map[string]schema.Attribute{
			"vcpu":      schema.Int64Attribute{Required: true, Description: "Positive virtual CPU count.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
			"memory_gb": schema.Int64Attribute{Required: true, Description: "Memory requested in GB, reported as MB at 1024 MB/GB.", Validators: []validator.Int64{int64validator.Between(1, math.MaxInt64/1024)}},
			"disk_gb":   schema.Int64Attribute{Required: true, Description: "Positive disk capacity in GB, subject to the selected image minimum.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
		}},
		"password_wo":         schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, Description: "Optional root password sent only during creation, read from configuration and never persisted. Increment password_wo_version to replace the VDS when changing it. Default credential delivery when omitted is unverified; do not assume it is emailed or recoverable from state.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
		"password_wo_version": schema.Int64Attribute{Optional: true, Description: "Positive non-secret generation number. Changes replace the whole VDS, using password_wo; changing only the write-only password cannot produce a diff.", Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"delete_mode":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("graceful"), Description: "Local deletion policy: graceful (default) or force (explicit power cut). Updating this setting only updates state. Never automatically escalates to force.", Validators: []validator.String{stringvalidator.OneOf("graceful", "force")}},
		"timeouts":            timeouts.Attributes(ctx, timeouts.Opts{Create: true, Read: true, Delete: true, CreateDescription: "Creation budget, default 15m.", ReadDescription: "Refresh budget, default 2m.", DeleteDescription: "Deletion budget, default 20m. Apply timeout changes before destroying."}),
	}
	for _, name := range []string{"ipv4", "status", "price", "created_at", "next_bill_date"} {
		a[name] = schema.StringAttribute{Computed: true, Description: observationDescription(name)}
	}
	for _, name := range []string{"vcpu", "memory_mb", "disk_gb"} {
		a[name] = schema.Int64Attribute{Computed: true, Description: observationDescription(name)}
	}
	resp.Schema = schema.Schema{Description: "Manage a standard or custom HexCore VDS. All provisioning inputs require replacement. Shutdown and destruction are bounded and confirmed through resource observations. Only timeout and deletion-policy changes are state-only updates. Import requires JSON provisioning metadata; image metadata cannot be independently verified.", Attributes: a}
}

func (r *vdsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m vdsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateSizing(m, false, &resp.Diagnostics)
	if !m.Timeouts.IsNull() && !m.Timeouts.IsUnknown() {
		for k, v := range m.Timeouts.Attributes() {
			s := v.(types.String)
			if s.IsNull() || s.IsUnknown() {
				continue
			}
			d, err := time.ParseDuration(s.ValueString())
			if err != nil || d <= 0 {
				resp.Diagnostics.AddAttributeError(path.Root("timeouts").AtName(k), "Invalid timeout", "Operation timeouts must be positive durations.")
			}
		}
	}
}

func validateSizing(m vdsModel, requireKnown bool, diags *diag.Diagnostics) {
	if m.Size.IsUnknown() || m.CustomSize.IsUnknown() {
		if requireKnown {
			diags.AddError("Unknown sizing", "Sizing must be known before creating a VDS.")
		}
		return
	}
	if m.Size.IsNull() == m.CustomSize.IsNull() {
		diags.AddError("Invalid sizing", "Exactly one of size and custom_size must be configured.")
		return
	}
	if !m.CustomSize.IsNull() {
		for k, v := range m.CustomSize.Attributes() {
			n := v.(types.Int64)
			if n.IsUnknown() && !requireKnown {
				continue
			}
			if n.IsUnknown() || n.IsNull() || n.ValueInt64() <= 0 || (k == "memory_gb" && n.ValueInt64() > math.MaxInt64/1024) {
				diags.AddAttributeError(path.Root("custom_size").AtName(k), "Invalid custom quantity", "Use a known positive int64 quantity; memory_gb must fit in int64 after multiplying by 1024.")
			}
		}
	}
}

func (r *vdsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Client unavailable", "Configure the provider before creating a VDS.")
		return
	}
	var m vdsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	var password types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("password_wo"), &password)...)
	validateSizing(m, true, &resp.Diagnostics)
	if password.IsUnknown() {
		resp.Diagnostics.AddError("Unknown creation password", "password_wo must be known at creation.")
	}
	for _, s := range []types.String{m.Name, m.Region, m.Image} {
		if s.IsUnknown() || s.IsNull() {
			resp.Diagnostics.AddError("Unknown creation input", "Name, region and image must be known at creation.")
			break
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	budget, diags := m.Timeouts.Create(ctx, 15*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	shape, err := r.validateCatalogs(opCtx, m)
	if err != nil {
		resp.Diagnostics.AddError("Cannot validate creation inputs", safeError(err))
		return
	}
	var created *hexcore.VDS
	if !m.Size.IsNull() {
		created, err = r.client.api.VDS.Create(opCtx, hexcore.CreateVDSRequest{Name: m.Name.ValueString(), Region: m.Region.ValueString(), Image: m.Image.ValueString(), Size: m.Size.ValueString(), Password: password.ValueString()})
	} else {
		attrs := m.CustomSize.Attributes()
		created, err = r.client.api.VDS.CreateCustom(opCtx, hexcore.CreateCustomVDSRequest{Name: m.Name.ValueString(), Region: m.Region.ValueString(), Image: m.Image.ValueString(), Password: password.ValueString(), Size: hexcore.CustomSize{VCPU: attrs["vcpu"].(types.Int64).ValueInt64(), Memory: attrs["memory_gb"].(types.Int64).ValueInt64(), Disk: attrs["disk_gb"].(types.Int64).ValueInt64()}})
	}
	if err != nil {
		resp.Diagnostics.AddError("VDS creation did not return an ID", safeError(err)+" Creation may have been accepted. Do not blindly apply again: inspect the account VDS list/control panel, recover the ID and original selectors, then use structured import. Names are not unique; the provider never adopts or deletes a same-name object automatically.")
		return
	}
	m.ID = types.StringValue(created.ID)
	m.observe(created, false)
	// Persist valid partial state before any subsequent request/wait can fail.
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	last, err := waitVDS(opCtx, r.client, created.ID, func(o observation) bool {
		return o.presence == present && ready(o.vds, m.Name.ValueString(), m.Region.ValueString(), shape)
	})
	if last.vds != nil {
		m.observe(last.vds, false)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	if err != nil {
		resp.Diagnostics.AddError("VDS readiness incomplete", fmt.Sprintf("VDS ID %s is retained in state. %s Verify Running status, assigned IPv4, region and expected name/shape in the control panel before further changes. Creation was submitted once.", created.ID, safeError(err)))
	}
}

// Inputs are checked against current catalogs at apply, without any mutation.
// This API publishes no general custom-plan bounds; do not invent account limits.
func (r *vdsResource) validateCatalogs(ctx context.Context, m vdsModel) (vdsShape, error) {
	regions, err := r.client.api.Regions.List(ctx)
	if err != nil {
		return vdsShape{}, err
	}
	available := false
	for _, v := range regions {
		if v.Slug == m.Region.ValueString() && v.Available {
			available = true
		}
	}
	if !available {
		return vdsShape{}, &catalogError{"The region is absent or unavailable in the current catalog."}
	}
	var shape vdsShape
	if !m.Size.IsNull() {
		sizes, err := r.client.api.Sizes.List(ctx)
		if err != nil {
			return shape, err
		}
		found := false
		for _, v := range sizes {
			if v.Slug == m.Size.ValueString() {
				found = true
				shape = vdsShape{v.VCPU, v.Memory, v.Disk}
				break
			}
		}
		if !found || shape.vcpu <= 0 || shape.memoryMB <= 0 || shape.diskGB <= 0 {
			return shape, &catalogError{"The size is absent or has invalid quantities in the current catalog."}
		}
	} else {
		a := m.CustomSize.Attributes()
		shape = vdsShape{a["vcpu"].(types.Int64).ValueInt64(), a["memory_gb"].(types.Int64).ValueInt64() * 1024, a["disk_gb"].(types.Int64).ValueInt64()}
	}
	images, err := r.client.api.Images.List(ctx)
	if err != nil {
		return shape, err
	}
	found, minimum := false, int64(0)
	for _, v := range images {
		if v.Slug == m.Image.ValueString() {
			found, minimum = true, v.MinDiskSize
			break
		}
	}
	if !found {
		apps, err := r.client.api.Applications.List(ctx)
		if err != nil {
			return shape, err
		}
		for _, v := range apps {
			if v.Slug == m.Image.ValueString() {
				found, minimum = true, v.MinDiskSize
				break
			}
		}
	}
	if !found {
		return shape, &catalogError{"The image/application slug is absent from the current catalogs."}
	}
	if shape.diskGB < minimum {
		return shape, &catalogError{"The requested disk is smaller than the current image minimum."}
	}
	return shape, nil
}

// Only internally authored strings may be included verbatim in diagnostics.
type catalogError struct{ message string }

func (e *catalogError) Error() string { return e.message }

func (r *vdsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m vdsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client unavailable", "Configure the provider before reading a VDS.")
		return
	}
	budget, diags := m.Timeouts.Read(ctx, 2*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	o, err := observeVDS(opCtx, r.client.api, m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("VDS presence indeterminate", safeError(err)+" State has been retained; only a successful complete account listing can establish absence.")
		return
	}
	if o.presence == absent {
		resp.State.RemoveResource(ctx)
		return
	}
	m.observe(o.vds, true)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *vdsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vdsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Provisioning inputs have replacement modifiers. Only local policy/timeouts
	// can reach Update; retain current observations without issuing an API write.
	state.DeleteMode, state.Timeouts = plan.DeleteMode, plan.Timeouts
	state.PasswordWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vdsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m vdsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client unavailable", "Configure the provider before deleting a VDS.")
		return
	}
	budget, diags := m.Timeouts.Delete(ctx, 20*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	id := m.ID.ValueString()
	fail := func(err error) {
		resp.Diagnostics.AddError("VDS deletion incomplete", fmt.Sprintf("VDS ID %s remains tracked. %s Inspect the control panel before retrying. No uncertain mutation is replayed within this operation; graceful shutdown never escalates to force.", id, safeError(err)))
	}
	o, err := observeVDS(opCtx, r.client.api, id)
	if err != nil {
		fail(err)
		return
	}
	if o.presence == absent {
		return
	}
	if !halted(o) {
		var action *hexcore.Action
		if m.DeleteMode.ValueString() == "force" {
			action, err = r.client.api.VDS.HardShutdown(opCtx, id)
		} else {
			action, err = r.client.api.VDS.Shutdown(opCtx, id)
		}
		if err != nil {
			fail(err)
			return
		}
		// An accepted action is not completion. Unknown/action-error statuses fail.
		_, err = r.client.api.Actions.Wait(opCtx, action.ID, hexcore.WaitOptions{PollInterval: r.client.pollInterval})
		if err != nil {
			fail(err)
			return
		}
		o, err = waitVDS(opCtx, r.client, id, func(o observation) bool { return o.presence == absent || halted(o) })
		if err != nil {
			fail(err)
			return
		}
		if o.presence == absent {
			return
		}
	}
	// Destroy is submitted once, only after observing the exact ID halted.
	if err := r.client.api.VDS.Destroy(opCtx, id); err != nil {
		fail(err)
		return
	}
	_, err = waitVDS(opCtx, r.client, id, func(o observation) bool { return o.presence == absent })
	if err != nil {
		fail(err)
	}
}
