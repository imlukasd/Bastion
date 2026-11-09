package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/imlukasd/Bastion/internal/platform/config"
)

const (
	secret   = "s3cr3t-pw"
	payDB    = "postgres://pay_app:" + secret + "@127.0.0.1:55432/bastion_pay?sslmode=disable"
	ticketDB = "postgresql://tickets_app:" + secret + "@db.internal/bastion_tickets"
	redisURL = "redis://:" + secret + "@127.0.0.1:6379/0"
)

func env(m map[string]string) config.Lookup {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

func without(m map[string]string, key string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func with(m map[string]string, key, value string) map[string]string {
	out := without(m, key)
	out[key] = value
	return out
}

// requireErr checks err wraps want, names every variable and leaks no secret (E36).
func requireErr(t *testing.T, err, want error, names ...string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	for _, n := range names {
		if !strings.Contains(err.Error(), n) {
			t.Errorf("error %q does not name %s", err, n)
		}
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q contains the secret", err)
	}
}

func TestLoadPay(t *testing.T) {
	full := map[string]string{"PAY_DB_URL": payDB, "BASTION_REDIS_URL": redisURL}
	got, err := config.LoadPay(env(full))
	if err != nil {
		t.Fatalf("LoadPay: %v", err)
	}
	if want := (config.Pay{DBURL: payDB, RedisURL: redisURL}); got != want {
		t.Fatalf("LoadPay = %+v, want %+v", got, want)
	}
	for _, name := range []string{"PAY_DB_URL", "BASTION_REDIS_URL"} {
		_, err := config.LoadPay(env(without(full, name)))
		requireErr(t, err, config.ErrMissing, name)
		_, err = config.LoadPay(env(with(full, name, "")))
		requireErr(t, err, config.ErrMissing, name)
	}
	_, err = config.LoadPay(env(nil))
	requireErr(t, err, config.ErrMissing, "PAY_DB_URL", "BASTION_REDIS_URL")
}

func TestLoadTickets(t *testing.T) {
	full := map[string]string{"TIX_DB_URL": ticketDB, "BASTION_REDIS_URL": redisURL}
	got, err := config.LoadTickets(env(full))
	if err != nil {
		t.Fatalf("LoadTickets: %v", err)
	}
	if want := (config.Tickets{DBURL: ticketDB, RedisURL: redisURL}); got != want {
		t.Fatalf("LoadTickets = %+v, want %+v", got, want)
	}
	for _, name := range []string{"TIX_DB_URL", "BASTION_REDIS_URL"} {
		_, err := config.LoadTickets(env(without(full, name)))
		requireErr(t, err, config.ErrMissing, name)
		_, err = config.LoadTickets(env(with(full, name, "")))
		requireErr(t, err, config.ErrMissing, name)
	}
	_, err = config.LoadTickets(env(nil))
	requireErr(t, err, config.ErrMissing, "TIX_DB_URL", "BASTION_REDIS_URL")
}

// D7: a Pay process needs no Tickets variable, and the other way round.
func TestLoad_IgnoresTheOtherProduct(t *testing.T) {
	if _, err := config.LoadPay(env(map[string]string{"PAY_DB_URL": payDB, "BASTION_REDIS_URL": redisURL})); err != nil {
		t.Errorf("LoadPay without TIX_DB_URL: %v", err)
	}
	if _, err := config.LoadTickets(env(map[string]string{"TIX_DB_URL": ticketDB, "BASTION_REDIS_URL": "rediss://cache.internal:6380"})); err != nil {
		t.Errorf("LoadTickets without PAY_DB_URL: %v", err)
	}
}

func TestLoad_RejectsInvalidURLs(t *testing.T) {
	full := map[string]string{"PAY_DB_URL": payDB, "BASTION_REDIS_URL": redisURL}
	invalid := []struct{ name, value string }{
		{"PAY_DB_URL", "mysql://u:" + secret + "@db/bastion_pay"},
		{"PAY_DB_URL", "postgres://u:" + secret + "@/bastion_pay"},
		{"PAY_DB_URL", "postgres://u:" + secret + "@[::1/bastion_pay"},
		{"PAY_DB_URL", "host=db user=pay_app password=" + secret},
		{"BASTION_REDIS_URL", "http://:" + secret + "@127.0.0.1:6379"},
		{"BASTION_REDIS_URL", "redis://:" + secret + "@"},
	}
	for _, c := range invalid {
		_, err := config.LoadPay(env(with(full, c.name, c.value)))
		requireErr(t, err, config.ErrInvalid, c.name)
	}
	both := with(with(full, "PAY_DB_URL", "mysql://db"), "BASTION_REDIS_URL", "http://cache")
	_, err := config.LoadPay(env(both))
	requireErr(t, err, config.ErrInvalid, "PAY_DB_URL", "BASTION_REDIS_URL")
}
