// Package e36 violates E36: a credential in code.
package e36

// Connect uses a hard-coded credential.
func Connect() string {
	password := "kX9#mQ2$vL7!pR4@wT8&zN3*"
	return password
}
