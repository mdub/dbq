package main

import (
	"context"
	"fmt"

	"github.com/databricks/databricks-sdk-go/service/sql"
)

// WarehousesCmd lists SQL warehouses
type WarehousesCmd struct{}

func (c *WarehousesCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()
	warehouses, err := client.Warehouses.ListAll(ctx, sql.ListWarehousesRequest{})
	if err != nil {
		return fmt.Errorf("failed to list warehouses: %w", err)
	}

	for _, wh := range warehouses {
		fmt.Printf("%-20s %-40s %s\n", wh.Id, wh.Name, wh.State)
	}
	return nil
}
