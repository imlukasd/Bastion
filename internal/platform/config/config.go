package config

import "errors"

// Lookup reads one environment variable; os.LookupEnv in cmd, a map in tests.
type Lookup func(key string) (string, bool)

// Errors of LoadPay and LoadTickets; each names the variable, never its value (E36).
var (
	ErrMissing = errors.New("config: required variable missing")
	ErrInvalid = errors.New("config: invalid value")
)

// Pay is the configuration of a Bastion Pay process (TD §33).
type Pay struct {
	DBURL    string // PAY_DB_URL
	RedisURL string // BASTION_REDIS_URL
}

// Tickets is the configuration of a Bastion Tickets process (TD §33).
type Tickets struct {
	DBURL    string // TIX_DB_URL
	RedisURL string // BASTION_REDIS_URL
}

// errNotImplemented marks the P1-03 G2 stubs; step 3 replaces them.
var errNotImplemented = errors.New("config: not implemented")

// LoadPay reads PAY_DB_URL and BASTION_REDIS_URL.
func LoadPay(lookup Lookup) (Pay, error) {
	_ = lookup
	return Pay{}, errNotImplemented
}

// LoadTickets reads TIX_DB_URL and BASTION_REDIS_URL.
func LoadTickets(lookup Lookup) (Tickets, error) {
	_ = lookup
	return Tickets{}, errNotImplemented
}
