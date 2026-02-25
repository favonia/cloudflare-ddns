package pp

// ID is the identifier of a message.
type ID int

// All the registered hints.
const (
	MessageUpdateDockerTemplate                      ID = iota // PUID or PGID was used
	MessageAuthTokenNewPrefix                                  // "CF_*" to "CLOUDFLARE_*"
	MessageIP4DetectionFails                                   // How to turn off IPv4
	MessageIP6DetectionFails                                   // How to set up IPv6 or turn it off
	MessageIP4MappedIP6Address                                 // IPv4-mapped IPv6 addresses are bad for AAAA records
	MessageDetectionTimeouts                                   // Longer detection timeout
	MessageUpdateTimeouts                                      // Longer update timeout
	MessageRecordPermission                                    // Permissions to update DNS tokens
	MessageWAFListPermission                                   // Permissions to update WAF lists
	MessageExperimentalShoutrrr                                // New feature introduced in 1.12.0 on 2024/6/28
	MessageExperimentalWAF                                     // New feature introduced in 1.14.0 on 2024/8/25
	MessageExperimentalLocalWithInterface                      // New feature introduced in 1.15.0
	MessageUndocumentedCustomCloudflareTraceProvider           // Undocumented feature
)
