#!/usr/bin/env bash
# Set up a QEMU/KVM test environment for PCIe topology analyzer
set -euo pipefail

# Configuration
readonly VM_NAME="pcie-topology-test"
readonly VM_MEMORY="8G"
readonly VM_CPUS="4"
readonly VM_DISK_SIZE="20G"
readonly VM_IMAGE="ubuntu-22.04-server-cloudimg-amd64.img"
readonly CLOUD_INIT="cloud-init.img"
readonly UBUNTU_URL="https://cloud-images.ubuntu.com/releases/22.04/release/${VM_IMAGE}"

# Create working directory
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
}

check_requirements() {
    local required_tools=(
        qemu-system-x86_64
        qemu-img
        cloud-localds
        wget
        virsh
    )

    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            log "Missing required tool: $tool"
            log "Please install required packages:"
            log "sudo apt-get install qemu-kvm qemu-utils cloud-image-utils wget libvirt-clients"
            exit 1
        fi
    done
}

download_cloud_image() {
    local image_path="$WORK_DIR/$VM_IMAGE"
    
    if [[ ! -f "$image_path" ]]; then
        log "Downloading Ubuntu cloud image..."
        wget -O "$image_path" "$UBUNTU_URL"
    fi
    
    # Create a copy for modification
    qemu-img convert -O qcow2 "$image_path" "$WORK_DIR/vm-disk.qcow2"
    qemu-img resize "$WORK_DIR/vm-disk.qcow2" "$VM_DISK_SIZE"
}

create_cloud_init() {
    cat > "$WORK_DIR/cloud-init.yml" << 'EOF'
#cloud-config
hostname: pcie-topology-test
users:
  - name: test
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC... # Replace with your public key
package_update: true
package_upgrade: true
packages:
  - pciutils
  - hwloc
  - numactl
  - libvirt-clients
  - git
  - make
  - gcc
write_files:
  - path: /etc/systemd/network/10-eth0.network
    content: |
      [Match]
      Name=eth0
      [Network]
      DHCP=yes
runcmd:
  - systemctl enable systemd-networkd
  - systemctl start systemd-networkd
EOF

    cloud-localds "$WORK_DIR/$CLOUD_INIT" "$WORK_DIR/cloud-init.yml"
}

create_virtual_pcie_devices() {
    # Create virtual PCIe devices using QEMU's device emulation
    cat > "$WORK_DIR/pcie-devices.xml" << 'EOF'
<domain type='kvm' xmlns:qemu='http://libvirt.org/schemas/domain/qemu/1.0'>
  <name>pcie-topology-test</name>
  <memory unit='GiB'>8</memory>
  <vcpu placement='static'>4</vcpu>
  <cpu mode='host-passthrough'>
    <topology sockets='2' cores='2' threads='1'/>
    <numa>
      <cell id='0' cpus='0-1' memory='4' unit='GiB'/>
      <cell id='1' cpus='2-3' memory='4' unit='GiB'/>
    </numa>
  </cpu>
  <numatune>
    <memory mode='strict' nodeset='0-1'/>
  </numatune>
  <os>
    <type arch='x86_64' machine='q35'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <devices>
    <emulator>/usr/bin/qemu-system-x86_64</emulator>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='vm-disk.qcow2'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='cdrom'>
      <driver name='qemu' type='raw'/>
      <source file='cloud-init.img'/>
      <target dev='sda' bus='sata'/>
      <readonly/>
    </disk>
    <interface type='network'>
      <source network='default'/>
      <model type='virtio'/>
    </interface>
    <console type='pty'>
      <target type='serial'/>
    </console>
    <!-- Virtual PCIe devices for testing -->
    <hostdev mode='subsystem' type='pci' managed='yes'>
      <driver name='vfio'/>
      <source>
        <address domain='0x0000' bus='0x01' slot='0x00' function='0x0'/>
      </source>
    </hostdev>
  </devices>
  <qemu:commandline>
    <qemu:arg value='-device'/>
    <qemu:arg value='pcie-root-port,port=0x10,chassis=1,id=pci.1,bus=pcie.0,multifunction=on,addr=0x1'/>
    <qemu:arg value='-device'/>
    <qemu:arg value='vfio-pci-dummy,host=01:00.0'/>
  </qemu:commandline>
</domain>
EOF
}

start_test_vm() {
    log "Starting test VM..."
    
    # Define the VM
    virsh define "$WORK_DIR/pcie-devices.xml"
    
    # Start the VM
    virsh start "$VM_NAME"
    
    # Wait for VM to boot
    log "Waiting for VM to boot..."
    sleep 30
    
    # Get VM IP
    local vm_ip
    vm_ip=$(virsh domifaddr "$VM_NAME" | grep -oE "\b([0-9]{1,3}\.){3}[0-9]{1,3}\b" || true)
    
    if [[ -n "$vm_ip" ]]; then
        log "VM IP address: $vm_ip"
    else
        log "Could not determine VM IP address"
    fi
}

main() {
    check_requirements
    
    log "Setting up test environment..."
    download_cloud_image
    create_cloud_init
    create_virtual_pcie_devices
    start_test_vm
    
    log "Test environment setup complete!"
    log "You can connect to the VM using: ssh test@<vm-ip>"
    log "The PCIe topology analyzer can be tested inside the VM"
}

main "$@"

