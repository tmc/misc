package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/tmc/misc/vim-jsonrpc/pkg/protocol"
)

type HandlerFunc func(ctx context.Context, params interface{}) (interface{}, error)

type Server struct {
	transportType string
	addr          string
	socket        string
	handlers      map[string]HandlerFunc
	mu            sync.RWMutex
	listener      net.Listener
}

func New(transportType, addr, socket string) (*Server, error) {
	return &Server{
		transportType: transportType,
		addr:          addr,
		socket:        socket,
		handlers:      make(map[string]HandlerFunc),
	}, nil
}

func (s *Server) RegisterHandler(method string, handler HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

func (s *Server) Start() error {
	switch s.transportType {
	case "stdio":
		return s.startStdio()
	case "tcp":
		return s.startTCP()
	case "unix":
		return s.startUnix()
	default:
		return fmt.Errorf("unknown transport type: %s", s.transportType)
	}
}

func (s *Server) startStdio() error {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		
		resp := s.handleMessage([]byte(line))
		if resp != nil {
			data, err := json.Marshal(resp)
			if err != nil {
				log.Printf("Failed to marshal response: %v", err)
				continue
			}
			fmt.Println(string(data))
		}
	}
	return scanner.Err()
}

func (s *Server) startTCP() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener
	defer listener.Close()
	
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) startUnix() error {
	os.Remove(s.socket)
	listener, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	s.listener = listener
	defer listener.Close()
	
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		
		resp := s.handleMessage([]byte(line))
		if resp != nil {
			data, err := json.Marshal(resp)
			if err != nil {
				log.Printf("Failed to marshal response: %v", err)
				continue
			}
			conn.Write(append(data, '\n'))
		}
	}
}

func (s *Server) handleMessage(data []byte) *protocol.Response {
	msg, err := protocol.ParseMessage(data)
	if err != nil {
		return protocol.NewErrorResponse(nil, protocol.ParseError, err.Error(), nil)
	}
	
	switch m := msg.(type) {
	case *protocol.Request:
		return s.handleRequest(m)
	case *protocol.Notification:
		s.handleNotification(m)
		return nil
	default:
		return protocol.NewErrorResponse(nil, protocol.InvalidRequest, "Invalid request", nil)
	}
}

func (s *Server) handleRequest(req *protocol.Request) *protocol.Response {
	s.mu.RLock()
	handler, exists := s.handlers[req.Method]
	s.mu.RUnlock()
	
	if !exists {
		return protocol.NewErrorResponse(req.ID, protocol.MethodNotFound, "Method not found", nil)
	}
	
	ctx := context.Background()
	result, err := handler(ctx, req.Params)
	if err != nil {
		return protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
	}
	
	return protocol.NewResponse(req.ID, result)
}

func (s *Server) handleNotification(notif *protocol.Notification) {
	s.mu.RLock()
	handler, exists := s.handlers[notif.Method]
	s.mu.RUnlock()
	
	if exists {
		ctx := context.Background()
		go handler(ctx, notif.Params)
	}
}

func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}