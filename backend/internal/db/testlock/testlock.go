// Package testlock holds the key of the session-level advisory lock that
// integration tests take while they create rows other packages' tests
// assert on globally (e.g. the number of active admins). It is a leaf
// package so every test can import it, including internal tests of
// packages that internal/db/dbtest itself imports (auth).
package testlock

// Key is the pg_advisory_lock key shared by dbtest.Open, the auth test
// fixtures and the seed/admin tests: 0x6c6561726e, "learn" in ASCII.
const Key int64 = 0x6c6561726e
