package optional

// Get the value of a pointer or return a default value
func GetOrDefault[T any](value *T, defaultValue T) T {
	if value != nil {
		return *value
	}

	return defaultValue
}
