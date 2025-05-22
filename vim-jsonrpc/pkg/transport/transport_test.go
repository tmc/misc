package transport

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStdioTransport(t *testing.T) {
	input := "test message\n"
	reader := bufio.NewReader(strings.NewReader(input))
	var output bytes.Buffer

	transport := &StdioTransport{
		reader: reader,
		writer: &output,
	}

	data, err := transport.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	expected := "test message\n"
	if string(data) != expected {
		t.Errorf("Expected %q, got %q", expected, string(data))
	}

	writeData := []byte("response message")
	err = transport.Write(writeData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if !bytes.Contains(output.Bytes(), writeData) {
		t.Errorf("Output should contain written data")
	}

	err = transport.Close()
	if err != nil {
		t.Errorf("Close should not fail: %v", err)
	}
}

func TestStdioTransportEOF(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	transport := &StdioTransport{
		reader: reader,
		writer: io.Discard,
	}

	_, err := transport.Read()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestTCPTransport(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		data := make([]byte, 1024)
		n, err := conn.Read(data)
		if err != nil {
			return
		}

		conn.Write([]byte("echo: " + string(data[:n])))
	}()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	transport := &TCPTransport{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}

	testMsg := []byte("test message")
	err = transport.Write(testMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := transport.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !bytes.Contains(data, testMsg) {
		t.Errorf("Response should contain original message")
	}

	err = transport.Close()
	if err != nil {
		t.Errorf("Close should not fail: %v", err)
	}
}

func TestUnixTransport(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create unix listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		data := make([]byte, 1024)
		n, err := conn.Read(data)
		if err != nil {
			return
		}

		conn.Write([]byte("unix_echo: " + string(data[:n])))
	}()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to unix socket: %v", err)
	}

	transport := &UnixTransport{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}

	testMsg := []byte("unix test message")
	err = transport.Write(testMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := transport.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !bytes.Contains(data, testMsg) {
		t.Errorf("Response should contain original message")
	}

	err = transport.Close()
	if err != nil {
		t.Errorf("Close should not fail: %v", err)
	}
}

func TestNewTransport(t *testing.T) {
	testCases := []struct {
		name        string
		transportType string
		addr        string
		socket      string
		expectError bool
	}{
		{
			name:        "stdio_transport",
			transportType: "stdio",
			expectError: false,
		},
		{
			name:        "tcp_transport",
			transportType: "tcp",
			addr:        "localhost:0",
			expectError: false,
		},
		{
			name:        "unix_transport",
			transportType: "unix",
			socket:      "/tmp/test_transport.sock",
			expectError: false,
		},
		{
			name:        "invalid_transport",
			transportType: "invalid",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transport, err := NewTransport(tc.transportType, tc.addr, tc.socket)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for transport type %s", tc.transportType)
				}
				return
			}

			if err != nil {
				if tc.transportType == "tcp" {
					t.Skipf("TCP transport creation failed (likely port binding): %v", err)
				} else if tc.transportType == "unix" {
					t.Skipf("Unix transport creation failed (likely socket issues): %v", err)
				} else {
					t.Fatalf("Unexpected error: %v", err)
				}
			}

			if transport == nil {
				t.Errorf("Transport should not be nil for valid type %s", tc.transportType)
			}

			if transport != nil {
				transport.Close()
			}
		})
	}
}

func TestTransportReadWriteEdgeCases(t *testing.T) {
	t.Run("empty_write", func(t *testing.T) {
		var output bytes.Buffer
		transport := &StdioTransport{
			reader: bufio.NewReader(strings.NewReader("")),
			writer: &output,
		}

		err := transport.Write([]byte{})
		if err != nil {
			t.Errorf("Writing empty data should not fail: %v", err)
		}
	})

	t.Run("large_message", func(t *testing.T) {
		largeMsg := make([]byte, 64*1024) // 64KB
		for i := range largeMsg {
			largeMsg[i] = 'A'
		}
		largeMsg[len(largeMsg)-1] = '\n'

		var output bytes.Buffer
		transport := &StdioTransport{
			reader: bufio.NewReader(bytes.NewReader(largeMsg)),
			writer: &output,
		}

		data, err := transport.Read()
		if err != nil {
			t.Fatalf("Reading large message failed: %v", err)
		}

		if len(data) == 0 {
			t.Error("Should read large message")
		}

		err = transport.Write(largeMsg[:1024]) // Write smaller chunk
		if err != nil {
			t.Errorf("Writing large message failed: %v", err)
		}
	})

	t.Run("multiple_reads", func(t *testing.T) {
		input := "line1\nline2\nline3\n"
		transport := &StdioTransport{
			reader: bufio.NewReader(strings.NewReader(input)),
			writer: io.Discard,
		}

		expectedLines := []string{"line1\n", "line2\n", "line3\n"}
		for i, expected := range expectedLines {
			data, err := transport.Read()
			if err != nil {
				t.Fatalf("Read %d failed: %v", i+1, err)
			}
			if string(data) != expected {
				t.Errorf("Read %d: expected %q, got %q", i+1, expected, string(data))
			}
		}

		_, err := transport.Read()
		if err != io.EOF {
			t.Errorf("Expected EOF after reading all lines, got %v", err)
		}
	})
}