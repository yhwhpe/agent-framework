package config

import (
	"testing"
)

func TestParseMinIOEndpoint(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"localhost:9000", "localhost:9000"},
		{"http://localhost:9000", "localhost:9000"},
		{"https://minio.example.com:9000", "minio.example.com:9000"},
		{"http://minio.example.com:9000/path", "minio.example.com:9000"},
		{"http://minio.example.com:9000/path/to/something", "minio.example.com:9000"},
		{"${{Bucket.MINIO_PRIVATE_HOST}}:${{Bucket.MINIO_PRIVATE_PORT}}", ""},
		{"http://${{Bucket.MINIO_PRIVATE_HOST}}:${{Bucket.MINIO_PRIVATE_PORT}}", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := parseMinIOEndpoint(test.input)
		if result != test.expected {
			t.Errorf("parseMinIOEndpoint(%q) = %q; expected %q", test.input, result, test.expected)
		}
	}
}
