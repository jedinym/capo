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
					alias:    "stage1",
					pullspec: "image1",
					copies: []Copy{
						{source: []string{"/app"}, dest: "/usr/app"},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"stage1", "app/file.txt", true},
				{"stage1", "app", true},
				{"stage1", "app/subdir/file.txt", true},
				{"stage1", "other/file.txt", false},
				{"stage1", "ap", false},
			},
		},
		{
			name: "multiple builders with dependency chain",
			builders: []Builder{
				{
					alias:    "base",
					pullspec: "base-image",
					copies: []Copy{
						{source: []string{"/src"}, dest: "/build"},
					},
				},
				{
					alias:    "final",
					pullspec: "final-image",
					copies: []Copy{
						{source: []string{"/build"}, dest: "/app"},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"final", "build/file.txt", true},
				{"final", "build", true},
				{"final", "build/subdir/file.txt", true},
				{"base", "src/file.txt", true},
				{"base", "src", true},
				{"final", "other/file.txt", false},
				{"base", "other/file.txt", false},
			},
		},
		{
			name: "builders with copies not in dependency tree",
			builders: []Builder{
				{
					alias:    "unused",
					pullspec: "unused-image",
					copies: []Copy{
						{source: []string{"/unused/path"}, dest: "/nowhere"},
					},
				},
				{
					alias:    "base",
					pullspec: "base-image",
					copies: []Copy{
						{source: []string{"/src"}, dest: "/build"},
					},
				},
				{
					alias:    "final",
					pullspec: "final-image",
					copies: []Copy{
						{source: []string{"/build"}, dest: "/app"},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"final", "build/file.txt", true},
				{"base", "src/file.txt", true},
				{"unused", "unused/path/file.txt", false}, // Not in dependency tree from final
				{"unused", "unused/path", false},
				{"final", "unused/path/file.txt", false},
			},
		},
		{
			name: "complex multi-stage with multiple sources",
			builders: []Builder{
				{
					alias:    "deps",
					pullspec: "deps-image",
					copies: []Copy{
						{source: []string{"/deps/lib1", "/deps/lib2"}, dest: "/libraries"},
					},
				},
				{
					alias:    "build",
					pullspec: "build-image",
					copies: []Copy{
						{source: []string{"/libraries"}, dest: "/compiled"},
						{source: []string{"/src"}, dest: "/compiled/src"},
					},
				},
				{
					alias:    "final",
					pullspec: "final-image",
					copies: []Copy{
						{source: []string{"/compiled"}, dest: "/app"},
					},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"final", "compiled/app", true},
				{"final", "compiled", true},
				{"build", "libraries/lib.so", true},
				{"build", "src/main.go", true},
				{"deps", "deps/lib1/header.h", true},
				{"deps", "deps/lib2/binary", true},
				{"final", "other/path", false},
				{"build", "other/path", false},
			},
		},
		{
			name: "builder with no copies",
			builders: []Builder{
				{
					alias:    "empty",
					pullspec: "empty-image",
					copies:   []Copy{},
				},
			},
			testCases: []struct {
				alias    string
				path     string
				expected bool
			}{
				{"empty", "any/path", false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mask := NewCopyMask(tt.builders)

			for _, tc := range tt.testCases {
				result := mask.Includes(tc.alias, tc.path)
				if result != tc.expected {
					t.Errorf("Includes(%q, %q) = %v, want %v", tc.alias, tc.path, result, tc.expected)
				}
			}
		})
	}
}

func TestCopyMaskWithEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		builders []Builder
		alias    string
		path     string
		expected bool
	}{
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
			alias:    "test",
			path:     "anything",
			expected: true,
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
			alias:    "test",
			path:     "exact/path",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mask := NewCopyMask(tt.builders)
			result := mask.Includes(tt.alias, tt.path)
			if result != tt.expected {
				t.Errorf("Includes(%q, %q) = %v, want %v", tt.alias, tt.path, result, tt.expected)
			}
		})
	}
}

func TestNewCopyMaskPanicsOnEmptyBuilders(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewCopyMask should panic when given empty builders slice")
		}
	}()

	NewCopyMask([]Builder{})
}
