//go:build !unix

package navigation

// lockNavigationFile is a no-op on platforms without flock support; writes
// fall back to the pre-locking last-writer-wins behavior.
func lockNavigationFile(string) (func(), error) {
	return func() {}, nil
}
