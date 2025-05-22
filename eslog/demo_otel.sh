#!/bin/bash
# Demonstration script for eslog-to-otel

echo "=== ESLog to OpenTelemetry Exporter Demo ==="
echo ""

# Generate some sample data by capturing real events
echo "Generating sample data..."
cat > test_processes.json << EOF
{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exec":{"args":["/bin/ls","-la"],"cwd":{"path":"/tmp"},"dyld_exec_path":"","env":["PATH=/usr/bin:/bin"],"fds":[],"image_cputype":0,"image_cpusubtype":0,"last_fd":3,"script":null,"target":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12345,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/bin/ls"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":12344,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"ppid":12344,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.ls","start_time":"2025-01-20T10:00:00Z","team_id":null,"tty":null}}},"event_type":1,"global_seq_num":1,"mach_time":12345678900,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/bin/bash"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":1,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"ppid":1,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.bash","start_time":"2025-01-20T09:00:00Z","team_id":null,"tty":null},"schema_version":1,"seq_num":1,"thread":{"thread_id":123},"time":"2025-01-20T10:00:00Z","version":1}
{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"open":{"file":{"path":"/tmp/test.txt"},"mode":2}},"event_type":8,"global_seq_num":2,"mach_time":12345678901,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12345,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/bin/ls"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":12344,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"ppid":12344,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.ls","start_time":"2025-01-20T10:00:00Z","team_id":null,"tty":null},"schema_version":1,"seq_num":2,"thread":{"thread_id":123},"time":"2025-01-20T10:00:01Z","version":1}
{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"stat":{"source":{"path":"/tmp/test.txt"}}},"event_type":5,"global_seq_num":3,"mach_time":12345678902,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12345,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/bin/ls"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":12344,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"ppid":12344,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.ls","start_time":"2025-01-20T10:00:00Z","team_id":null,"tty":null},"schema_version":1,"seq_num":3,"thread":{"thread_id":123},"time":"2025-01-20T10:00:01Z","version":1}
{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"read":{"fd":{"fd":3,"fdtype":0},"size":1024}},"event_type":10,"global_seq_num":4,"mach_time":12345678903,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12345,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/bin/ls"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":12344,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"ppid":12344,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.ls","start_time":"2025-01-20T10:00:00Z","team_id":null,"tty":null},"schema_version":1,"seq_num":4,"thread":{"thread_id":123},"time":"2025-01-20T10:00:02Z","version":1}
{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exit":{"exit_code":0,"reason":"normal"}},"event_type":17,"global_seq_num":5,"mach_time":12345678904,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12345,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/bin/ls"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":12344,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"ppid":12344,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.ls","start_time":"2025-01-20T10:00:00Z","team_id":null,"tty":null},"schema_version":1,"seq_num":5,"thread":{"thread_id":123},"time":"2025-01-20T10:00:03Z","version":1}
EOF

echo ""
echo "1. Testing stdout export with traces and metrics:"
echo "================================================"
./tools/eslog-to-otel/eslog-to-otel \
    -exporter stdout \
    -service "demo-app" \
    -use-metrics=true \
    -temporality=delta \
    -verbose < test_processes.json | head -50

echo ""
echo "2. Testing filtering by process name:"
echo "===================================="
./tools/eslog-to-otel/eslog-to-otel \
    -exporter stdout \
    -name "ls" \
    -use-metrics=true \
    -temporality=cumulative \
    -metrics-export=3s < test_processes.json | head -30

echo ""
echo "3. Testing with root span disabled:"
echo "==================================="
./tools/eslog-to-otel/eslog-to-otel \
    -exporter stdout \
    -create-root-span=false \
    -use-metrics=true < test_processes.json | head -30

echo ""
echo "4. Demonstrating W3C trace context:"
echo "==================================="
echo "When a process has TRACEPARENT environment variable, it will be linked"
# Add example with TRACEPARENT
cat > test_trace_context.json << EOF
{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exec":{"args":["/usr/bin/curl","https://api.example.com"],"cwd":{"path":"/home/user"},"dyld_exec_path":"","env":["PATH=/usr/bin:/bin","TRACEPARENT=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"],"fds":[],"image_cputype":0,"image_cpusubtype":0,"last_fd":3,"script":null,"target":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":54321,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/usr/bin/curl"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":12344,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"ppid":12344,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.curl","start_time":"2025-01-20T10:00:00Z","team_id":null,"tty":null}}},"event_type":1,"global_seq_num":1,"mach_time":12345678900,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12344,"pidversion":1,"rgid":0,"ruid":0},"cdhash":"","codesigning_flags":0,"executable":{"path":"/bin/bash"},"group_id":0,"is_es_client":false,"is_platform_binary":true,"original_ppid":1,"parent_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"ppid":1,"responsible_audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":1,"pidversion":1,"rgid":0,"ruid":0},"session_id":0,"signing_id":"com.apple.bash","start_time":"2025-01-20T09:00:00Z","team_id":null,"tty":null},"schema_version":1,"seq_num":1,"thread":{"thread_id":123},"time":"2025-01-20T10:00:00Z","version":1}
EOF

./tools/eslog-to-otel/eslog-to-otel \
    -exporter stdout \
    -verbose < test_trace_context.json | grep -E "(traceparent|linked)"

echo ""
echo "5. If Jaeger is running, export to it:"
echo "====================================="
echo "Run: ./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint localhost:4317 < test_processes.json"
echo "Then view traces at: http://localhost:16686"
echo ""

# Cleanup
rm -f test_processes.json test_trace_context.json

echo "Demo complete!"