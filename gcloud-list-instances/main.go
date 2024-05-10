package main

import (
	"context"
	"fmt"
	"os"

	compute "google.golang.org/api/compute/v1"
)

func main() {
	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		fmt.Printf("Error creating Compute service: %v\n", err)
		return
	}
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	instancesService := compute.NewInstancesService(computeService)
	req := instancesService.AggregatedList(projectID)
	if err := req.Pages(ctx, func(page *compute.InstanceAggregatedList) error {
		for zone, instancesScopedList := range page.Items {
			for _, instance := range instancesScopedList.Instances {
				fmt.Printf("Instance: %s, Zone: %s\n", instance.Name, zone)
			}
		}
		return nil
	}); err != nil {
		fmt.Printf("Error listing instances: %v\n", err)
		return
	}
}
