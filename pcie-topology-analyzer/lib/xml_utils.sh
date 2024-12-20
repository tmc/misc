#!/usr/bin/env bash
# XML generation utility functions

# Generate CPU topology XML
generate_cpu_topology_xml() {
    local sockets="$1"
    local cores_per_socket="$2"
    local threads_per_core="$3"
    local output_file="$4"
    
    cat >> "$output_file" << EOF
  <cpu mode='host-passthrough'>
    <topology sockets='$sockets' cores='$cores_per_socket' threads='$threads_per_core'/>
EOF
}

# Generate NUMA cell XML
generate_numa_cell_xml() {
    local node_id="$1"
    local cpus="$2"
    local memory="$3"
    local output_file="$4"
    
    cat >> "$output_file" << EOF
    <numa>
      <cell id='$node_id' cpus='$cpus' memory='$memory' unit='GiB'/>
    </numa>
EOF
}

# Generate device XML with NUMA affinity
generate_device_xml() {
    local device_type="$1"  # network, disk, etc.
    local device_config="$2" # Additional device-specific config
    local numa_node="$3"
    local output_file="$4"
    
    case "$device_type" in
        network)
            cat >> "$output_file" << EOF
    <interface type='network'>
      <source network='default'/>
      <model type='virtio'/>
      <driver name='vhost' queues='4'/>
      <numa node='$numa_node'/>
      $device_config
    </interface>
EOF
            ;;
        disk)
            cat >> "$output_file" << EOF
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw' cache='none' io='native'/>
      <source file='/var/lib/libvirt/images/disk.img'/>
      <target dev='vda' bus='virtio'/>
      <numa node='$numa_node'/>
      $device_config
    </disk>
EOF
            ;;
    esac
}

