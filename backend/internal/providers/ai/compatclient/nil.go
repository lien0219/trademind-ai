package compatclient

import "fmt"

// ErrNilClient indicates a nil compat client.
func ErrNilClient() error {
	return fmt.Errorf("compatclient: nil client")
}
