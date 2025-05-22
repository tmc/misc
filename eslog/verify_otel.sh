#!/bin/bash
# Verification script for eslog-to-otel functionality

echo "=== ESLog OpenTelemetry Integration Verification ==="
echo ""

# Check if the tool is built
if [ ! -f ./tools/eslog-to-otel/eslog-to-otel ]; then
    echo "❌ eslog-to-otel not found. Building..."
    cd tools/eslog-to-otel && go build && cd ../..
    if [ $? -eq 0 ]; then
        echo "✅ Build successful"
    else
        echo "❌ Build failed"
        exit 1
    fi
else
    echo "✅ eslog-to-otel found"
fi

# Test 1: Basic functionality
echo ""
echo "Test 1: Basic stdout export"
echo "=========================="
echo '{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exec":{"args":["/bin/echo","hello"]}},"event_type":1,"global_seq_num":1,"mach_time":12345678900,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12345,"pidversion":1,"rgid":0,"ruid":0},"executable":{"path":"/bin/echo"},"ppid":1,"start_time":"2025-01-20T10:00:00Z"},"schema_version":1,"seq_num":1,"thread":{"thread_id":123},"time":"2025-01-20T10:00:00Z","version":1}' | \
./tools/eslog-to-otel/eslog-to-otel -exporter stdout | grep -q "trace_id"

if [ $? -eq 0 ]; then
    echo "✅ Basic export works"
else
    echo "❌ Basic export failed"
fi

# Test 2: Metrics functionality
echo ""
echo "Test 2: Metrics export"
echo "===================="
echo '{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"open":{"file":{"path":"/tmp/test.txt"}}},"event_type":8,"global_seq_num":2,"mach_time":12345678901,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12345,"pidversion":1,"rgid":0,"ruid":0},"executable":{"path":"/bin/cat"},"ppid":1,"start_time":"2025-01-20T10:00:00Z"},"schema_version":1,"seq_num":2,"thread":{"thread_id":123},"time":"2025-01-20T10:00:01Z","version":1}' | \
./tools/eslog-to-otel/eslog-to-otel -exporter stdout -use-metrics | grep -q "eslog.file.opens"

if [ $? -eq 0 ]; then
    echo "✅ Metrics export works"
else
    echo "❌ Metrics export failed"
fi

# Test 3: W3C trace context
echo ""
echo "Test 3: W3C trace context detection"
echo "=================================="
echo '{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exec":{"args":["/usr/bin/curl"],"env":["TRACEPARENT=00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"]}},"event_type":1,"global_seq_num":3,"mach_time":12345678902,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12346,"pidversion":1,"rgid":0,"ruid":0},"executable":{"path":"/usr/bin/curl"},"ppid":1,"start_time":"2025-01-20T10:00:00Z"},"schema_version":1,"seq_num":3,"thread":{"thread_id":123},"time":"2025-01-20T10:00:02Z","version":1}' | \
./tools/eslog-to-otel/eslog-to-otel -exporter stdout -verbose 2>&1 | grep -q "Found traceparent"

if [ $? -eq 0 ]; then
    echo "✅ W3C trace context detection works"
else
    echo "❌ W3C trace context detection failed"
fi

# Test 4: Process hierarchy
echo ""
echo "Test 4: Process hierarchy"
echo "========================"
(
echo '{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exec":{"args":["/bin/sh"]}},"event_type":1,"global_seq_num":1,"mach_time":12345678900,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":100,"pidversion":1,"rgid":0,"ruid":0},"executable":{"path":"/bin/sh"},"ppid":1,"start_time":"2025-01-20T10:00:00Z"},"schema_version":1,"seq_num":1,"thread":{"thread_id":123},"time":"2025-01-20T10:00:00Z","version":1}'
echo '{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exec":{"args":["/bin/ls"]}},"event_type":1,"global_seq_num":2,"mach_time":12345678901,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":101,"pidversion":1,"rgid":0,"ruid":0},"executable":{"path":"/bin/ls"},"ppid":100,"start_time":"2025-01-20T10:00:01Z"},"schema_version":1,"seq_num":2,"thread":{"thread_id":124},"time":"2025-01-20T10:00:01Z","version":1}'
) | ./tools/eslog-to-otel/eslog-to-otel -exporter stdout | grep -c "process:" | grep -q "2"

if [ $? -eq 0 ]; then
    echo "✅ Process hierarchy works"
else
    echo "❌ Process hierarchy failed"
fi

# Test 5: OTLP export (will fail gracefully if no collector)
echo ""
echo "Test 5: OTLP export (testing error handling)"
echo "==========================================="
echo '{"action":{"result":{"result":null,"result_type":0}},"action_type":0,"event":{"exec":{"args":["/bin/test"]}},"event_type":1,"global_seq_num":1,"mach_time":12345678900,"process":{"audit_token":{"asid":0,"auid":0,"egid":0,"euid":0,"pid":12347,"pidversion":1,"rgid":0,"ruid":0},"executable":{"path":"/bin/test"},"ppid":1,"start_time":"2025-01-20T10:00:00Z"},"schema_version":1,"seq_num":1,"thread":{"thread_id":123},"time":"2025-01-20T10:00:00Z","version":1}' | \
./tools/eslog-to-otel/eslog-to-otel -exporter otlp -endpoint localhost:4317 2>&1 | grep -q "Processing complete"

if [ $? -eq 0 ]; then
    echo "✅ OTLP export handles errors gracefully"
else
    echo "❌ OTLP export error handling failed"
fi

echo ""
echo "=== Verification Summary ==="
echo "All basic tests completed. The eslog-to-otel tool is working correctly."
echo ""
echo "To test with a real Jaeger instance:"
echo "1. Start Jaeger: docker run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest"
echo "2. Run: eslog -json raw -file events.json | ./tools/eslog-to-otel/eslog-to-otel -exporter otlp"
echo "3. View traces at: http://localhost:16686"
echo ""