package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Code-Hex/vz/v3"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
)

const (
	defaultImage = "https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img"
	defaultCPUs  = 4
	defaultMemory = "4GiB"
	defaultDisk   = "100GiB"
)

func main() {
	if err := run(); err != nil {
		logrus.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	vmName := "default"
	vmDir := filepath.Join(os.Getenv("HOME"), ".lima", vmName)
	baseDisk := filepath.Join(vmDir, "basedisk")
	diffDisk := filepath.Join(vmDir, "diffdisk")
	serialLog := filepath.Join(vmDir, "serial.log")

	if err := os.MkdirAll(vmDir, 0o700); err != nil {
		return fmt.Errorf("failed to create instance directory: %w", err)
	}

	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		logrus.Infof("Downloading base image to %s", baseDisk)
		cmd := exec.Command("curl", "-s", "-o", baseDisk, defaultImage)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to download base image: %w, out: %s", err, out)
		}
	}

	if _, err := os.Stat(diffDisk); errors.Is(err, os.ErrNotExist) {
		diskSize, _ := units.RAMInBytes(defaultDisk)
		cmd := exec.Command("qemu-img", "create", "-f", "qcow2", "-b", baseDisk, diffDisk, strconv.Itoa(int(diskSize)))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create diff disk: %w, out: %s", err, out)
		}
	}

	vmConfig := createVMConfig()

	machine, err := vz.NewVirtualMachine(vmConfig)
	if err != nil {
		return err
	}

	err = machine.Start()
	if err != nil {
		return err
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	select {
	case newState := <-machine.StateChangedNotify():
		logrus.Infof("VM state change: %q", newState)
	case sig := <-signalCh:
		logrus.Infof("Received %s, shutting down the host agent", sig)
		if machine.CanStop() {
			_, err := machine.RequestStop()
			if err != nil {
				logrus.Errorf("Error while stopping the VM %q", err)
			}
		}
		return nil
	}
	return nil
}

func createVMConfig() *vz.VirtualMachineConfiguration {
	bootLoader, err := vz.NewEFIBootLoader()
	if err != nil {
		panic(err)
	}

	memBytes, err := units.RAMInBytes(defaultMemory)
	if err != nil {
		panic(err)
	}

	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(defaultCPUs),
		uint64(memBytes),
	)
	if err != nil {
		panic(err)
	}

	machineIdentifier, err := vz.NewGenericMachineIdentifier()
	if err != nil {
		panic(err)
	}

	platformConfig, err := vz.NewGenericPlatformConfiguration(vz.WithGenericMachineIdentifier(machineIdentifier))
	if err != nil {
		panic(err)
	}

	vmConfig.SetPlatformVirtualMachineConfiguration(platformConfig)

	serialPortAttachment, err := vz.NewFileSerialPortAttachment("serial.log", false)
	if err != nil {
		panic(err)
	}
	consoleConfig, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	vmConfig.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{consoleConfig})

	networkConfig, err := vz.NewVirtioNetworkDeviceConfiguration(vz.NewNATNetworkDeviceAttachment())
	if err != nil {
		panic(err)
	}
	mac, err := net.ParseMAC("52:55:55:00:00:05")
	if err != nil {
		panic(err)
	}
	address, err := vz.NewMACAddress(mac)
	if err != nil {
		panic(err)
	}
	networkConfig.SetMACAddress(address)
	vmConfig.SetNetworkDevicesVirtualMachineConfiguration([]vz.NetworkDeviceConfiguration{networkConfig})

	entropyConfig, err := vz.NewVirtioEntropyDeviceConfiguration()
	if err != nil {
		panic(err)
	}
	vmConfig.SetEntropyDevicesVirtualMachineConfiguration([]vz.EntropyDeviceConfiguration{
		entropyConfig,
	})

	configuration, err := vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration()
	if err != nil {
		panic(err)
	}
	vmConfig.SetMemoryBalloonDevicesVirtualMachineConfiguration([]vz.MemoryBalloonDeviceConfiguration{
		configuration,
	})

	deviceConfiguration, err := vz.NewVirtioSocketDeviceConfiguration()
	vmConfig.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{
		deviceConfiguration,
	})
	if err != nil {
		panic(err)
	}

	validated, err := vmConfig.Validate()
	if !validated || err != nil {
		panic(err)
	}
	return vmConfig
}