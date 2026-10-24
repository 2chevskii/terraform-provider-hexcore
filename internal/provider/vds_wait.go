package provider

import (
	"context"
	"errors"
	"net"
	"time"

	hexcore "github.com/2chevskii/go-hexcore"
)

type presence int

const (
	indeterminate presence = iota
	present
	absent
)

type observation struct {
	presence presence
	vds      *hexcore.VDS
}

// List is complete and unfiltered in SDK v1.0.0. Revisit this policy if pagination
// or filters are introduced. Only a successful list can establish absence.
func observeVDS(ctx context.Context, client *hexcore.Client, id string) (observation, error) {
	got, getErr := client.VDS.Get(ctx, id)
	items, listErr := client.VDS.List(ctx)
	if listErr != nil {
		return observation{presence: indeterminate}, listErr
	}
	for _, listed := range items {
		if listed.ID != id {
			continue
		}
		if getErr != nil {
			return observation{presence: present, vds: &listed}, nil
		}
		if got == nil || got.ID != id {
			return observation{presence: indeterminate}, errors.New("unexpected VDS identity")
		}
		// Get owns fields it actually exposes; fill omitted fields once from List.
		merged := *got
		if merged.Status == nil {
			merged.Status = listed.Status
		}
		if merged.Region == nil {
			merged.Region = listed.Region
		}
		if merged.IPv4 == nil {
			merged.IPv4 = listed.IPv4
		}
		if merged.Price == nil {
			merged.Price = listed.Price
		}
		if merged.CreatedAt == nil {
			merged.CreatedAt = listed.CreatedAt
		}
		if merged.NextBillDate == "" {
			merged.NextBillDate = listed.NextBillDate
		}
		return observation{presence: present, vds: &merged}, nil
	}
	return observation{presence: absent}, nil
}

func pause(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	t := time.NewTimer(interval)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func waitVDS(ctx context.Context, client *configuredClient, id string, done func(observation) bool) (observation, error) {
	last := observation{presence: indeterminate}
	for {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		current, err := observeVDS(ctx, client.api, id)
		if err != nil {
			return last, err
		}
		last = current
		if done(current) {
			return current, nil
		}
		if err := pause(ctx, client.pollInterval); err != nil {
			return last, err
		}
	}
}

type vdsShape struct{ vcpu, memoryMB, diskGB int64 }

func ready(v *hexcore.VDS, name, region string, shape vdsShape) bool {
	if v == nil || v.Status == nil || *v.Status != "Running" || v.IPv4 == nil {
		return false
	}
	ip := net.ParseIP(*v.IPv4)
	// Get and List can both omit region. Preserve the configured selector in
	// that case, but reject a mismatching region when the API reports one.
	regionMatches := v.Region == nil || *v.Region == region
	return ip != nil && ip.To4() != nil && !ip.IsUnspecified() && v.Name == name && regionMatches && v.VCPU == shape.vcpu && v.Memory == shape.memoryMB && v.Disk == shape.diskGB
}

func halted(o observation) bool {
	return o.presence == present && o.vds != nil && o.vds.Status != nil && (*o.vds.Status == "halted" || *o.vds.Status == "Halted")
}
