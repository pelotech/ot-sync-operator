package resourcegenerator

import (
	"testing"
)

func TestCalculateDiskSize(t *testing.T) {
	testCases := []struct {
		name        string
		diskSize    string
		addDiskSize bool
		expected    string
		expectErr   bool
	}{
		{
			name:        "should accept Gi",
			diskSize:    "20Gi",
			addDiskSize: false,
			expected:    "20Gi",
			expectErr:   false,
		},
		{
			name:        "should accept Mi",
			diskSize:    "20Mi",
			addDiskSize: false,
			expected:    "20Mi",
			expectErr:   false,
		},
		{
			name:        "should fail on integer",
			diskSize:    "2",
			addDiskSize: false,
			expected:    "",
			expectErr:   true,
		},
		{
			name:        "should fail on malformed string",
			diskSize:    "x34",
			addDiskSize: false,
			expected:    "",
			expectErr:   true,
		},
		{
			name:        "should increase disk size",
			diskSize:    "100Gi",
			addDiskSize: true,
			expected:    "133Gi",
			expectErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := CalculateDiskSize(tc.diskSize, tc.addDiskSize)
			if (err != nil) != tc.expectErr {
				t.Errorf("unexpected error status: got %v, want %v", err, tc.expectErr)
			}
			if result != tc.expected {
				t.Errorf("unexpected result: got %s, want %s", result, tc.expected)
			}
		})
	}
}
