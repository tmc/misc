package webkithost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmc/apple/dispatch"
	"github.com/tmc/apple/foundation"
	"github.com/tmc/apple/virtualization"
)

type vzConfig struct {
	CPUs         uint   `json:"cpus"`
	MemoryMiB    uint64 `json:"memoryMiB"`
	BootMode     string `json:"bootMode"`
	Disk         string `json:"disk,omitempty"`
	EFIVars      string `json:"efiVars,omitempty"`
	SerialLog    string `json:"serialLog,omitempty"`
	Network      bool   `json:"network,omitempty"`
	ReadOnly     bool   `json:"readOnly,omitempty"`
	AppendSerial bool   `json:"appendSerial,omitempty"`
}

type vzReport struct {
	Supported    bool     `json:"supported"`
	Valid        bool     `json:"valid"`
	CPUs         uint     `json:"cpus,omitempty"`
	MemoryMiB    uint64   `json:"memoryMiB,omitempty"`
	MinCPUs      uint     `json:"min_cpus"`
	MaxCPUs      uint     `json:"max_cpus"`
	MinMemoryMiB uint64   `json:"min_memory_mib"`
	MaxMemoryMiB uint64   `json:"max_memory_mib"`
	BootMode     string   `json:"bootMode,omitempty"`
	Disk         string   `json:"disk,omitempty"`
	EFIVars      string   `json:"efiVars,omitempty"`
	SerialLog    string   `json:"serialLog,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}

type vzSession struct {
	vm    virtualization.VZVirtualMachine
	queue dispatch.Queue
}

func (h *Host) applyVZSession(name, verb string) error {
	id := strings.Split(name, "/")[1]
	switch verb {
	case "validate":
		return h.validateVZConfig(id)
	case "start":
		return h.startVZ(id)
	case "pause":
		return h.pauseVZ(id)
	case "resume":
		return h.resumeVZ(id)
	case "stop":
		return h.stopVZ(id, false)
	case "kill":
		return h.stopVZ(id, true)
	case "destroy":
		h.vzMu.Lock()
		delete(h.vzVMs, id)
		h.vzMu.Unlock()
		return h.appleFS.WriteFile("vz/"+id+"/status", []byte("status destroyed\n"))
	default:
		return nil
	}
}

func (h *Host) validateVZConfig(id string) error {
	var cfg vzConfig
	if err := json.Unmarshal([]byte(readAppleFSString(h, "vz/"+id+"/config")), &cfg); err != nil {
		_ = h.appleFS.WriteFile("vz/"+id+"/status", []byte("status error\nerror parse config: "+err.Error()+"\n"))
		return fmt.Errorf("parse config: %w", err)
	}
	report := validateVZConfig(cfg)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := h.appleFS.WriteFile("vz/"+id+"/report", data); err != nil {
		return err
	}
	if err := h.appleFS.WriteFile("vz/status", []byte(vzStatus(report))); err != nil {
		return err
	}
	state := "valid"
	if !report.Valid {
		state = "invalid"
	}
	return h.appleFS.WriteFile("vz/"+id+"/status", []byte("status "+state+"\n"))
}

func validateVZConfig(cfg vzConfig) vzReport {
	vmClass := virtualization.GetVZVirtualMachineClass()
	cfgClass := virtualization.GetVZVirtualMachineConfigurationClass()
	report := vzReport{
		Supported:    vmClass.IsSupported(),
		CPUs:         cfg.CPUs,
		MemoryMiB:    cfg.MemoryMiB,
		MinCPUs:      cfgClass.MinimumAllowedCPUCount(),
		MaxCPUs:      cfgClass.MaximumAllowedCPUCount(),
		MinMemoryMiB: cfgClass.MinimumAllowedMemorySize() / (1024 * 1024),
		MaxMemoryMiB: cfgClass.MaximumAllowedMemorySize() / (1024 * 1024),
		BootMode:     cfg.BootMode,
		Disk:         cfg.Disk,
		EFIVars:      cfg.EFIVars,
		SerialLog:    cfg.SerialLog,
	}
	if !report.Supported {
		report.Errors = append(report.Errors, "virtualization not supported")
	}
	if cfg.CPUs == 0 {
		report.Errors = append(report.Errors, "cpus required")
	} else if cfg.CPUs < report.MinCPUs || cfg.CPUs > report.MaxCPUs {
		report.Errors = append(report.Errors, fmt.Sprintf("cpus out of range %d-%d", report.MinCPUs, report.MaxCPUs))
	}
	if cfg.MemoryMiB == 0 {
		report.Errors = append(report.Errors, "memoryMiB required")
	} else if cfg.MemoryMiB < report.MinMemoryMiB || cfg.MemoryMiB > report.MaxMemoryMiB {
		report.Errors = append(report.Errors, fmt.Sprintf("memoryMiB out of range %d-%d", report.MinMemoryMiB, report.MaxMemoryMiB))
	}
	switch cfg.BootMode {
	case "", "linux", "efi", "mac":
	default:
		report.Errors = append(report.Errors, fmt.Sprintf("unknown bootMode %q", cfg.BootMode))
	}
	report.Valid = len(report.Errors) == 0
	return report
}

func (h *Host) startVZ(id string) error {
	cfg, err := h.readVZConfig(id)
	if err != nil {
		_ = h.appleFS.WriteFile("vz/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	session, report, err := buildVZSession(id, cfg)
	if err != nil {
		_ = h.writeVZReport(id, report)
		_ = h.appleFS.WriteFile("vz/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.writeVZReport(id, report); err != nil {
		return err
	}

	h.vzMu.Lock()
	h.vzVMs[id] = session
	h.vzMu.Unlock()

	if err := h.appleFS.WriteFile("vz/"+id+"/status", []byte("status starting\n")); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := vzOnQueue(ctx, session, func(vm virtualization.VZVirtualMachine) error {
		if !vm.CanStart() {
			return fmt.Errorf("vz cannot start from state %s", vzStateText(vm.State()))
		}
		return vm.Start(ctx)
	}); err != nil {
		_ = h.appleFS.WriteFile("vz/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	return h.writeVZState(id, session)
}

func (h *Host) pauseVZ(id string) error {
	return h.applyVZVMAction(id, "pausing", func(vm virtualization.VZVirtualMachine, done chan error) {
		if !vm.CanPause() {
			done <- fmt.Errorf("vz cannot pause from state %s", vzStateText(vm.State()))
			return
		}
		vm.PauseWithCompletionHandler(func(err error) { done <- err })
	})
}

func (h *Host) resumeVZ(id string) error {
	return h.applyVZVMAction(id, "resuming", func(vm virtualization.VZVirtualMachine, done chan error) {
		if !vm.CanResume() {
			done <- fmt.Errorf("vz cannot resume from state %s", vzStateText(vm.State()))
			return
		}
		vm.ResumeWithCompletionHandler(func(err error) { done <- err })
	})
}

func (h *Host) stopVZ(id string, force bool) error {
	if force {
		return h.applyVZVMAction(id, "stopping", func(vm virtualization.VZVirtualMachine, done chan error) {
			if !vm.CanStop() {
				done <- fmt.Errorf("vz cannot stop from state %s", vzStateText(vm.State()))
				return
			}
			vm.StopWithCompletionHandler(func(err error) { done <- err })
		})
	}
	return h.applyVZVMAction(id, "stopping", func(vm virtualization.VZVirtualMachine, done chan error) {
		if !vm.CanRequestStop() {
			done <- fmt.Errorf("vz cannot request stop from state %s", vzStateText(vm.State()))
			return
		}
		_, err := vm.RequestStopWithError()
		done <- err
	})
}

func (h *Host) applyVZVMAction(id, pending string, action func(virtualization.VZVirtualMachine, chan error)) error {
	session, ok := h.vzSession(id)
	if !ok {
		err := fmt.Errorf("vz session not started")
		_ = h.appleFS.WriteFile("vz/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	if err := h.appleFS.WriteFile("vz/"+id+"/status", []byte("status "+pending+"\n")); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := vzOnQueue(ctx, session, func(vm virtualization.VZVirtualMachine) error {
		done := make(chan error, 1)
		action(vm, done)
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		_ = h.appleFS.WriteFile("vz/"+id+"/status", []byte("status error\nerror "+err.Error()+"\n"))
		return err
	}
	return h.writeVZState(id, session)
}

func (h *Host) vzSession(id string) (vzSession, bool) {
	h.vzMu.Lock()
	defer h.vzMu.Unlock()
	session, ok := h.vzVMs[id]
	return session, ok
}

func (h *Host) writeVZState(id string, session vzSession) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var state virtualization.VZVirtualMachineState
	if err := vzOnQueue(ctx, session, func(vm virtualization.VZVirtualMachine) error {
		state = vm.State()
		return nil
	}); err != nil {
		return err
	}
	return h.appleFS.WriteFile("vz/"+id+"/status", []byte("status "+vzStateText(state)+"\n"))
}

func (h *Host) readVZConfig(id string) (vzConfig, error) {
	var cfg vzConfig
	if err := json.Unmarshal([]byte(readAppleFSString(h, "vz/"+id+"/config")), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Disk == "" {
		cfg.Disk = strings.TrimSpace(readAppleFSString(h, "vz/"+id+"/disk"))
	}
	if cfg.Network || strings.Contains(readAppleFSString(h, "vz/"+id+"/net"), "nat") {
		cfg.Network = true
	}
	return cfg, nil
}

func (h *Host) writeVZReport(id string, report vzReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := h.appleFS.WriteFile("vz/"+id+"/report", data); err != nil {
		return err
	}
	return h.appleFS.WriteFile("vz/status", []byte(vzStatus(report)))
}

func buildVZSession(id string, cfg vzConfig) (vzSession, vzReport, error) {
	report := validateVZConfig(cfg)
	if cfg.BootMode == "" {
		cfg.BootMode = "efi"
		report.BootMode = cfg.BootMode
	}
	if cfg.BootMode != "efi" {
		report.Errors = append(report.Errors, "only efi bootMode can start")
	}
	if cfg.Disk == "" {
		report.Errors = append(report.Errors, "disk required")
	}
	if cfg.EFIVars == "" {
		cfg.EFIVars = filepath.Join(os.TempDir(), "wanix-vz-"+id, "efi.vars")
		report.EFIVars = cfg.EFIVars
	}
	if cfg.SerialLog == "" {
		cfg.SerialLog = filepath.Join(os.TempDir(), "wanix-vz-"+id, "serial.log")
		report.SerialLog = cfg.SerialLog
	}
	if len(report.Errors) > 0 {
		report.Valid = false
		return vzSession{}, report, errors.New(strings.Join(report.Errors, "; "))
	}

	if err := os.MkdirAll(filepath.Dir(cfg.EFIVars), 0755); err != nil {
		return vzSession{}, report, fmt.Errorf("efi vars dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.SerialLog), 0755); err != nil {
		return vzSession{}, report, fmt.Errorf("serial log dir: %w", err)
	}
	f, err := os.OpenFile(cfg.SerialLog, os.O_CREATE|os.O_WRONLY|serialOpenFlag(cfg.AppendSerial), 0644)
	if err != nil {
		return vzSession{}, report, fmt.Errorf("serial log: %w", err)
	}
	_ = f.Close()

	efiStore, err := vzEFIVariableStore(cfg.EFIVars)
	if err != nil {
		return vzSession{}, report, fmt.Errorf("efi variable store: %w", err)
	}
	boot := virtualization.NewVZEFIBootLoader()
	boot.SetVariableStore(efiStore)

	platform := virtualization.NewVZGenericPlatformConfiguration()
	platform.SetMachineIdentifier(virtualization.NewVZGenericMachineIdentifier())

	diskURL := foundation.NewURLFileURLWithPath(cfg.Disk)
	attachment, err := virtualization.NewDiskImageStorageDeviceAttachmentWithURLReadOnlyCachingModeSynchronizationModeError(
		diskURL,
		cfg.ReadOnly,
		virtualization.VZDiskImageCachingModeAutomatic,
		virtualization.VZDiskImageSynchronizationModeFsync,
	)
	if err != nil {
		return vzSession{}, report, fmt.Errorf("disk attachment: %w", err)
	}
	block := virtualization.NewVirtioBlockDeviceConfigurationWithAttachment(attachment)
	block.SetBlockDeviceIdentifier("wanix")

	serialURL := foundation.NewURLFileURLWithPath(cfg.SerialLog)
	serialAttachment, err := virtualization.NewFileSerialPortAttachmentWithURLAppendError(serialURL, cfg.AppendSerial)
	if err != nil {
		return vzSession{}, report, fmt.Errorf("serial attachment: %w", err)
	}
	serial := virtualization.NewVZVirtioConsoleDeviceSerialPortConfiguration()
	serial.SetAttachment(serialAttachment)

	vmcfg := virtualization.NewVZVirtualMachineConfiguration()
	vmcfg.SetPlatform(platform)
	vmcfg.SetBootLoader(boot)
	vmcfg.SetCPUCount(cfg.CPUs)
	vmcfg.SetMemorySize(cfg.MemoryMiB * 1024 * 1024)
	vmcfg.SetStorageDevices([]virtualization.VZStorageDeviceConfiguration{block.VZStorageDeviceConfiguration})
	vmcfg.SetSerialPorts([]virtualization.VZSerialPortConfiguration{serial.VZSerialPortConfiguration})
	vmcfg.SetEntropyDevices([]virtualization.VZEntropyDeviceConfiguration{
		virtualization.NewVZVirtioEntropyDeviceConfiguration().VZEntropyDeviceConfiguration,
	})
	if cfg.Network {
		netdev := virtualization.NewVZVirtioNetworkDeviceConfiguration()
		netdev.SetAttachment(virtualization.NewVZNATNetworkDeviceAttachment())
		vmcfg.SetNetworkDevices([]virtualization.VZNetworkDeviceConfiguration{netdev.VZNetworkDeviceConfiguration})
	}
	if ok, err := vmcfg.ValidateWithError(); err != nil {
		report.Valid = false
		report.Errors = append(report.Errors, err.Error())
		return vzSession{}, report, fmt.Errorf("validate VZ configuration: %w", err)
	} else if !ok {
		report.Valid = false
		report.Errors = append(report.Errors, "validateWithError returned false")
		return vzSession{}, report, errors.New("validate VZ configuration: returned false")
	}

	queue := dispatch.QueueCreate("github.com.tmc.wanix.vz." + id)
	report.Valid = true
	return vzSession{vm: virtualization.NewVirtualMachineWithConfigurationQueue(vmcfg, queue), queue: queue}, report, nil
}

func vzOnQueue(ctx context.Context, session vzSession, f func(virtualization.VZVirtualMachine) error) error {
	done := make(chan error, 1)
	session.queue.Async(func() {
		done <- f(session.vm)
	})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func vzEFIVariableStore(name string) (virtualization.VZEFIVariableStore, error) {
	url := foundation.NewURLFileURLWithPath(name)
	if _, err := os.Stat(name); err == nil {
		return virtualization.NewEFIVariableStoreWithURL(url), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return virtualization.VZEFIVariableStore{}, err
	}
	return virtualization.NewEFIVariableStoreCreatingVariableStoreAtURLOptionsError(url, 0)
}

func serialOpenFlag(appendSerial bool) int {
	if appendSerial {
		return os.O_APPEND
	}
	return os.O_TRUNC
}

func vzStateText(state virtualization.VZVirtualMachineState) string {
	switch state {
	case virtualization.VZVirtualMachineStateStopped:
		return "stopped"
	case virtualization.VZVirtualMachineStateRunning:
		return "running"
	case virtualization.VZVirtualMachineStatePaused:
		return "paused"
	case virtualization.VZVirtualMachineStateError:
		return "error"
	case virtualization.VZVirtualMachineStateStarting:
		return "starting"
	case virtualization.VZVirtualMachineStatePausing:
		return "pausing"
	case virtualization.VZVirtualMachineStateResuming:
		return "resuming"
	case virtualization.VZVirtualMachineStateStopping:
		return "stopping"
	default:
		return fmt.Sprintf("unknown-%d", state)
	}
}

func vzStatus(report vzReport) string {
	status := "unsupported"
	if report.Supported {
		status = "supported"
	}
	return fmt.Sprintf("api vz\nstatus %s\nmin-cpus %d\nmax-cpus %d\nmin-memory-mib %d\nmax-memory-mib %d\n",
		status, report.MinCPUs, report.MaxCPUs, report.MinMemoryMiB, report.MaxMemoryMiB)
}
