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

func TestDescribeCommentRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		regex string
		want  string
	}{
		{
			name:  "empty matches all comments",
			regex: "",
			want:  "(empty; matches all comments)",
		},
		{
			name:  "readable regex stays raw",
			regex: "^Created by Cloudflare DDNS$",
			want:  "^Created by Cloudflare DDNS$",
		},
		{
			name:  "graphic unicode stays raw",
			regex: "^猫+$",
			want:  "^猫+$",
		},
		{
			name:  "leading whitespace is quoted",
			regex: " ^Created by Cloudflare DDNS$",
			want:  "\" ^Created by Cloudflare DDNS$\"",
		},
		{
			name:  "trailing whitespace is quoted",
			regex: "^Created by Cloudflare DDNS$ ",
			want:  "\"^Created by Cloudflare DDNS$ \"",
		},
		{
			name:  "control characters are quoted",
			regex: "^Created by\tCloudflare DDNS$",
			want:  "\"^Created by\\tCloudflare DDNS$\"",
		},
		{
			name:  "newlines are quoted",
			regex: "^Created by\nCloudflare DDNS$",
			want:  "\"^Created by\\nCloudflare DDNS$\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := describeCommentRegex(test.regex); got != test.want {
				t.Fatalf("describeCommentRegex(%q) = %q, want %q", test.regex, got, test.want)
			}
		})
	}
}
