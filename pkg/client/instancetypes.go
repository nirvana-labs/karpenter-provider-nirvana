package client

import (
	"context"
	"fmt"

	"github.com/nirvana-labs/nirvana-go/instance_types"
)

func (c *Client) GetInstanceType(ctx context.Context, region, name string) (*InstanceTypeSpec, error) {
	it, err := c.sdk.InstanceTypes.Get(ctx, instance_types.InstanceTypeGetParamsRegion(region), name)
	if err != nil {
		return nil, fmt.Errorf("getting instance type %s in %s: %w", name, region, err)
	}
	return &InstanceTypeSpec{
		Name:     it.Name,
		VCPU:     int(it.Vcpu),
		MemoryGB: int(it.MemoryGB),
		Region:   string(it.Region),
	}, nil
}

func (c *Client) ListInstanceTypes(ctx context.Context) ([]InstanceTypeSpec, error) {
	pager := c.sdk.InstanceTypes.ListAutoPaging(ctx, instance_types.InstanceTypeListParams{})

	var specs []InstanceTypeSpec
	for pager.Next() {
		it := pager.Current()
		specs = append(specs, InstanceTypeSpec{
			Name:     it.Name,
			VCPU:     int(it.Vcpu),
			MemoryGB: int(it.MemoryGB),
			Region:   string(it.Region),
		})
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("listing instance types: %w", err)
	}
	return specs, nil
}
