package main

func defaultConfig() config {
	const (
		zonesURL = "https://api.cloudflare.com/client/v4/zones"
		// A well-formed placeholder account ID; the invalid token is rejected
		// at the auth layer before the account ID is validated (spec §D).
		placeholderAccountID = "0123456789abcdef0123456789abcdef"
		wafListsURL          = "https://api.cloudflare.com/client/v4/accounts/" + placeholderAccountID + "/rules/lists"
		// #nosec G101 -- documented example token used only as a negative probe
		invalidToken = "Sn3lZJTBX6kkg7OdcBUAxOO963GEIyGQqnFTOFYY"
	)

	return config{
		Name:             "Cloudflare auth-error behavior watch",
		SnapshotDate:     "2026-06-30",
		UserAgent:        "cloudflare-ddns-token-probe",
		PauseBetweenRuns: "5s",
		RequestTimeout:   "15s",
		Reminders: []string{
			"Re-check hintRecordPermission in internal/api/cloudflare_records.go if the Zone ListZones invalid-token classification changes.",
			"Re-check hintWAFListPermission in internal/api/cloudflare_waf.go if the WAF ListLists invalid-token classification changes.",
			"Both hints fire only on AuthenticationError or AuthorizationError. If either endpoint reclassifies an invalid token (for example to RequestError), the corresponding hint silently stops firing.",
			"Malformed-token probes are the gateway RequestError regression net; the offline oauthBearerRegex check in internal/config/env_auth.go keeps that path out of normal operation.",
		},
		RelatedPaths: []string{
			"internal/api/cloudflare_records.go",
			"internal/api/cloudflare_waf.go",
			"internal/config/env_auth.go",
		},
		Probes: []probe{
			{
				Name:                 "zones invalid token",
				Kind:                 "well-formed, invalid token",
				Endpoint:             zonesURL,
				Operation:            "ListZones",
				AccountID:            "",
				Token:                invalidToken,
				IncludeAuthorization: nil,
				ExpectedRaw: expectedRaw{
					StatusCode:   403,
					Success:      false,
					ResultStatus: nil,
					Errors:       []apiError{{Code: 9109, Message: "Invalid access token", ErrorChain: nil}},
					Messages:     []apiMessageInfo{},
				},
				ExpectedSDK: &expectedSDK{
					ErrorType:    "AuthenticationError",
					ErrorCode:    9109,
					ErrorMessage: "Invalid access token (9109)",
				},
			},
			{
				Name:                 "zones malformed token",
				Kind:                 "malformed",
				Endpoint:             zonesURL,
				Operation:            "",
				AccountID:            "",
				Token:                "abc-def",
				IncludeAuthorization: nil,
				ExpectedRaw: expectedRaw{
					StatusCode:   400,
					Success:      false,
					ResultStatus: nil,
					Errors: []apiError{{
						Code:    6003,
						Message: "Invalid request headers",
						ErrorChain: []apiErrorChain{
							{Code: 6111, Message: "Invalid format for Authorization header"},
						},
					}},
					Messages: []apiMessageInfo{},
				},
				ExpectedSDK: nil,
			},
			{
				Name:                 "waf lists invalid token",
				Kind:                 "well-formed, invalid token",
				Endpoint:             wafListsURL,
				Operation:            "ListLists",
				AccountID:            placeholderAccountID,
				Token:                invalidToken,
				IncludeAuthorization: nil,
				ExpectedRaw: expectedRaw{
					StatusCode:   401,
					Success:      false,
					ResultStatus: nil,
					Errors:       []apiError{{Code: 10000, Message: "Authentication error", ErrorChain: nil}},
					Messages:     []apiMessageInfo{},
				},
				ExpectedSDK: &expectedSDK{
					ErrorType:    "AuthorizationError",
					ErrorCode:    10000,
					ErrorMessage: "Authentication error (10000)",
				},
			},
			{
				Name:                 "waf lists malformed token",
				Kind:                 "malformed",
				Endpoint:             wafListsURL,
				Operation:            "",
				AccountID:            placeholderAccountID,
				Token:                "abc-def",
				IncludeAuthorization: nil,
				ExpectedRaw: expectedRaw{
					StatusCode:   400,
					Success:      false,
					ResultStatus: nil,
					// The WAF lists endpoint rejects a malformed token through a
					// different handler than zones: it returns code 9106 with no
					// error_chain rather than the gateway's 6003/6111 pair.
					Errors:   []apiError{{Code: 9106, Message: "Authentication failed (status: 400)", ErrorChain: nil}},
					Messages: []apiMessageInfo{},
				},
				ExpectedSDK: nil,
			},
		},
	}
}
