package pp

// Hint is the identifier of a hint.
type Hint int

// All the registered hints.
const (
	HintUpdateDockerTemplate           Hint = iota // PUID or PGID was used
	HintAuthTokenNewPrefix                         // "CF_*" to "CLOUDFLARE_*"
	HintIP4DetectionFails                          // How to turn off IPv4
	HintIP6DetectionFails                          // How to set up IPv6 or turn it off
	HintIP4MappedIP6Address                        // IPv4-mapped IPv6 addresses are bad for AAAA records
	HintDetectionTimeouts                          // Longer detection timeout
	HintUpdateTimeouts                             // Longer update timeout
	HintRecordPermission                           // Permissions to update DNS tokens
	HintWAFListPermission                          // Permissions to update WAF lists
	HintMismatchedRecordAttributes                 // Attributes of DNS records have been changed
	HintMismatchedWAFListAttributes                // Attributes of WAF lists have been changed
	HintExperimentalShoutrrr                       // New feature introduced in 1.12.0 on 2024/6/28
	HintExperimentalWAF                            // New feature introduced in 1.14.0 on 2024/8/25
	HintExperimentalLocalWithInterface             // New feature introduced in 1.15.0
	HintDebugConstProvider                         // Undocumented feature
)
