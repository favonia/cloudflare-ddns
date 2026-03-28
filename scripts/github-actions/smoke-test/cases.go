package main

var allCases = []smokeCase{
	{
		Name:             "emoji-invalid",
		Env:              map[string]string{"EMOJI": "invalid"},
		ExpectedExitCode: 1,
		ExactOutput: "😡 EMOJI (\"invalid\") is not a boolean: " +
			"strconv.ParseBool: parsing \"invalid\": invalid syntax\n👋 Bye!",
	},
	{
		Name: "managed-record-regex-invalid",
		Env: map[string]string{
			"CLOUDFLARE_API_TOKEN":          "deadbeaf",
			"DOMAINS":                       "example.org",
			"MANAGED_RECORDS_COMMENT_REGEX": "(",
			"UPDATE_CRON":                   "@once",
		},
		ExpectedExitCode: 1,
		OrderedFragments: []string{
			"🌟 Cloudflare DDNS",
			"📖 Reading settings . . .",
			"📖 Checking settings . . .",
			"MANAGED_RECORDS_COMMENT_REGEX=\"(\" is invalid",
			"👋 Bye!",
		},
	},
	{
		Name:             "quiet-invalid",
		Env:              map[string]string{"EMOJI": "false", "QUIET": "invalid"},
		ExpectedExitCode: 1,
		ExactOutput: "QUIET (\"invalid\") is not a boolean: " +
			"strconv.ParseBool: parsing \"invalid\": invalid syntax\nBye!",
	},
	{
		Name: "waf-item-comment-regex-mismatch",
		Env: map[string]string{
			"CLOUDFLARE_API_TOKEN":                 "deadbeaf",
			"MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX": "^other$",
			"UPDATE_CRON":                          "@once",
			"WAF_LISTS":                            "account/list",
			"WAF_LIST_ITEM_COMMENT":                "managed-waf",
		},
		ExpectedExitCode: 1,
		OrderedFragments: []string{
			"🌟 Cloudflare DDNS",
			"📖 Reading settings . . .",
			"📖 Checking settings . . .",
			"WAF_LIST_ITEM_COMMENT=\"managed-waf\" does not match MANAGED_WAF_LIST_ITEMS_COMMENT_REGEX=\"^other$\"",
			"👋 Bye!",
		},
	},
}
