//go:build !(linux || darwin)

package embedded

// Other platforms run without a store lock; deploy a single embedded broker per directory.
func lockStore(string) (func(), error) {
	return func() {}, nil
}
