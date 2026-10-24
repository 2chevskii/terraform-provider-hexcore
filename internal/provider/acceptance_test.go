package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func requireTerraform(t *testing.T) {
	t.Helper()
	if p := os.Getenv("TF_ACC_TERRAFORM_PATH"); p != "" {
		if _, err := os.Stat(p); err != nil {
			t.Fatal("TF_ACC_TERRAFORM_PATH does not exist")
		}
		return
	}
	if _, err := exec.LookPath("terraform"); err != nil {
		t.Skip("offline CLI acceptance requires Terraform >= 1.11 on PATH or TF_ACC_TERRAFORM_PATH pointing to Terraform or OpenTofu; no binary is downloaded by tests")
	}
}
func offlineFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"hexcore": providerserver.NewProtocol6WithError(&hexcoreProvider{version: "offline-test", pollInterval: time.Millisecond})}
}
func offlineConfig(b *stubAPI, name, sizing, extra string) string {
	return fmt.Sprintf(`
terraform {
  required_version = ">= 1.11.0"
  required_providers {
    hexcore = {
      source = "registry.terraform.io/2chevskii/hexcore"
    }
  }
}
provider "hexcore" {
  token = "offline-only"
  endpoint = %q
}
resource "hexcore_vds" "test" {
  name = %q
  region = "test-region"
  image = "debian-12"
  %s
  %s
}
`, b.server.URL, name, sizing, extra)
}
func assertCleanStub(b *stubAPI) tfresource.TestCheckFunc {
	return func(_ *terraform.State) error {
		b.mu.Lock()
		defer b.mu.Unlock()
		if len(b.servers) != 1 || b.servers["62527"].Name != "unrelated-synthetic-object" {
			return errors.New("test-owned VDS left behind or unrelated object changed")
		}
		for _, id := range b.mutationIDs {
			if id == "62527" {
				return errors.New("unrelated ID targeted")
			}
		}
		return nil
	}
}
func importID(custom bool) tfresource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		r := s.RootModule().Resources["hexcore_vds.test"]
		if r == nil {
			return "", errors.New("resource missing")
		}
		m := map[string]any{"id": r.Primary.ID, "image": "debian-12", "region": "test-region"}
		if custom {
			m["custom_size"] = map[string]int64{"vcpu": 2, "memory_gb": 3, "disk_gb": 15}
		} else {
			m["size"] = "small"
		}
		b, err := json.Marshal(m)
		return string(b), err
	}
}

func TestOfflineAcceptanceStandard(t *testing.T) {
	requireTerraform(t)
	b := newStub(t)
	config := offlineConfig(b, "tf-offline", `size = "small"`, "")
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: offlineFactories(), CheckDestroy: assertCleanStub(b), Steps: []tfresource.TestStep{
		{Config: config, Check: tfresource.ComposeAggregateTestCheckFunc(tfresource.TestCheckResourceAttr("hexcore_vds.test", "memory_mb", "1024"), tfresource.TestCheckResourceAttr("hexcore_vds.test", "price", "12.3400"), tfresource.TestCheckResourceAttr("hexcore_vds.test", "status", "Running"))},
		{Config: config, PlanOnly: true, ConfigPlanChecks: tfresource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{ResourceName: "hexcore_vds.test", ImportState: true, ImportStateIdFunc: importID(false), ImportStateVerify: true},
		{ResourceName: "hexcore_vds.test", ImportState: true, ImportStateKind: tfresource.ImportBlockWithID, ImportStateIdFunc: importID(false)},
		{Config: config, PlanOnly: true, ConfigPlanChecks: tfresource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: config, PreConfig: func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			for id, v := range b.servers {
				if id != "62527" {
					v.IPv4 = str("192.0.2.20")
					v.Price = nil
					b.servers[id] = v
				}
			}
		}, Check: tfresource.TestCheckResourceAttr("hexcore_vds.test", "ipv4", "192.0.2.20")},
		{Config: offlineConfig(b, "tf-offline", `size = "small"`, `delete_mode = "force"`), ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("hexcore_vds.test", plancheck.ResourceActionUpdate)}}, Check: func(*terraform.State) error {
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.createCalls != 1 || len(b.shutdowns) != 0 {
				return errors.New("policy update mutated API")
			}
			return nil
		}},
		{Config: offlineConfig(b, "tf-replaced", `size = "small"`, ""), ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("hexcore_vds.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: tfresource.TestCheckResourceAttr("hexcore_vds.test", "name", "tf-replaced")},
	}})
}

func TestOfflineAcceptanceCustom(t *testing.T) {
	requireTerraform(t)
	b := newStub(t)
	config := offlineConfig(b, "tf-offline", `custom_size = {
    vcpu = 2
    memory_gb = 3
    disk_gb = 15
  }`, "")
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: offlineFactories(), CheckDestroy: assertCleanStub(b), Steps: []tfresource.TestStep{
		{Config: config, Check: tfresource.ComposeAggregateTestCheckFunc(tfresource.TestCheckResourceAttr("hexcore_vds.test", "memory_mb", "3072"), tfresource.TestCheckResourceAttr("hexcore_vds.test", "custom_size.memory_gb", "3"))},
		{ResourceName: "hexcore_vds.test", ImportState: true, ImportStateIdFunc: importID(true), ImportStateVerify: true},
		{ResourceName: "hexcore_vds.test", ImportState: true, ImportStateKind: tfresource.ImportBlockWithID, ImportStateIdFunc: importID(true)},
		{Config: config, PlanOnly: true, ConfigPlanChecks: tfresource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}})
}

type secretFreePlan struct{ secret string }

func (c secretFreePlan) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	b, err := json.Marshal(req.Plan)
	if err != nil {
		resp.Error = err
		return
	}
	if strings.Contains(string(b), c.secret) {
		resp.Error = errors.New("write-only secret present in Terraform JSON plan")
	}
}
func TestOfflineAcceptanceWriteOnly(t *testing.T) {
	requireTerraform(t)
	b := newStub(t)
	secret := "OfflineSecret-DO-NOT-PERSIST-761a"
	t.Setenv("TF_VAR_test_password", secret)
	logPath := filepath.Join(t.TempDir(), "provider.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	previousStderr := os.Stderr
	os.Stderr = logFile
	t.Cleanup(func() { os.Stderr = previousStderr; _ = logFile.Close() })
	t.Setenv("TF_LOG", "TRACE")
	config := func(version int) string {
		return offlineConfig(b, "tf-offline", `size = "small"`, fmt.Sprintf("password_wo = var.test_password\n  password_wo_version = %d", version)) + `
variable "test_password" {
  type = string
  sensitive = true
  ephemeral = true
}
`
	}
	checkState := func(s *terraform.State) error {
		body, err := json.Marshal(s)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), secret) {
			return errors.New("write-only secret in state")
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.password != secret {
			return errors.New("creation did not receive config password")
		}
		return nil
	}
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: offlineFactories(), CheckDestroy: assertCleanStub(b), Steps: []tfresource.TestStep{
		{Config: config(1), ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{secretFreePlan{secret}}}, Check: checkState},
		{Config: config(1), PlanOnly: true, ConfigPlanChecks: tfresource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan(), secretFreePlan{secret}}}},
		{Config: config(1), PreConfig: func() { t.Setenv("TF_VAR_test_password", "EditedWriteOnlyValue-NoGenerationChange") }, PlanOnly: true, ConfigPlanChecks: tfresource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan(), secretFreePlan{secret}, secretFreePlan{"EditedWriteOnlyValue-NoGenerationChange"}}}},
		{Config: config(2), PreConfig: func() { t.Setenv("TF_VAR_test_password", secret) }, ConfigPlanChecks: tfresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("hexcore_vds.test", plancheck.ResourceActionDestroyBeforeCreate), secretFreePlan{secret}}}, Check: checkState},
	}})
	logs, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal("cannot inspect provider logs")
	}
	if strings.Contains(string(logs), secret) || strings.Contains(string(logs), "EditedWriteOnlyValue-NoGenerationChange") {
		t.Fatal("write-only secret in provider logs")
	}
}

func TestOfflineAcceptanceDataSources(t *testing.T) {
	requireTerraform(t)
	b := newStub(t)
	config := offlineConfig(b, "tf-offline", `size = "small"`, "") + `
data "hexcore_regions" "test" {}
data "hexcore_sizes" "test" {}
data "hexcore_images" "test" {}
data "hexcore_applications" "test" {}
data "hexcore_vds" "test" {
  id = hexcore_vds.test.id
}
`
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: offlineFactories(), CheckDestroy: assertCleanStub(b), Steps: []tfresource.TestStep{{Config: config, Check: tfresource.ComposeAggregateTestCheckFunc(
		tfresource.TestCheckResourceAttr("data.hexcore_regions.test", "regions.#", "2"), tfresource.TestCheckResourceAttr("data.hexcore_regions.test", "regions.1.available", "false"), tfresource.TestCheckResourceAttr("data.hexcore_sizes.test", "sizes.0.month_price", "12.3400"), tfresource.TestCheckResourceAttr("data.hexcore_images.test", "images.0.id", "999999999999999999999999"), tfresource.TestCheckResourceAttr("data.hexcore_vds.test", "status", "Running"),
	)}}})
}

func TestOfflineAcceptancePlanning(t *testing.T) {
	requireTerraform(t)
	b := newStub(t)
	config := offlineConfig(b, "tf-offline", `size = "small"`, "")
	steps := []tfresource.TestStep{{Config: config}}
	for _, changed := range []string{
		offlineConfig(b, "tf-offline", `size = "z-large"`, ""),
		strings.Replace(config, `image = "debian-12"`, `image = "app"`, 1),
		strings.Replace(config, `region = "test-region"`, `region = "another-region"`, 1),
		offlineConfig(b, "tf-offline", `custom_size = { vcpu = 1, memory_gb = 1, disk_gb = 10 }`, ""),
	} {
		steps = append(steps, tfresource.TestStep{Config: changed, PlanOnly: true, ExpectNonEmptyPlan: true, ConfigPlanChecks: tfresource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectResourceAction("hexcore_vds.test", plancheck.ResourceActionDestroyBeforeCreate)}}})
	}
	steps = append(steps, tfresource.TestStep{Config: offlineConfig(b, "tf-offline", `size = "small"`, `custom_size = { vcpu = 1, memory_gb = 1, disk_gb = 10 }`), PlanOnly: true, ExpectError: regexp.MustCompile("Exactly one")})
	steps = append(steps, tfresource.TestStep{Config: config, PlanOnly: true})
	tfresource.UnitTest(t, tfresource.TestCase{ProtoV6ProviderFactories: offlineFactories(), CheckDestroy: assertCleanStub(b), Steps: steps})
}
