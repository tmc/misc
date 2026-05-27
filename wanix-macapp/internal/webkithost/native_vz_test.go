package webkithost

import (
	"strings"
	"testing"

	"github.com/tmc/apple/virtualization"
	"github.com/tmc/misc/wanix-macapp/internal/applefs"
)

func TestValidateVZConfigRejectsEmpty(t *testing.T) {
	report := validateVZConfig(vzConfig{})
	if report.Valid {
		t.Fatal("empty config is valid")
	}
	if len(report.Errors) == 0 {
		t.Fatal("empty config produced no errors")
	}
}

func TestVZStatus(t *testing.T) {
	report := vzReport{Supported: true, MinCPUs: 1, MaxCPUs: 8, MinMemoryMiB: 512, MaxMemoryMiB: 65536}
	got := vzStatus(report)
	for _, want := range []string{"api vz\n", "status supported\n", "min-cpus 1\n", "max-memory-mib 65536\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status = %q, missing %q", got, want)
		}
	}
}

func TestBuildVZSessionRequiresEFIDisk(t *testing.T) {
	_, report, err := buildVZSession("1", vzConfig{
		CPUs:      reportSafeCPU(),
		MemoryMiB: reportSafeMemoryMiB(),
		BootMode:  "linux",
	})
	if err == nil {
		t.Fatal("buildVZSession succeeded without efi disk")
	}
	text := strings.Join(report.Errors, "\n")
	for _, want := range []string{"only efi bootMode can start", "disk required"} {
		if !strings.Contains(text, want) {
			t.Fatalf("errors = %q, missing %q", text, want)
		}
	}
}

func TestReadVZConfigUsesSessionFiles(t *testing.T) {
	h := &Host{appleFS: applefs.NewRoot()}
	id := strings.TrimSpace(readAppleFSString(h, "vz/clone"))
	if err := h.appleFS.WriteFile("vz/"+id+"/config", []byte(`{"cpus":2,"memoryMiB":1024,"bootMode":"efi"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := h.appleFS.WriteFile("vz/"+id+"/disk", []byte("/tmp/disk.img\n")); err != nil {
		t.Fatal(err)
	}
	if err := h.appleFS.WriteFile("vz/"+id+"/net", []byte("nat\n")); err != nil {
		t.Fatal(err)
	}
	cfg, err := h.readVZConfig(id)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Disk != "/tmp/disk.img" || !cfg.Network {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestVZStateText(t *testing.T) {
	if got := vzStateText(virtualization.VZVirtualMachineStateRunning); got != "running" {
		t.Fatalf("running state = %q", got)
	}
}

func reportSafeCPU() uint {
	return virtualization.GetVZVirtualMachineConfigurationClass().MinimumAllowedCPUCount()
}

func reportSafeMemoryMiB() uint64 {
	return virtualization.GetVZVirtualMachineConfigurationClass().MinimumAllowedMemorySize() / (1024 * 1024)
}
