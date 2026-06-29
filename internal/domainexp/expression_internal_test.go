// vim: nowrap

package domainexp

import (
	"io"
	"testing"
)

// TestInvalidDomainErrorError pins invalidDomainError.Error(). The method exists
// only to satisfy the error interface, so the value can be stored in
// syntax.ParseError.Cause and classified by errors.AsType. It is never invoked in
// production: reportExpressionError matches the type and reads the domain/cause
// fields directly rather than the error string. This unit test exercises the
// delegation to the wrapped cause that production code bypasses.
func TestInvalidDomainErrorError(t *testing.T) {
	t.Parallel()
	cause := io.ErrUnexpectedEOF // a static sentinel error to delegate to
	err := &invalidDomainError{domain: "b.*.a.org", cause: cause}
	if got := err.Error(); got != cause.Error() {
		t.Errorf("Error() = %q, want %q", got, cause.Error())
	}
}
