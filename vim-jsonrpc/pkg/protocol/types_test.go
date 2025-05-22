package protocol

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestParseRequest(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"method":"test","params":{"key":"value"}}`)
	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}
	
	req, ok := msg.(*Request)
	if !ok {
		t.Fatalf("Expected Request, got %T", msg)
	}
	
	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", req.JSONRPC)
	}
	if req.ID != float64(1) {
		t.Errorf("Expected ID 1, got %v", req.ID)
	}
	if req.Method != "test" {
		t.Errorf("Expected method test, got %s", req.Method)
	}
}

func TestParseNotification(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","method":"notify","params":["param1","param2"]}`)
	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}
	
	notif, ok := msg.(*Notification)
	if !ok {
		t.Fatalf("Expected Notification, got %T", msg)
	}
	
	if notif.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", notif.JSONRPC)
	}
	if notif.Method != "notify" {
		t.Errorf("Expected method notify, got %s", notif.Method)
	}
}

func TestParseResponse(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"result":"success"}`)
	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}
	
	resp, ok := msg.(*Response)
	if !ok {
		t.Fatalf("Expected Response, got %T", msg)
	}
	
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", resp.JSONRPC)
	}
	if resp.ID != float64(1) {
		t.Errorf("Expected ID 1, got %v", resp.ID)
	}
	if resp.Result != "success" {
		t.Errorf("Expected result success, got %v", resp.Result)
	}
}

func TestParseErrorResponse(t *testing.T) {
	data := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)
	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}
	
	resp, ok := msg.(*Response)
	if !ok {
		t.Fatalf("Expected Response, got %T", msg)
	}
	
	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Method not found" {
		t.Errorf("Expected error message 'Method not found', got %s", resp.Error.Message)
	}
}

func TestNewRequest(t *testing.T) {
	req := NewRequest(42, "test_method", map[string]string{"key": "value"})
	
	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", req.JSONRPC)
	}
	if req.ID != 42 {
		t.Errorf("Expected ID 42, got %v", req.ID)
	}
	if req.Method != "test_method" {
		t.Errorf("Expected method test_method, got %s", req.Method)
	}
}

func TestNewNotification(t *testing.T) {
	notif := NewNotification("test_notify", []string{"param1", "param2"})
	
	if notif.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", notif.JSONRPC)
	}
	if notif.Method != "test_notify" {
		t.Errorf("Expected method test_notify, got %s", notif.Method)
	}
}

func TestNewResponse(t *testing.T) {
	resp := NewResponse(123, "test_result")
	
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", resp.JSONRPC)
	}
	if resp.ID != 123 {
		t.Errorf("Expected ID 123, got %v", resp.ID)
	}
	if resp.Result != "test_result" {
		t.Errorf("Expected result test_result, got %v", resp.Result)
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(456, InvalidParams, "Invalid parameters", nil)
	
	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", resp.JSONRPC)
	}
	if resp.ID != 456 {
		t.Errorf("Expected ID 456, got %v", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}
	if resp.Error.Code != InvalidParams {
		t.Errorf("Expected error code %d, got %d", InvalidParams, resp.Error.Code)
	}
	if resp.Error.Message != "Invalid parameters" {
		t.Errorf("Expected error message 'Invalid parameters', got %s", resp.Error.Message)
	}
}

func TestErrorImplementsError(t *testing.T) {
	err := &Error{Code: -32600, Message: "Invalid Request"}
	expected := "JSON-RPC error -32600: Invalid Request"
	if err.Error() != expected {
		t.Errorf("Expected error string %s, got %s", expected, err.Error())
	}
}

func TestMarshalRequest(t *testing.T) {
	req := NewRequest(1, "subtract", []int{42, 23})
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	
	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}
	
	if unmarshaled.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %s", unmarshaled.JSONRPC)
	}
	if unmarshaled.Method != "subtract" {
		t.Errorf("Expected method subtract, got %s", unmarshaled.Method)
	}
}

func TestParseInvalidMessages(t *testing.T) {
	testCases := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{
			name:        "empty_json",
			data:        []byte(`{}`),
			expectError: true,
		},
		{
			name:        "invalid_jsonrpc_version",
			data:        []byte(`{"jsonrpc":"1.0","id":1,"method":"test"}`),
			expectError: true, // This should fail validation
		},
		{
			name:        "missing_method",
			data:        []byte(`{"jsonrpc":"2.0","id":1}`),
			expectError: false, // This will be parsed as response
		},
		{
			name:        "null_params",
			data:        []byte(`{"jsonrpc":"2.0","id":1,"method":"test","params":null}`),
			expectError: false,
		},
		{
			name:        "string_id",
			data:        []byte(`{"jsonrpc":"2.0","id":"test_id","method":"test"}`),
			expectError: false,
		},
		{
			name:        "null_id",
			data:        []byte(`{"jsonrpc":"2.0","id":null,"method":"test"}`),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseMessage(tc.data)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s", tc.name)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

func TestResponseWithComplexTypes(t *testing.T) {
	complexResult := map[string]interface{}{
		"status": "success",
		"data": []interface{}{
			map[string]interface{}{"id": 1, "name": "test1"},
			map[string]interface{}{"id": 2, "name": "test2"},
		},
		"metadata": map[string]interface{}{
			"total": 2,
			"page":  1,
		},
	}

	resp := NewResponse(42, complexResult)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal complex response: %v", err)
	}

	var unmarshaled Response
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal complex response: %v", err)
	}

	if unmarshaled.ID != float64(42) {
		t.Errorf("Expected ID 42, got %v", unmarshaled.ID)
	}

	resultMap, ok := unmarshaled.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be a map, got %T", unmarshaled.Result)
	}

	if resultMap["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", resultMap["status"])
	}
}

func TestNotificationWithoutID(t *testing.T) {
	notif := NewNotification("test_method", map[string]string{"key": "value"})
	
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	// Verify that ID field is not present in JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal to raw map: %v", err)
	}

	if _, exists := raw["id"]; exists {
		t.Error("Notification should not have an ID field")
	}

	if raw["jsonrpc"] != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %v", raw["jsonrpc"])
	}

	if raw["method"] != "test_method" {
		t.Errorf("Expected method test_method, got %v", raw["method"])
	}
}

func TestErrorWithData(t *testing.T) {
	errorData := map[string]interface{}{
		"details": "More error information",
		"code":    "INVALID_PARAM",
	}

	resp := NewErrorResponse(123, InvalidParams, "Invalid parameter provided", errorData)

	if resp.Error.Data == nil {
		t.Error("Error should have data")
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal error response with data: %v", err)
	}

	var unmarshaled Response
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if unmarshaled.Error == nil {
		t.Fatal("Expected error in response")
	}

	errorDataMap, ok := unmarshaled.Error.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected error data to be a map, got %T", unmarshaled.Error.Data)
	}

	if errorDataMap["details"] != "More error information" {
		t.Errorf("Expected error details, got %v", errorDataMap["details"])
	}
}

func TestBatchRequest(t *testing.T) {
	// Test batch request creation and parsing
	requests := []interface{}{
		NewRequest(1, "method1", "param1"),
		NewRequest(2, "method2", "param2"),
		NewNotification("notify", "param3"),
	}

	data, err := json.Marshal(requests)
	if err != nil {
		t.Fatalf("Failed to marshal batch request: %v", err)
	}

	var unmarshaled []json.RawMessage
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal batch request: %v", err)
	}

	if len(unmarshaled) != 3 {
		t.Errorf("Expected 3 requests in batch, got %d", len(unmarshaled))
	}

	// Parse each message in the batch
	for i, rawMsg := range unmarshaled {
		msg, err := ParseMessage(rawMsg)
		if err != nil {
			t.Errorf("Failed to parse message %d: %v", i, err)
		}

		switch i {
		case 0, 1:
			if _, ok := msg.(*Request); !ok {
				t.Errorf("Message %d should be a Request, got %T", i, msg)
			}
		case 2:
			if _, ok := msg.(*Notification); !ok {
				t.Errorf("Message %d should be a Notification, got %T", i, msg)
			}
		}
	}
}

func TestStandardErrorCodes(t *testing.T) {
	errorCodes := map[int]string{
		ParseError:     "Parse error",
		InvalidRequest: "Invalid Request",
		MethodNotFound: "Method not found",
		InvalidParams:  "Invalid params",
		InternalError:  "Internal error",
	}

	for code, description := range errorCodes {
		err := &Error{Code: code, Message: description}
		
		if err.Code != code {
			t.Errorf("Expected error code %d, got %d", code, err.Code)
		}

		errorString := err.Error()
		expectedFormat := fmt.Sprintf("JSON-RPC error %d: %s", code, description)
		if errorString != expectedFormat {
			t.Errorf("Expected error string %q, got %q", expectedFormat, errorString)
		}
	}
}