package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

func getWorkspaceHost() (string, error) {
	if CLI.Workspace == "" {
		return "", fmt.Errorf("no workspace specified. Use --workspace or $DBQ_WORKSPACE")
	}
	host := CLI.Workspace
	// Support simple names like "my-workspace"
	if !strings.Contains(host, ".") {
		host = host + ".cloud.databricks.com"
	}
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	return strings.TrimSuffix(host, "/"), nil
}

const defaultWarehouse = "Serverless Starter Warehouse"

func getWarehouseID(client *databricks.WorkspaceClient) (string, error) {
	warehouse := CLI.Warehouse
	if warehouse == "" {
		warehouse = defaultWarehouse
	}

	// If it looks like a warehouse ID (hex string), return as-is
	if isWarehouseID(warehouse) {
		return warehouse, nil
	}

	// Otherwise, look up by name
	ctx := context.Background()
	warehouses, err := client.Warehouses.ListAll(ctx, sql.ListWarehousesRequest{})
	if err != nil {
		return "", fmt.Errorf("failed to list warehouses: %w", err)
	}

	for _, wh := range warehouses {
		if wh.Name == warehouse {
			return wh.Id, nil
		}
	}

	return "", fmt.Errorf("warehouse not found: %s", warehouse)
}

func isWarehouseID(s string) bool {
	if len(s) < 16 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
