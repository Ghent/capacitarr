package integrations

import "testing"

func TestArrExtractPosterURL(t *testing.T) {
	tests := []struct {
		name     string
		images   []arrImage
		expected string
	}{
		{
			name:     "empty array returns empty string",
			images:   []arrImage{},
			expected: "",
		},
		{
			name:     "nil array returns empty string",
			images:   nil,
			expected: "",
		},
		{
			name: "no poster type returns empty string",
			images: []arrImage{
				{CoverType: "banner", RemoteURL: "https://example.com/banner.jpg", URL: "/banner.jpg"},
				{CoverType: "fanart", RemoteURL: "https://example.com/fanart.jpg", URL: "/fanart.jpg"},
			},
			expected: "",
		},
		{
			name: "poster with remoteUrl preferred over url",
			images: []arrImage{
				{CoverType: "banner", RemoteURL: "https://example.com/banner.jpg"},
				{CoverType: "poster", RemoteURL: "https://cdn.example.com/poster.jpg", URL: "/api/v3/mediacover/1/poster.jpg"},
			},
			expected: "https://cdn.example.com/poster.jpg",
		},
		{
			name: "poster with only url when remoteUrl is empty",
			images: []arrImage{
				{CoverType: "poster", RemoteURL: "", URL: "/api/v3/mediacover/1/poster.jpg"},
			},
			expected: "/api/v3/mediacover/1/poster.jpg",
		},
		{
			name: "poster with only url when remoteUrl is missing",
			images: []arrImage{
				{CoverType: "poster", URL: "/api/v3/mediacover/1/poster.jpg"},
			},
			expected: "/api/v3/mediacover/1/poster.jpg",
		},
		{
			name: "cover type fallback for Readarr books",
			images: []arrImage{
				{CoverType: "cover", RemoteURL: "https://cdn.example.com/cover.jpg", URL: "/cover.jpg"},
			},
			expected: "https://cdn.example.com/cover.jpg",
		},
		{
			name: "cover type with only url",
			images: []arrImage{
				{CoverType: "cover", RemoteURL: "", URL: "/api/v1/mediacover/1/cover.jpg"},
			},
			expected: "/api/v1/mediacover/1/cover.jpg",
		},
		{
			name: "poster type preferred over cover type",
			images: []arrImage{
				{CoverType: "cover", RemoteURL: "https://cdn.example.com/cover.jpg"},
				{CoverType: "poster", RemoteURL: "https://cdn.example.com/poster.jpg"},
			},
			expected: "https://cdn.example.com/poster.jpg",
		},
		{
			name: "first poster wins if multiple posterURLs exist",
			images: []arrImage{
				{CoverType: "poster", RemoteURL: "https://cdn.example.com/first-poster.jpg"},
				{CoverType: "poster", RemoteURL: "https://cdn.example.com/second-poster.jpg"},
			},
			expected: "https://cdn.example.com/first-poster.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arrExtractPosterURL(tt.images)
			if result != tt.expected {
				t.Errorf("arrExtractPosterURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}
