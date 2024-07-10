package setter

// ResponseCode encodes the minimum information to generate messages for monitors and notifiers.
type ResponseCode int

const (
	// ResponseNoop means no changes were needed.
	// The records were already updated or already deleted.
	ResponseNoop ResponseCode = iota

	// ResponseUpdated means records should be updated
	// and we updated them, or that they should be deleted
	// and we deleted them.
	ResponseUpdated

	// ResponseFailed means records should be updated
	// but we failed to finish the updating, or that they
	// should be deleted and we failed to finish the deletion.
	ResponseFailed

	// ResponseSanityFailed means sanity check failed and
	// there is no point in performing actual operations.
	ResponseSanityFailed
)
