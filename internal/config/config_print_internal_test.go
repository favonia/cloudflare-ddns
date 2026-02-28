package config

import "testing"

func TestDescribeLiteralText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "(empty)",
		},
		{
			name:  "plain text",
			input: "Created by Cloudflare DDNS",
			want:  "\"Created by Cloudflare DDNS\"",
		},
		{
			name:  "control characters stay visible",
			input: "Created by\tCloudflare DDNS",
			want:  "\"Created by\\tCloudflare DDNS\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := describeLiteralText(test.input); got != test.want {
				t.Fatalf("describeLiteralText(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestDescribeCommentRegexTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "empty matches all comments",
			template: "",
			want:     "(empty; matches all comments)",
		},
		{
			name:     "readable regex stays raw",
			template: "^Created by Cloudflare DDNS$",
			want:     "^Created by Cloudflare DDNS$",
		},
		{
			name:     "graphic unicode stays raw",
			template: "^猫+$",
			want:     "^猫+$",
		},
		{
			name:     "leading whitespace is quoted",
			template: " ^Created by Cloudflare DDNS$",
			want:     "\" ^Created by Cloudflare DDNS$\"",
		},
		{
			name:     "trailing whitespace is quoted",
			template: "^Created by Cloudflare DDNS$ ",
			want:     "\"^Created by Cloudflare DDNS$ \"",
		},
		{
			name:     "control characters are quoted",
			template: "^Created by\tCloudflare DDNS$",
			want:     "\"^Created by\\tCloudflare DDNS$\"",
		},
		{
			name:     "newlines are quoted",
			template: "^Created by\nCloudflare DDNS$",
			want:     "\"^Created by\\nCloudflare DDNS$\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := describeCommentRegexTemplate(test.template); got != test.want {
				t.Fatalf("describeCommentRegexTemplate(%q) = %q, want %q", test.template, got, test.want)
			}
		})
	}
}
