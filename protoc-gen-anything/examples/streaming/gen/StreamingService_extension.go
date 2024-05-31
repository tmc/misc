package proto

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"time"
)

// StreamClient is a client for streaming data.
type StreamClient struct {
	client proto.StreamingServiceClient
}

// StreamData streams data to the server.
func (c *StreamClient) StreamData(ctx context.Context) error {
	stream, err := c.client.StreamData(ctx)
	if err != nil {
		return err
	}
	for i := 0; i < 10; i++ {
		msg := &proto.StreamMessage{
			Id:      fmt.Sprintf("msg-%d", i),
			Payload: fmt.Sprintf("Payload %d", i),
		}
		if err := stream.Send(msg); err != nil {
			return err
		}
		time.Sleep(time.Second)
	}
	stream.CloseSend()
	return nil
}
