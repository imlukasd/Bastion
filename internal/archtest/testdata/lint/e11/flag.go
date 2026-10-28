package e11

// Combine switches behavior on a boolean parameter.
func Combine(a, b int, subtract bool) int {
	if subtract {
		return a - b
	}
	return a + b
}
