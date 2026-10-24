package provider

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type importCustomSize struct {
	VCPU     int64 `json:"vcpu"`
	MemoryGB int64 `json:"memory_gb"`
	DiskGB   int64 `json:"disk_gb"`
}
type importMetadata struct {
	ID                string            `json:"id"`
	Image             string            `json:"image"`
	Region            string            `json:"region"`
	Size              *string           `json:"size"`
	CustomSize        *importCustomSize `json:"custom_size"`
	PasswordWOVersion *int64            `json:"password_wo_version"`
	DeleteMode        *string           `json:"delete_mode"`
	Timeouts          *importTimeouts   `json:"timeouts"`
}

type importTimeouts struct {
	Create *string `json:"create"`
	Read   *string `json:"read"`
	Delete *string `json:"delete"`
}

const importHelp = `Use a JSON identifier with a positive decimal string id, original image and region slugs, and exactly one of size or custom_size. Example: {"id":"123456","image":"debian-12","region":"example-region","size":"example-size"}. Custom sizing uses {"vcpu":1,"memory_gb":1,"disk_gb":10}. Optional delete_mode, positive timeouts and a non-secret password_wo_version number can preserve matching local settings. Never include tokens or passwords. The original image cannot be independently verified by the current SDK; plain-ID import cannot preserve creation metadata.`

func parseImport(s string) (importMetadata, bool) {
	var m importMetadata
	d := json.NewDecoder(strings.NewReader(s))
	d.DisallowUnknownFields()
	if d.Decode(&m) != nil {
		return m, false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return m, false
	}
	if !idPattern.MatchString(m.ID) || !slugPattern.MatchString(m.Image) || !slugPattern.MatchString(m.Region) || (m.Size == nil) == (m.CustomSize == nil) {
		return m, false
	}
	if m.Size != nil && !slugPattern.MatchString(*m.Size) {
		return m, false
	}
	if c := m.CustomSize; c != nil && (c.VCPU <= 0 || c.MemoryGB <= 0 || c.MemoryGB > math.MaxInt64/1024 || c.DiskGB <= 0) {
		return m, false
	}
	if m.PasswordWOVersion != nil && *m.PasswordWOVersion <= 0 {
		return m, false
	}
	if m.DeleteMode != nil && *m.DeleteMode != "graceful" && *m.DeleteMode != "force" {
		return m, false
	}
	if m.Timeouts != nil {
		for _, v := range []*string{m.Timeouts.Create, m.Timeouts.Read, m.Timeouts.Delete} {
			if v != nil {
				duration, err := time.ParseDuration(*v)
				if err != nil || duration <= 0 {
					return m, false
				}
			}
		}
	}
	return m, true
}

func (r *vdsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	metadata, ok := parseImport(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid structured import identifier", importHelp)
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Client unavailable", "Configure the provider before importing a VDS.")
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	o, err := observeVDS(opCtx, r.client.api, metadata.ID)
	if err != nil {
		resp.Diagnostics.AddError("Cannot import VDS", safeError(err)+" Presence is indeterminate; import did not adopt any object.")
		return
	}
	if o.presence != present {
		resp.Diagnostics.AddError("Cannot import missing VDS", "The complete account listing omitted the requested ID.")
		return
	}
	if o.vds.Region != nil && *o.vds.Region != metadata.Region {
		resp.Diagnostics.AddError("Import region mismatch", "The declared region does not match the observed VDS region.")
		return
	}
	var shape vdsShape
	if metadata.Size != nil {
		sizes, err := r.client.api.Sizes.List(opCtx)
		if err != nil {
			resp.Diagnostics.AddError("Cannot validate imported size", safeError(err))
			return
		}
		found := false
		for _, v := range sizes {
			if v.Slug == *metadata.Size {
				shape, found = vdsShape{v.VCPU, v.Memory, v.Disk}, true
				break
			}
		}
		if !found {
			resp.Diagnostics.AddError("Unknown imported size", "The declared standard size is absent from the current catalog; its shape cannot be verified.")
			return
		}
	} else {
		c := metadata.CustomSize
		shape = vdsShape{c.VCPU, c.MemoryGB * 1024, c.DiskGB}
	}
	if o.vds.VCPU != shape.vcpu || o.vds.Memory != shape.memoryMB || o.vds.Disk != shape.diskGB {
		resp.Diagnostics.AddError("Import shape mismatch", "Observed CPU/memory/disk do not match the declared provisioning metadata. Shape matching does not independently prove the original size slug.")
		return
	}
	m := vdsModel{
		ID: types.StringValue(metadata.ID), Name: types.StringValue(o.vds.Name), Region: types.StringValue(metadata.Region), Image: types.StringValue(metadata.Image),
		Size: types.StringNull(), CustomSize: types.ObjectNull(customSizeTypes), PasswordWO: types.StringNull(), PasswordWOVersion: types.Int64Null(), DeleteMode: types.StringValue("graceful"),
		Timeouts: timeouts.Value{Object: types.ObjectNull(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "delete": types.StringType})},
	}
	if metadata.Size != nil {
		m.Size = types.StringValue(*metadata.Size)
	} else {
		c := metadata.CustomSize
		obj, diags := types.ObjectValue(customSizeTypes, map[string]attr.Value{"vcpu": types.Int64Value(c.VCPU), "memory_gb": types.Int64Value(c.MemoryGB), "disk_gb": types.Int64Value(c.DiskGB)})
		resp.Diagnostics.Append(diags...)
		m.CustomSize = obj
	}
	// These optional, non-secret declarations preserve intentional local settings
	// on import; omitted fields retain the same defaults as new configuration.
	if metadata.PasswordWOVersion != nil {
		m.PasswordWOVersion = types.Int64Value(*metadata.PasswordWOVersion)
	}
	if metadata.DeleteMode != nil {
		m.DeleteMode = types.StringValue(*metadata.DeleteMode)
	}
	if metadata.Timeouts != nil {
		m.Timeouts = timeouts.Value{Object: types.ObjectValueMust(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "delete": types.StringType}, map[string]attr.Value{"create": pointerString(metadata.Timeouts.Create), "read": pointerString(metadata.Timeouts.Read), "delete": pointerString(metadata.Timeouts.Delete)})}
	}
	m.observe(o.vds, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
