package main

import (
	"testing"
)

func TestCopyMaskFiltering(t *testing.T) {
	tests := []struct {
		name      string
		builders  []Builder
		testCases []struct {
			alias    string
			path     string
			expected bool
		}
	}{
		{
			name: "single builder with single copy",
			builders: []Builder{
				{
					alias:    "builder",
					pullspec: "builder-image",
					copies: []Copy{
						{source: []string{"/app"}, dest: "/usr/app", stage: FINAL_STAGE},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"builder", "app/file.txt", true},
				{"builder", "app", true},
				{"builder", "app/subdir/file.txt", true},
				{"builder", "other/file.txt", false},
				{"builder", "ap", false},
			},
		},
		{
			name: "transitive copy",
			builders: []Builder{
				{
					alias:    "first",
					pullspec: "builder-image",
					copies: []Copy{
						{source: []string{"/app"}, dest: "/usr/app", stage: "second"},
					},
				},
				{
					alias:    "second",
					pullspec: "builder-image",
					copies: []Copy{
						{source: []string{"/usr/app"}, dest: "/usr/app", stage: FINAL_STAGE},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"first", "app/file.txt", true},
				{"first", "app", true},
				{"first", "app/subdir/file.txt", true},
				{"first", "other/file.txt", false},
				{"first", "ap", false},
				{"second", "usr/app/file.txt", true},
				{"second", "usr/app/subdir/file.txt", true},
				{"second", "ap", false},
			},
		},
		{
			name: "transitive and final copy mix",
			builders: []Builder{
				{
					alias:    "first",
					pullspec: "builder-image",
					copies: []Copy{
						{source: []string{"/app"}, dest: "/usr/app", stage: "second"},
						{source: []string{"/lib"}, dest: "/app/lib", stage: FINAL_STAGE},
					},
				},
				{
					alias:    "second",
					pullspec: "builder-image",
					copies: []Copy{
						{source: []string{"/usr/app"}, dest: "/usr/app", stage: FINAL_STAGE},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"first", "app/file.txt", true},
				{"first", "lib/lib.h", true},
			},
		},
		{
			name: "root path as source",
			builders: []Builder{
				{
					alias:    "test",
					pullspec: "test-image",
					copies: []Copy{
						{source: []string{"/"}, dest: "/copy"},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"test", "anything", true},
			},
		},
		{
			name: "path exactly matches source",
			builders: []Builder{
				{
					alias:    "test",
					pullspec: "test-image",
					copies: []Copy{
						{source: []string{"/exact/path"}, dest: "/dest"},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"test", "exact/path", true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masks := NewCopyMasks(tt.builders)

			for _, tc := range tt.testCases {
				mask, exists := masks[tc.alias]
				if !exists {
					t.Errorf("No mask found for alias %q", tc.alias)
					continue
				}
				result := mask.Includes(tc.path)
				if result != tc.expected {
					t.Errorf("Includes(%q) for alias %q = %v, want %v", tc.path, tc.alias, result, tc.expected)
				}
			}
		})
	}
}

func TestNewCopyMasksReturnsEmptyOnEmptyBuilders(t *testing.T) {
	result := NewCopyMasks([]Builder{})
	if len(result) != 0 {
		t.Errorf("NewCopyMasks should return empty map when given empty builders slice, got %v", result)
	}
}
