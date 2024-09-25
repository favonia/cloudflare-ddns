package pp

// Hint is the identifier of a hint.
type Hint int

// All the registered hints.
const (
	HintUpdateDockerTemplate Hint = iota
	HintIP4DetectionFails
	HintIP6DetectionFails
	HintIP4MappedIP6Address
	HintDetectionTimeouts
	HintUpdateTimeouts
	HintRecordPermission
	HintWAFListPermission
	HintMismatchedRecordAttributes
	HintMismatchedWAFListAttributes
	Hint1111Blockage
	HintExperimentalShoutrrr           // introduced in 1.12.0
	HintExperimentalWAF                // introduced in 1.14.0
	HintExperimentalLocalWithInterface // introduced in 1.15.0
)
