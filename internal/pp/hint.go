package pp

// Hint is the identifier of a hint.
type Hint int

// All the registered hints.
const (
	HintUpdateDockerTemplate Hint = iota
	HintIP4DetectionFails
	HintIP6DetectionFails
	HintDetectionTimeouts
	HintUpdateTimeouts
	HintCloudflareWAFPermissions
	Hint1111Blockage
)
