package provider

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	hexcore "github.com/2chevskii/go-hexcore"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestObservationReconciliation(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		getStatus, listStatus int
		exists                bool
		want                  presence
	}{
		{"get omits list fields", 0, 0, true, present},
		{"get 403 present", 403, 0, true, present},
		{"get 404 present", 404, 0, true, present},
		{"get 403 absent", 403, 0, false, absent},
		{"empty list", 0, 0, false, absent},
		{"both forbidden", 403, 403, true, indeterminate},
		{"get succeeds list fails", 0, 500, true, indeterminate},
		{"get 404 list fails", 404, 403, false, indeterminate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newStub(t)
			delete(b.servers, "62527")
			b.getStatus, b.listStatus = tc.getStatus, tc.listStatus
			if tc.exists {
				b.servers["123"] = syntheticVDS("123")
			}
			o, err := observeVDS(context.Background(), b.client(t).api, "123")
			if o.presence != tc.want || (err != nil) != (tc.want == indeterminate) {
				t.Fatalf("presence=%v error=%v", o.presence, err != nil)
			}
			if tc.want == present && (o.vds.Status == nil || o.vds.Region == nil || o.vds.Price == nil) {
				t.Fatal("list fields not reconciled")
			}
			if b.listCalls != 1 || b.getCalls != 1 {
				t.Fatal("expected one Get and one full List")
			}
		})
	}
}

func TestSizingAndNameValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*vdsModel)
		want   bool
	}{
		{"standard", func(m *vdsModel) {}, false},
		{"neither", func(m *vdsModel) { m.Size = types.StringNull() }, true},
		{"unknown size", func(m *vdsModel) { m.Size = types.StringUnknown() }, false},
		{"unknown custom", func(m *vdsModel) { m.Size = types.StringNull(); m.CustomSize = types.ObjectUnknown(customSizeTypes) }, false},
		{"both", func(m *vdsModel) { m.CustomSize = customObject(1, 1, 10) }, true},
		{"custom", func(m *vdsModel) { m.Size = types.StringNull(); m.CustomSize = customObject(1, 1, 10) }, false},
		{"zero", func(m *vdsModel) { m.Size = types.StringNull(); m.CustomSize = customObject(0, 1, 10) }, true},
		{"negative", func(m *vdsModel) { m.Size = types.StringNull(); m.CustomSize = customObject(1, 1, -1) }, true},
		{"overflow", func(m *vdsModel) { m.Size = types.StringNull(); m.CustomSize = customObject(1, math.MaxInt64, 10) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel()
			tc.modify(&m)
			var diags diag.Diagnostics
			validateSizing(m, false, &diags)
			if diags.HasError() != tc.want {
				t.Fatalf("%v", diags)
			}
		})
	}
	for _, tc := range []struct {
		s    types.String
		want bool
	}{
		{types.StringUnknown(), false}, {types.StringValue("valid"), false}, {types.StringValue(strings.Repeat("a", 45)), false}, {types.StringValue(strings.Repeat("a", 46)), true}, {types.StringValue(strings.Repeat("я", 23)), true}, {types.StringValue("\n"), true},
	} {
		var resp validator.StringResponse
		nameValidator{}.ValidateString(context.Background(), validator.StringRequest{ConfigValue: tc.s}, &resp)
		if resp.Diagnostics.HasError() != tc.want {
			t.Fatal("name validation differs")
		}
	}
	for _, id := range []string{"123", "9999999999999999999999999999999999999"} {
		if !idPattern.MatchString(id) {
			t.Fatal("string ID rejected")
		}
	}
}

func customObject(cpu, memory, disk int64) types.Object {
	return types.ObjectValueMust(customSizeTypes, map[string]attr.Value{"vcpu": types.Int64Value(cpu), "memory_gb": types.Int64Value(memory), "disk_gb": types.Int64Value(disk)})
}

func TestCreateStandardAndCustom(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprint(custom), func(t *testing.T) {
			b := newStub(t)
			r := &vdsResource{client: b.client(t)}
			m := testModel()
			m.Timeouts = timeoutValue("2s")
			if custom {
				m.Size = types.StringNull()
				m.CustomSize = customObject(2, 3, 15)
			}
			req := createRequest(t, m, "synthetic-password")
			resp := resource.CreateResponse{State: tfsdk.State{Schema: req.Plan.Schema}}
			r.Create(context.Background(), req, &resp)
			noErrors(t, resp.Diagnostics)
			var got vdsModel
			noErrors(t, resp.State.Get(context.Background(), &got))
			if got.ID.IsNull() || got.ID.ValueString() == "" || got.Status.ValueString() != "Running" || got.Price.ValueString() != "12.3400" || b.createCalls != 1 {
				t.Fatal("creation did not converge")
			}
			if !got.PasswordWO.IsNull() || strings.Contains(resp.State.Raw.String(), "synthetic-password") {
				t.Fatal("password persisted")
			}
			if b.password != "synthetic-password" {
				t.Fatal("password must come from configuration")
			}
			if custom && (got.MemoryMB.ValueInt64() != 3072 || got.VCPU.ValueInt64() != 2 || got.CustomSize.Attributes()["memory_gb"].(types.Int64).ValueInt64() != 3) {
				t.Fatal("custom units incorrect")
			}
		})
	}
}

func TestCreateFailureRecovery(t *testing.T) {
	for _, tc := range []struct {
		name                string
		notReady, malformed bool
		hasID               bool
	}{
		{"wait timeout", true, false, true}, {"uncertain decode", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newStub(t)
			b.notReady, b.malformedCreate = tc.notReady, tc.malformed
			r := &vdsResource{client: b.client(t)}
			m := testModel()
			m.Timeouts = timeoutValue("25ms")
			req := createRequest(t, m, "synthetic-password")
			resp := resource.CreateResponse{State: tfsdk.State{Schema: req.Plan.Schema}}
			r.Create(context.Background(), req, &resp)
			if !resp.Diagnostics.HasError() || b.createCalls != 1 || b.destroyCalls != 0 {
				t.Fatal("failure replayed or missing diagnostic")
			}
			if tc.hasID {
				var got vdsModel
				noErrors(t, resp.State.Get(context.Background(), &got))
				if got.ID.ValueString() != "1001" {
					t.Fatal("accepted ID lost")
				}
			}
			if strings.Contains(fmt.Sprint(resp.Diagnostics), "synthetic-password") {
				t.Fatal("secret in failure diagnostic")
			}
		})
	}
}

func TestCatalogValidationPreventsWrites(t *testing.T) {
	for _, change := range []func(*vdsModel){func(m *vdsModel) { m.Region = types.StringValue("missing") }, func(m *vdsModel) { m.Image = types.StringValue("missing") }, func(m *vdsModel) { m.Size = types.StringValue("missing") }, func(m *vdsModel) { m.Size = types.StringNull(); m.CustomSize = customObject(1, 1, 1) }} {
		b := newStub(t)
		r := &vdsResource{client: b.client(t)}
		m := testModel()
		change(&m)
		req := createRequest(t, m, "")
		resp := resource.CreateResponse{State: tfsdk.State{Schema: req.Plan.Schema}}
		r.Create(context.Background(), req, &resp)
		if !resp.Diagnostics.HasError() || b.createCalls != 0 {
			t.Fatal("invalid selection submitted")
		}
	}
}

func TestRefreshDriftAndRetention(t *testing.T) {
	ctx := context.Background()
	b := newStub(t)
	v := syntheticVDS("123")
	v.Name = "changed-remotely"
	v.Region = str("another-region")
	v.IPv4 = nil
	v.CreatedAt = nil
	v.NextBillDate = ""
	b.servers["123"] = v
	r := &vdsResource{client: b.client(t)}
	m := testModel()
	m.ID = types.StringValue("123")
	state := resourceState(t, m)
	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	noErrors(t, resp.Diagnostics)
	var got vdsModel
	noErrors(t, resp.State.Get(ctx, &got))
	if got.Name.ValueString() != "changed-remotely" || got.Region.ValueString() != "another-region" || got.Image != m.Image || got.Size != m.Size || !got.IPv4.IsNull() || !got.CreatedAt.IsNull() || !got.NextBillDate.IsNull() {
		t.Fatal("drift or null mapping incorrect")
	}
	b.getStatus, b.listStatus = 403, 403
	resp = resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
		t.Fatal("ambiguous 403 dropped state")
	}
	b.listStatus = 0
	delete(b.servers, "123")
	resp = resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	noErrors(t, resp.Diagnostics)
	if !resp.State.Raw.IsNull() {
		t.Fatal("confirmed absence did not remove state")
	}
}

func TestDeleteLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name, mode                                                           string
		initialHalted, alreadyAbsent, shutdownKeeps, destroyKeeps, malformed bool
		action                                                               hexcore.ActionStatus
		wantError                                                            bool
		wantShutdown, wantDestroy                                            int
	}{
		{name: "graceful", mode: "graceful", wantShutdown: 1, wantDestroy: 1},
		{name: "force", mode: "force", wantShutdown: 1, wantDestroy: 1},
		{name: "halted", mode: "graceful", initialHalted: true, wantDestroy: 1},
		{name: "already absent", alreadyAbsent: true},
		{name: "halt timeout", shutdownKeeps: true, wantError: true, wantShutdown: 1},
		{name: "destroy timeout", initialHalted: true, destroyKeeps: true, wantError: true, wantDestroy: 1},
		{name: "action failed", action: hexcore.ActionFailed, wantError: true, wantShutdown: 1},
		{name: "unknown action", action: "unrecognized", wantError: true, wantShutdown: 1},
		{name: "action in progress", action: hexcore.ActionInProgress, wantError: true, wantShutdown: 1},
		{name: "uncertain shutdown", malformed: true, wantError: true, wantShutdown: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newStub(t)
			v := syntheticVDS("123")
			if tc.initialHalted {
				v.Status = str("halted")
			}
			if !tc.alreadyAbsent {
				b.servers["123"] = v
			}
			b.shutdownKeeps, b.destroyKeeps, b.malformedShutdown = tc.shutdownKeeps, tc.destroyKeeps, tc.malformed
			if tc.action != "" {
				b.actionStatus = tc.action
			}
			r := &vdsResource{client: b.client(t)}
			m := testModel()
			m.ID = types.StringValue("123")
			m.DeleteMode = types.StringValue(tc.mode)
			m.Timeouts = timeoutValue("30ms")
			state := resourceState(t, m)
			resp := resource.DeleteResponse{State: state}
			r.Delete(context.Background(), resource.DeleteRequest{State: state}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError || len(b.shutdowns) != tc.wantShutdown || b.destroyCalls != tc.wantDestroy {
				t.Fatalf("errors=%v shutdowns=%v destroy=%d", resp.Diagnostics, b.shutdowns, b.destroyCalls)
			}
			if tc.wantShutdown > 0 {
				expected := "shutdown"
				if tc.mode == "force" {
					expected = "hard_shutdown"
				}
				if b.shutdowns[0] != expected {
					t.Fatal("wrong shutdown policy")
				}
			}
			for _, id := range b.mutationIDs {
				if id != "123" {
					t.Fatal("unrelated ID targeted")
				}
			}
			if tc.wantError && !resp.State.Raw.Equal(state.Raw) {
				t.Fatal("state lost on delete failure")
			}
			if !tc.wantError && !tc.alreadyAbsent && b.listCalls < 3 {
				t.Fatal("asynchronous deletion not polled")
			}
		})
	}
}

func TestCancellationAndIndeterminateDelete(t *testing.T) {
	for _, canceled := range []bool{true, false} {
		b := newStub(t)
		b.servers["123"] = syntheticVDS("123")
		if !canceled {
			b.listStatus = 403
		}
		r := &vdsResource{client: b.client(t)}
		m := testModel()
		m.ID = types.StringValue("123")
		state := resourceState(t, m)
		ctx, cancel := context.WithCancel(context.Background())
		if canceled {
			cancel()
		}
		resp := resource.DeleteResponse{State: state}
		r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
		cancel()
		if !resp.Diagnostics.HasError() || b.destroyCalls != 0 || len(b.shutdowns) != 0 {
			t.Fatal("indeterminate deletion mutated resource")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if pause(ctx, time.Hour) != context.Canceled || time.Since(start) > time.Second {
		t.Fatal("poll ignored cancellation")
	}
}

func TestStructuredImport(t *testing.T) {
	for _, tc := range []struct {
		name, input           string
		getStatus, listStatus int
		exists, wantError     bool
	}{
		{name: "standard", input: `{"id":"123","region":"test-region","image":"debian-12","size":"small"}`, exists: true},
		{name: "custom", input: `{"id":"123","region":"test-region","image":"debian-12","custom_size":{"vcpu":1,"memory_gb":1,"disk_gb":10}}`, exists: true},
		{name: "plain id", input: "123", wantError: true},
		{name: "malformed", input: "{", wantError: true},
		{name: "missing image", input: `{"id":"123","region":"test-region","size":"small"}`, wantError: true},
		{name: "bad ID", input: `{"id":"../123","region":"test-region","image":"debian-12","size":"small"}`, wantError: true},
		{name: "secret", input: `{"id":"123","region":"test-region","image":"debian-12","size":"small","token":"never-print-this"}`, wantError: true},
		{name: "missing VDS", input: `{"id":"123","region":"test-region","image":"debian-12","size":"small"}`, wantError: true},
		{name: "permission", input: `{"id":"123","region":"test-region","image":"debian-12","size":"small"}`, exists: true, getStatus: 403, listStatus: 403, wantError: true},
		{name: "shape", input: `{"id":"123","region":"test-region","image":"debian-12","size":"z-large"}`, exists: true, wantError: true},
		{name: "region", input: `{"id":"123","region":"another","image":"debian-12","size":"small"}`, exists: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newStub(t)
			if tc.exists {
				b.servers["123"] = syntheticVDS("123")
			}
			b.getStatus, b.listStatus = tc.getStatus, tc.listStatus
			r := &vdsResource{client: b.client(t)}
			state := resourceState(t, testModel())
			resp := resource.ImportStateResponse{State: state}
			r.ImportState(context.Background(), resource.ImportStateRequest{ID: tc.input}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("%v", resp.Diagnostics)
			}
			if strings.Contains(fmt.Sprint(resp.Diagnostics), "never-print-this") {
				t.Fatal("secret leaked")
			}
			if !tc.wantError {
				var m vdsModel
				noErrors(t, resp.State.Get(context.Background(), &m))
				if m.ID.ValueString() != "123" || m.Image.ValueString() != "debian-12" || m.DeleteMode.ValueString() != "graceful" {
					t.Fatal("import incomplete")
				}
			}
			if b.createCalls+b.destroyCalls+len(b.shutdowns) != 0 {
				t.Fatal("import mutated remote")
			}
		})
	}
	// Parse without machine-width integer conversion.
	if _, ok := parseImport(`{"id":"9999999999999999999999999999","region":"r","image":"i","size":"s"}`); !ok {
		t.Fatal("large ID rejected")
	}
}

func TestLocalOnlyUpdate(t *testing.T) {
	b := newStub(t)
	r := &vdsResource{client: b.client(t)}
	m := testModel()
	m.ID = types.StringValue("123")
	m.Status = types.StringValue("Running")
	m.IPv4 = types.StringValue("192.0.2.10")
	state := resourceState(t, m)
	m.DeleteMode = types.StringValue("force")
	m.Timeouts = timeoutValue("5s")
	plan := resourceState(t, m)
	resp := resource.UpdateResponse{State: state}
	r.Update(context.Background(), resource.UpdateRequest{State: state, Plan: tfsdk.Plan{Schema: plan.Schema, Raw: plan.Raw}}, &resp)
	noErrors(t, resp.Diagnostics)
	var got vdsModel
	noErrors(t, resp.State.Get(context.Background(), &got))
	if got.DeleteMode.ValueString() != "force" || got.Status.ValueString() != "Running" || b.getCalls+b.listCalls+b.createCalls+b.destroyCalls != 0 {
		t.Fatal("local policy update touched API or lost observations")
	}
	var password types.String
	noErrors(t, resp.State.GetAttribute(context.Background(), path.Root("password_wo"), &password))
	if !password.IsNull() {
		t.Fatal("write-only password persisted")
	}
}
