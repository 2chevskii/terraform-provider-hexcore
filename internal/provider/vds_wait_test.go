package provider

import "testing"

func TestReadyRegionObservation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		region *string
		want   bool
	}{
		{name: "matching", region: str("test-region"), want: true},
		{name: "omitted", want: true},
		{name: "mismatching", region: str("another-region")},
		{name: "empty", region: str("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := syntheticVDS("123")
			v.Region = tc.region
			if got := ready(&v, "tf-offline", "test-region", vdsShape{1, 1024, 10}); got != tc.want {
				t.Fatalf("ready = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestHaltedStatusObservation(t *testing.T) {
	for _, tc := range []struct {
		status *string
		want   bool
	}{
		{status: str("halted"), want: true},
		{status: str("Halted"), want: true},
		{status: str("Running")},
		{status: str("halting")},
		{},
	} {
		v := syntheticVDS("123")
		v.Status = tc.status
		if got := halted(observation{presence: present, vds: &v}); got != tc.want {
			t.Fatalf("halted(%v) = %t, want %t", tc.status, got, tc.want)
		}
	}
	if halted(observation{presence: absent}) {
		t.Fatal("absent VDS must not be considered halted")
	}
}
