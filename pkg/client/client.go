package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
)

type RpcClient struct {
	endpoint string
}

func NewClient(endpoint string) *RpcClient {
	return &RpcClient{endpoint: endpoint}
}

// --- ExecuteQuery ---

type ExecuteQueryArgs struct {
	DbName    string `json:"db_name"`
	Query     string `json:"query"`
	SessionID string `json:"session_id,omitempty"`
}

type ExecuteQueryResult struct {
	Result    string `json:"result"`
	SessionID string `json:"session_id,omitempty"`
}

type JSONRPCRequest struct {
	Method string `json:"method"`
	Params [1]any `json:"params"`
	ID     int    `json:"id"`
}

type JSONRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  interface{}     `json:"error"`
	ID     int             `json:"id"`
}

func (c *RpcClient) ExecuteQuery(dbName, query, sessionID string) (*ExecuteQueryResult, error) {
	args := ExecuteQueryArgs{
		DbName:    dbName,
		Query:     query,
		SessionID: sessionID,
	}
	request := JSONRPCRequest{
		Method: "wwfs.ExecuteQuery",
		Params: [1]any{args},
		ID:     1,
	}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(c.endpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC server: %w. Is the server running?", err)
	}
	defer resp.Body.Close()

	var rpcResponse JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResponse); err != nil {
		return nil, err
	}

	if rpcResponse.Error != nil {
		return nil, fmt.Errorf("RPC error: %v", rpcResponse.Error)
	}

	var result ExecuteQueryResult
	if err := json.Unmarshal(rpcResponse.Result, &result); err != nil {
		// If the result is a plain string, it might be a direct result from a non-select query
		var strResult string
		if err2 := json.Unmarshal(rpcResponse.Result, &strResult); err2 == nil {
			return &ExecuteQueryResult{Result: strResult}, nil
		}
		return nil, err
	}

	return &result, nil
}

// --- GetTableSchema ---

type GetTableSchemaArgs struct {
	DbName    string `json:"db_name"`
	TableName string `json:"table_name"`
}

type GetTableSchemaResult struct {
	Schema *proto.Schema `json:"schema"`
}

func (c *RpcClient) GetTableSchema(dbName, tableName string) (*proto.Schema, error) {
	args := GetTableSchemaArgs{
		DbName:    dbName,
		TableName: tableName,
	}
	request := JSONRPCRequest{
		Method: "wwfs.GetTableSchema",
		Params: [1]any{args},
		ID:     1,
	}
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(c.endpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC server: %w. Is the server running?", err)
	}
	defer resp.Body.Close()

	var rpcResponse JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResponse); err != nil {
		return nil, err
	}

	if rpcResponse.Error != nil {
		return nil, fmt.Errorf("RPC error: %v", rpcResponse.Error)
	}

	var result GetTableSchemaResult
	if err := json.Unmarshal(rpcResponse.Result, &result); err != nil {
		return nil, err
	}

	return result.Schema, nil
}
