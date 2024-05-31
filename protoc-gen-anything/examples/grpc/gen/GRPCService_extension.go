package proto

import (
	"context"
	"log"
)

// GRPCServiceServer is the server API for GRPCService service.
type GRPCServiceServer struct{}

// SendMessage handles the SendMessage RPC.
func (s *GRPCServiceServer) SendMessage(ctx context.Context, req *GRPCMessage) (*GRPCMessage, error) {
	log.Printf("Received message: %s", req.Content)
	return &GRPCMessage{Id: req.Id, Content: "Echo: " + req.Content}, nil
}

// StreamMessages handles the StreamMessages RPC.
func (s *GRPCServiceServer) StreamMessages(ctx context.Context, req *GRPCMessage) (*GRPCMessage, error) {
	log.Printf("Received message: %s", req.Content)
	return &GRPCMessage{Id: req.Id, Content: "Echo: " + req.Content}, nil
}
