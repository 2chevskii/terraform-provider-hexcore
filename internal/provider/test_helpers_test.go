package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	hexcore "github.com/2chevskii/go-hexcore"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type localOnlyTransport struct{ next http.RoundTripper }

func (t localOnlyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.URL.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return t.next.RoundTrip(req)
	}
	return nil, errors.New("offline tests forbid non-loopback HTTP requests")
}
func TestMain(m *testing.M) {
	http.DefaultTransport = localOnlyTransport{http.DefaultTransport}
	_ = os.Setenv("CHECKPOINT_DISABLE", "1")
	_ = os.Setenv("TF_ACC_PROVIDER_NAMESPACE", "2chevskii")
	os.Exit(m.Run())
}

// Everything in this server is synthetic. The unrelated ID exercises cleanup isolation.
type stubAPI struct {
	mu                                                                        sync.Mutex
	server                                                                    *httptest.Server
	servers                                                                   map[string]hexcore.VDS
	pending                                                                   map[string]int
	nextID                                                                    int
	createCalls, destroyCalls, getCalls, listCalls                            int
	shutdowns                                                                 []string
	mutationIDs                                                               []string
	password, userAgent                                                       string
	getStatus, listStatus                                                     int
	notReady, malformedCreate, malformedShutdown, shutdownKeeps, destroyKeeps bool
	immediateDestroy                                                          bool
	uncertainDestroy                                                          bool
	actionStatus                                                              hexcore.ActionStatus
	regionAvailable                                                           bool
}

func str(s string) *string { return &s }
func syntheticVDS(id string) hexcore.VDS {
	return hexcore.VDS{ID: id, Name: "tf-offline", Region: str("test-region"), VCPU: 1, Memory: 1024, Disk: 10, IPv4: str("192.0.2.10"), Status: str("Running"), Price: str("12.3400"), CreatedAt: str("2000-01-01 10:20:30"), NextBillDate: "2000-02-01"}
}
func newStub(t *testing.T) *stubAPI {
	t.Helper()
	unrelated := syntheticVDS("62527")
	unrelated.Name = "unrelated-synthetic-object"
	b := &stubAPI{servers: map[string]hexcore.VDS{"62527": unrelated}, pending: map[string]int{}, nextID: 1000, actionStatus: hexcore.ActionCompleted, regionAvailable: true}
	b.server = httptest.NewServer(http.HandlerFunc(b.serveHTTP))
	t.Cleanup(b.server.Close)
	return b
}
func (b *stubAPI) client(t *testing.T) *configuredClient {
	t.Helper()
	c, err := hexcore.NewClient(hexcore.Config{Token: "offline-only", BaseURL: b.server.URL})
	if err != nil {
		t.Fatal(err)
	}
	return &configuredClient{api: c, pollInterval: time.Millisecond}
}
func (b *stubAPI) serveHTTP(w http.ResponseWriter, req *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	b.userAgent = req.Header.Get("User-Agent")
	write := func(status int, v any) { w.WriteHeader(status); _ = json.NewEncoder(w).Encode(v) }
	if req.Header.Get("Authorization") != "Bearer offline-only" && req.Header.Get("Authorization") != "Bearer configured-token" && req.Header.Get("Authorization") != "Bearer environment-token" {
		write(401, map[string]string{"message": "unauthorized"})
		return
	}
	if req.Method == "GET" {
		switch req.URL.Path {
		case "/regions":
			write(200, map[string]any{"regions": []hexcore.Region{{Slug: "z-unavailable", Name: "Unavailable", Available: false}, {Slug: "test-region", Name: "Synthetic", Available: b.regionAvailable}}})
			return
		case "/sizes":
			write(200, map[string]any{"sizes": []hexcore.Size{{Slug: "z-large", VCPU: 2, Memory: 2048, Disk: 20, MonthPrice: "24.6800", HourPrice: "0.0100"}, {Slug: "small", VCPU: 1, Memory: 1024, Disk: 10, MonthPrice: "12.3400", HourPrice: "0.0050"}}})
			return
		case "/images":
			write(200, map[string]any{"images": []hexcore.Image{{ID: "999999999999999999999999", Name: "Debian", Version: "12", Slug: "debian-12", MinDiskSize: 5}}})
			return
		case "/applications":
			write(200, map[string]any{"applications": []hexcore.Application{{ID: "80", Name: "Application", Version: "1.0", Slug: "app", MinDiskSize: 10}}})
			return
		case "/vds_list":
			b.listCalls++
			if b.listStatus != 0 {
				write(b.listStatus, map[string]string{"message": "offline-only should-never-leak"})
				return
			}
			for id, n := range b.pending {
				if n <= 0 {
					delete(b.pending, id)
					delete(b.servers, id)
				} else {
					b.pending[id]--
				}
			}
			items := []hexcore.VDS{}
			for _, v := range b.servers {
				items = append(items, v)
			}
			write(200, map[string]any{"vds": items})
			return
		}
		if strings.HasPrefix(req.URL.Path, "/vds/") {
			b.getCalls++
			if b.getStatus != 0 {
				write(b.getStatus, map[string]string{"message": "offline-only should-never-leak"})
				return
			}
			id := strings.TrimPrefix(req.URL.Path, "/vds/")
			v, ok := b.servers[id]
			if !ok {
				write(403, map[string]string{"message": "missing or inaccessible"})
				return
			}
			v.Status, v.Region, v.Price = nil, nil, nil // Live Get omits List fields.
			write(200, map[string]any{"vds": v})
			return
		}
		if strings.HasPrefix(req.URL.Path, "/actions/") {
			write(200, map[string]any{"actions": hexcore.Action{ID: "9000", Status: b.actionStatus, Message: str("offline-only should-never-leak")}})
			return
		}
	}
	if req.Method == "POST" && req.URL.Path == "/vds" {
		b.createCalls++
		var input struct {
			Name, Region, Image, Password string
			Size                          json.RawMessage
			CustomSize                    int                `json:"custom_size"`
			CustomSizeData                hexcore.CustomSize `json:"custom_size_data"`
		}
		if json.NewDecoder(req.Body).Decode(&input) != nil {
			write(400, map[string]string{"message": "invalid fixture request"})
			return
		}
		b.nextID++
		id := fmt.Sprint(b.nextID)
		v := syntheticVDS(id)
		v.Name, v.Region = input.Name, str(input.Region)
		b.password = input.Password
		if input.CustomSize == 1 {
			v.VCPU, v.Memory, v.Disk = input.CustomSizeData.VCPU, input.CustomSizeData.Memory*1024, input.CustomSizeData.Disk
		} else if string(input.Size) == `"z-large"` {
			v.VCPU, v.Memory, v.Disk = 2, 2048, 20
		}
		if b.notReady {
			v.Status, v.IPv4 = nil, nil
		}
		b.servers[id] = v
		if b.malformedCreate {
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"vds":`))
			return
		}
		created := v
		created.Status, created.IPv4, created.Region = nil, nil, nil
		write(202, map[string]any{"vds": created})
		return
	}
	if req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/action") {
		id := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/vds/"), "/action")
		b.mutationIDs = append(b.mutationIDs, id)
		if id == "62527" {
			write(400, map[string]string{"message": "unrelated object must never be mutated"})
			return
		}
		v, ok := b.servers[id]
		if !ok {
			write(403, map[string]string{"message": "missing"})
			return
		}
		var input struct{ Type string }
		_ = json.NewDecoder(req.Body).Decode(&input)
		switch input.Type {
		case "shutdown", "hard_shutdown":
			b.shutdowns = append(b.shutdowns, input.Type)
			if !b.shutdownKeeps {
				v.Status = str("halted")
				b.servers[id] = v
			}
			if b.malformedShutdown {
				w.WriteHeader(202)
				_, _ = w.Write([]byte(`{"action":`))
				return
			}
			write(202, map[string]any{"action": hexcore.Action{ID: "9000", ResourceID: id, Status: hexcore.ActionInProgress}})
			return
		case "destroy":
			b.destroyCalls++
			if v.Status == nil || *v.Status != "halted" {
				write(403, map[string]string{"message": "destroy before halted"})
				return
			}
			if !b.destroyKeeps {
				if b.immediateDestroy {
					delete(b.servers, id)
				} else {
					b.pending[id] = 1
				}
			}
			if b.uncertainDestroy {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(202)
				_, _ = w.Write([]byte("incomplete"))
				return
			}
			w.WriteHeader(202)
			return
		}
	}
	write(500, map[string]string{"message": "unexpected offline endpoint"})
}

func noErrors(t *testing.T, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}
func testModel() vdsModel {
	return vdsModel{ID: types.StringNull(), Name: types.StringValue("tf-offline"), Region: types.StringValue("test-region"), Image: types.StringValue("debian-12"), Size: types.StringValue("small"), CustomSize: types.ObjectNull(customSizeTypes), PasswordWO: types.StringNull(), DeleteMode: types.StringValue("graceful"), Timeouts: timeoutValue("100ms")}
}
func timeoutValue(d string) timeouts.Value {
	return timeouts.Value{Object: types.ObjectValueMust(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "delete": types.StringType}, map[string]attr.Value{"create": types.StringValue(d), "read": types.StringValue(d), "delete": types.StringValue(d)})}
}
func resourceState(t *testing.T, m vdsModel) tfsdk.State {
	t.Helper()
	var schemaResp resource.SchemaResponse
	(&vdsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	s := tfsdk.State{Schema: schemaResp.Schema}
	noErrors(t, s.Set(context.Background(), &m))
	return s
}
func createRequest(t *testing.T, m vdsModel, password string) resource.CreateRequest {
	t.Helper()
	plan := resourceState(t, m)
	m.PasswordWO = types.StringValue(password)
	if password == "" {
		m.PasswordWO = types.StringNull()
	}
	config := resourceState(t, m)
	return resource.CreateRequest{Plan: tfsdk.Plan{Schema: plan.Schema, Raw: plan.Raw}, Config: tfsdk.Config{Schema: config.Schema, Raw: config.Raw}}
}
