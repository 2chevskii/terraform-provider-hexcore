package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// In addition to in-process protocol factories, exercise actual local discovery
// and the documented examples using a compiled provider and a dev_overrides file.
func TestOfflineCLIExamples(t *testing.T) {
	requireTerraform(t)
	terraformPath := os.Getenv("TF_ACC_TERRAFORM_PATH")
	if terraformPath == "" {
		var err error
		terraformPath, err = exec.LookPath("terraform")
		if err != nil {
			t.Fatal(err)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	binary := "terraform-provider-hexcore"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, binary), ".")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("provider build: %s", out)
	}
	cliConfig := filepath.Join(t.TempDir(), "terraform.rc")
	content := fmt.Sprintf("provider_installation {\n  dev_overrides {\n    \"registry.terraform.io/2chevskii/hexcore\" = %q\n  }\n  direct {}\n}\n", filepath.ToSlash(binDir))
	if err := os.WriteFile(cliConfig, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	providerConfig, err := os.ReadFile(filepath.Join(root, "examples", "provider", "provider.tf"))
	if err != nil {
		t.Fatal(err)
	}
	providerConfig = []byte(strings.Replace(string(providerConfig), `provider "hexcore" {}`, fmt.Sprintf("provider \"hexcore\" {\n  endpoint = %q\n}", "ENDPOINT"), 1))
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom=%t", custom), func(t *testing.T) {
			b := newStub(t)
			b.immediateDestroy = true
			work := t.TempDir()
			cfg := strings.Replace(string(providerConfig), "ENDPOINT", b.server.URL, 1)
			if err := os.WriteFile(filepath.Join(work, "provider.tf"), []byte(cfg), 0600); err != nil {
				t.Fatal(err)
			}
			name := "hexcore_vds"
			if custom {
				name = "hexcore_vds_custom"
			}
			example, err := os.ReadFile(filepath.Join(root, "examples", "resources", name, "resource.tf"))
			if err != nil {
				t.Fatal(err)
			}
			resourceText := strings.ReplaceAll(strings.ReplaceAll(string(example), "example-region", "test-region"), "example-size", "small")
			if err := os.WriteFile(filepath.Join(work, "resource.tf"), []byte(resourceText), 0600); err != nil {
				t.Fatal(err)
			}
			run := func(want int, args ...string) []byte {
				t.Helper()
				cmd := exec.Command(terraformPath, args...)
				cmd.Dir = work
				cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+cliConfig, "HEXCORE_TOKEN=offline-only", "TF_VAR_creation_password=synthetic-cli-password", "TF_INPUT=0", "CHECKPOINT_DISABLE=1")
				out, err := cmd.CombinedOutput()
				code := 0
				if err != nil {
					if e, ok := err.(*exec.ExitError); ok {
						code = e.ExitCode()
					} else {
						t.Fatal(err)
					}
				}
				if code != want {
					t.Fatalf("terraform %s returned %d: %s", args[0], code, out)
				}
				return out
			}
			schema := run(0, "providers", "schema", "-json")
			if !strings.Contains(string(schema), `"write_only":true`) {
				t.Fatal("local provider schema not discovered")
			}
			run(0, "validate", "-no-color")
			run(0, "apply", "-auto-approve", "-no-color")
			run(0, "plan", "-detailed-exitcode", "-no-color")
			var state struct {
				Values struct {
					RootModule struct {
						Resources []struct{ Values map[string]any }
					} `json:"root_module"`
				}
			}
			if json.Unmarshal(run(0, "show", "-json"), &state) != nil || len(state.Values.RootModule.Resources) != 1 {
				t.Fatal("cannot obtain created ID")
			}
			id := state.Values.RootModule.Resources[0].Values["id"].(string)
			metadata := map[string]any{"id": id, "region": "test-region", "image": "debian-12"}
			if custom {
				metadata["custom_size"] = map[string]int{"vcpu": 1, "memory_gb": 1, "disk_gb": 10}
				metadata["password_wo_version"] = 1
			} else {
				metadata["size"] = "small"
				metadata["timeouts"] = map[string]string{"create": "15m", "read": "2m", "delete": "20m"}
			}
			identifier, _ := json.Marshal(metadata)
			run(0, "state", "rm", "hexcore_vds.example")
			run(0, "import", "-no-color", "hexcore_vds.example", string(identifier))
			run(0, "plan", "-detailed-exitcode", "-no-color")
			run(0, "destroy", "-auto-approve", "-no-color")
			if err := assertCleanStub(b)(nil); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Validate every read-only example against the exact same compiled schema.
	entries, err := os.ReadDir(filepath.Join(root, "examples", "data-sources"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			work := t.TempDir()
			example, err := os.ReadFile(filepath.Join(root, "examples", "data-sources", entry.Name(), "data-source.tf"))
			if err != nil {
				t.Fatal(err)
			}
			config := strings.Replace(string(providerConfig), "ENDPOINT", "http://127.0.0.1:1", 1) + string(example)
			if err := os.WriteFile(filepath.Join(work, "main.tf"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(terraformPath, "validate", "-no-color")
			cmd.Dir = work
			cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+cliConfig, "HEXCORE_TOKEN=offline-only")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("validate: %s", out)
			}
		})
	}
}
