package main

func defaultConfig() config {
	return config{
		Name:             "Cloudflare /user/tokens/verify drift watch",
		SnapshotDate:     "2026-03-22",
		URL:              "https://api.cloudflare.com/client/v4/user/tokens/verify",
		UserAgent:        "cloudflare-ddns-token-probe",
		PauseBetweenRuns: "5s",
		RequestTimeout:   "15s",
		Reminders: []string{
			"Re-check error classification in internal/api/cloudflare.go:CheckUsability if any observed response changes.",
			"CheckUsability treats RequestError and AuthorizationError as fatal (invalid token). Verify this still matches the observed error types.",
			"CheckUsability treats AuthenticationError (HTTP 403) as non-fatal/defensive. If 403 is ever observed here, re-evaluate.",
		},
		RelatedPaths: []string{
			"internal/api/cloudflare.go",
		},
		Probes: []probe{
			{
				Name:                 "missing Authorization",
				Kind:                 "missing-header",
				IncludeAuthorization: boolPtr(false),
				ExpectedRaw: expectedRaw{
					StatusCode:   400,
					Success:      false,
					ResultStatus: nil,
					Errors: []apiError{
						{
							Code:    1001,
							Message: "Missing \"Authorization\" header",
						},
					},
					Messages: []apiMessageInfo{},
				},
				ExpectedSDK: nil,
			},
			{
				Name:  "malformed bearer with spaces",
				Kind:  "malformed",
				Token: "not a token",
				ExpectedRaw: expectedRaw{
					StatusCode:   400,
					Success:      false,
					ResultStatus: nil,
					Errors: []apiError{
						{
							Code:    6003,
							Message: "Invalid request headers",
							ErrorChain: []apiErrorChain{
								{
									Code:    6111,
									Message: "Invalid format for Authorization header",
								},
							},
						},
					},
					Messages: []apiMessageInfo{},
				},
				ExpectedSDK: &expectedSDK{
					ErrorType:    "RequestError",
					ErrorCode:    6003,
					ErrorMessage: "Invalid request headers (6003)",
				},
			},
			{
				Name:  "malformed bearer with hyphen",
				Kind:  "malformed",
				Token: "abc-def",
				ExpectedRaw: expectedRaw{
					StatusCode:   400,
					Success:      false,
					ResultStatus: nil,
					Errors: []apiError{
						{
							Code:    6003,
							Message: "Invalid request headers",
							ErrorChain: []apiErrorChain{
								{
									Code:    6111,
									Message: "Invalid format for Authorization header",
								},
							},
						},
					},
					Messages: []apiMessageInfo{},
				},
				ExpectedSDK: &expectedSDK{
					ErrorType:    "RequestError",
					ErrorCode:    6003,
					ErrorMessage: "Invalid request headers (6003)",
				},
			},
			{
				Name:  "official API token example",
				Kind:  "well-formed-looking, same family, wrong token",
				Token: "Sn3lZJTBX6kkg7OdcBUAxOO963GEIyGQqnFTOFYY",
				ExpectedRaw: expectedRaw{
					StatusCode:   401,
					Success:      false,
					ResultStatus: nil,
					Errors: []apiError{
						{
							Code:    1000,
							Message: "Invalid API Token",
						},
					},
					Messages: []apiMessageInfo{},
				},
				ExpectedSDK: &expectedSDK{
					ErrorType:    "AuthorizationError",
					ErrorCode:    1000,
					ErrorMessage: "Invalid API Token (1000)",
				},
			},
		},
	}
}

func boolPtr(value bool) *bool {
	return &value
}
