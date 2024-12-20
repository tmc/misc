#!/usr/bin/env bash
# Run PCIe topology analyzer tests in the test VM
set -euo pipefail

readonly VM_NAME="pcie-topology-test"

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >&2
}

wait_for_vm() {
    local timeout=300
    local start_time=$(date +%s)
    
    while true; do
        if virsh domifaddr "$VM_NAME" | grep -q "ipv4"; then
            return 0
        fi
        
        if (( $(date +%s) - start_time > timeout )); then
            log "Timeout waiting for VM to start"
            return 1
        fi
        
        sleep 5
    done
}

copy_files_to_vm() {
    local vm_ip="$1"
    
    # Copy test files to VM
    scp -r ../pcie-topology-analyzer.sh "test@$vm_ip:~/"
    scp -r ../test_suite.sh "test@$vm_ip:~/"
    scp -r ../test_helpers.sh "test@$vm_ip:~/"
}

run_tests_in_vm() {
    local vm_ip="$1"
    
    # Run tests in VM
    ssh "test@$vm_ip" bash -c '
        cd ~
        chmod +x pcie-topology-analyzer.sh test_suite.sh
        sudo ./test_suite.sh
    '
}

cleanup_vm() {
    log "Cleaning up test VM..."
    virsh destroy "$VM_NAME" || true
    virsh undefine "$VM_NAME" --remove-all-storage || true
}

main() {
    log "Starting test execution..."
    
    # Wait for VM to be ready
    wait_for_vm
    
    # Get VM IP
    local vm_ip
    vm_ip=$(virsh domifaddr "$VM_NAME" | grep -oE "\b([0-9]{1,3}\.){3}[0-9]{1,3}\b")
    
    # Copy files and run tests
    copy_files_to_vm "$vm_ip"
    run_tests_in_vm "$vm_ip"
    
    log "Tests completed successfully!"
}

trap cleanup_vm EXIT
main "$@"

