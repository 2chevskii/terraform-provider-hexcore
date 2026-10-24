package provider

import (
	hexcore "github.com/2chevskii/go-hexcore"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type vdsModel struct {
	ID                types.String   `tfsdk:"id"`
	Name              types.String   `tfsdk:"name"`
	Region            types.String   `tfsdk:"region"`
	Image             types.String   `tfsdk:"image"`
	Size              types.String   `tfsdk:"size"`
	CustomSize        types.Object   `tfsdk:"custom_size"`
	PasswordWO        types.String   `tfsdk:"password_wo"`
	PasswordWOVersion types.Int64    `tfsdk:"password_wo_version"`
	DeleteMode        types.String   `tfsdk:"delete_mode"`
	IPv4              types.String   `tfsdk:"ipv4"`
	Status            types.String   `tfsdk:"status"`
	VCPU              types.Int64    `tfsdk:"vcpu"`
	MemoryMB          types.Int64    `tfsdk:"memory_mb"`
	DiskGB            types.Int64    `tfsdk:"disk_gb"`
	Price             types.String   `tfsdk:"price"`
	CreatedAt         types.String   `tfsdk:"created_at"`
	NextBillDate      types.String   `tfsdk:"next_bill_date"`
	Timeouts          timeouts.Value `tfsdk:"timeouts"`
}

var customSizeTypes = map[string]attr.Type{"vcpu": types.Int64Type, "memory_gb": types.Int64Type, "disk_gb": types.Int64Type}

func pointerString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
func optionalString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
func observedQuantity(n int64) types.Int64 {
	if n <= 0 {
		return types.Int64Null()
	}
	return types.Int64Value(n)
}
func observedValues(v *hexcore.VDS) map[string]attr.Value {
	return map[string]attr.Value{
		"name": types.StringValue(v.Name), "region": pointerString(v.Region), "ipv4": pointerString(v.IPv4), "status": pointerString(v.Status),
		"vcpu": observedQuantity(v.VCPU), "memory_mb": observedQuantity(v.Memory), "disk_gb": observedQuantity(v.Disk),
		"price": pointerString(v.Price), "created_at": pointerString(v.CreatedAt), "next_bill_date": optionalString(v.NextBillDate),
	}
}

func (m *vdsModel) observe(v *hexcore.VDS, refreshInputs bool) {
	m.IPv4, m.Status, m.Price = pointerString(v.IPv4), pointerString(v.Status), pointerString(v.Price)
	m.VCPU, m.MemoryMB, m.DiskGB = observedQuantity(v.VCPU), observedQuantity(v.Memory), observedQuantity(v.Disk)
	m.CreatedAt, m.NextBillDate = pointerString(v.CreatedAt), optionalString(v.NextBillDate)
	if refreshInputs {
		m.Name = types.StringValue(v.Name)
		if v.Region != nil {
			m.Region = types.StringValue(*v.Region)
		}
	}
	m.PasswordWO = types.StringNull()
}

func observationDescription(s string) string {
	switch s {
	case "memory_mb":
		return "Observed memory in MB (1024 MB per custom GB)."
	case "disk_gb":
		return "Observed disk capacity in GB."
	case "vcpu":
		return "Observed virtual CPU count."
	case "price":
		return "Observed price as decimal text, without floating point conversion. Null when unavailable."
	case "created_at", "next_bill_date":
		return "Raw API timestamp/date text; no timezone is inferred. Null when unavailable."
	default:
		return "Observed " + s + "; null when unavailable."
	}
}
