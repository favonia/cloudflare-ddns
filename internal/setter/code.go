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

	// ResponseUpdating means records should be updated
	// and we started the updating asynchronously, or that
	// they should be deleted and we started the deletion
	// asynchronously.
	ResponseUpdating

	// ResponseFailed means records should be updated
	// but we failed to finish the updating, or that they
	// should be deleted and we failed to finish the deletion.
	ResponseFailed
)
