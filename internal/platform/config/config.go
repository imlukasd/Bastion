package config

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// Lookup reads one environment variable; os.LookupEnv in cmd, a map in tests.
type Lookup func(key string) (string, bool)

// Errors of LoadPay and LoadTickets; each names the variable, never its value (E36).
var (
	ErrMissing = errors.New("config: required variable missing")
	ErrInvalid = errors.New("config: invalid value")
)

// Pay is the configuration of a Bastion Pay process (TD §33). Later items add
// their §33 variables as fields, with the §33 default.
type Pay struct {
	DBURL    string // PAY_DB_URL
	RedisURL string // BASTION_REDIS_URL
}

// Tickets is the configuration of a Bastion Tickets process (TD §33). Later
// items add their §33 variables as fields, with the §33 default.
type Tickets struct {
	DBURL    string // TIX_DB_URL
	RedisURL string // BASTION_REDIS_URL
}

// URL schemes accepted for each kind of variable.
func dbSchemes() []string    { return []string{"postgres", "postgresql"} }
func redisSchemes() []string { return []string{"redis", "rediss"} }

// LoadPay reads PAY_DB_URL and BASTION_REDIS_URL. Every problem is reported,
// joined into one error.
func LoadPay(lookup Lookup) (Pay, error) {
	var r reader
	c := Pay{
		DBURL:    r.url(lookup, "PAY_DB_URL", dbSchemes()),
		RedisURL: r.url(lookup, "BASTION_REDIS_URL", redisSchemes()),
	}
	if err := errors.Join(r.errs...); err != nil {
		return Pay{}, err
	}
	return c, nil
}

// LoadTickets reads TIX_DB_URL and BASTION_REDIS_URL. Every problem is
// reported, joined into one error.
func LoadTickets(lookup Lookup) (Tickets, error) {
	var r reader
	c := Tickets{
		DBURL:    r.url(lookup, "TIX_DB_URL", dbSchemes()),
		RedisURL: r.url(lookup, "BASTION_REDIS_URL", redisSchemes()),
	}
	if err := errors.Join(r.errs...); err != nil {
		return Tickets{}, err
	}
	return c, nil
}

// reader collects the errors of one Load call.
type reader struct {
	errs []error
}

// url returns the required variable name as a URL with one of schemes and a
// host. The URL may carry a password, so no error repeats the value or the
// parser's message, which quotes it (E36).
func (r *reader) url(lookup Lookup, name string, schemes []string) string {
	v, ok := lookup(name)
	if !ok || v == "" {
		r.errs = append(r.errs, fmt.Errorf("%w: %s", ErrMissing, name))
		return ""
	}
	u, err := url.Parse(v)
	switch {
	case err != nil:
		r.errs = append(r.errs, fmt.Errorf("%w: %s is not a URL", ErrInvalid, name))
	case !slices.Contains(schemes, u.Scheme):
		r.errs = append(r.errs, fmt.Errorf("%w: %s scheme must be %s", ErrInvalid, name, strings.Join(schemes, " or ")))
	case u.Hostname() == "":
		r.errs = append(r.errs, fmt.Errorf("%w: %s has no host", ErrInvalid, name))
	default:
		return v
	}
	return ""
}
