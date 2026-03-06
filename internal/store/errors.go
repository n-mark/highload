package store

import (
	"strings"
)

// mapPgError maps postgres unique violation errors to a domain error.
// For all other errors it returns the original error.
func mapPgError(err error, conflictErr error) error {
	if err != nil && strings.Contains(err.Error(), "unique") {
		return conflictErr
	}
	return err
}
