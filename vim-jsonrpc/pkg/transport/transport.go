package transport

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
)

type Transport interface {
	Read() ([]byte, error)
	Write([]byte) error
	Close() error
}

type StdioTransport struct {
	reader *bufio.Reader
	writer io.Writer
}

func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}
}

func (t *StdioTransport) Read() ([]byte, error) {
	return t.reader.ReadBytes('\n')
}

func (t *StdioTransport) Write(data []byte) error {
	_, err := t.writer.Write(append(data, '\n'))
	return err
}

func (t *StdioTransport) Close() error {
	return nil
}

type TCPTransport struct {
	conn   net.Conn
	reader *bufio.Reader
}

func NewTCPTransport(addr string) (*TCPTransport, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &TCPTransport{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

func (t *TCPTransport) Read() ([]byte, error) {
	return t.reader.ReadBytes('\n')
}

func (t *TCPTransport) Write(data []byte) error {
	_, err := t.conn.Write(append(data, '\n'))
	return err
}

func (t *TCPTransport) Close() error {
	return t.conn.Close()
}

type UnixTransport struct {
	conn   net.Conn
	reader *bufio.Reader
}

func NewUnixTransport(socket string) (*UnixTransport, error) {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, err
	}
	return &UnixTransport{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}, nil
}

func (t *UnixTransport) Read() ([]byte, error) {
	return t.reader.ReadBytes('\n')
}

func (t *UnixTransport) Write(data []byte) error {
	_, err := t.conn.Write(append(data, '\n'))
	return err
}

func (t *UnixTransport) Close() error {
	return t.conn.Close()
}

func NewTransport(transportType, addr, socket string) (Transport, error) {
	switch transportType {
	case "stdio":
		return NewStdioTransport(), nil
	case "tcp":
		return NewTCPTransport(addr)
	case "unix":
		return NewUnixTransport(socket)
	default:
		return nil, fmt.Errorf("unknown transport type: %s", transportType)
	}
}