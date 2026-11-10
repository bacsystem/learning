package auth

import "golang.org/x/crypto/bcrypt"

// The concurrency tests run several verifications at once under -race; at
// cost 12 bcrypt alone takes seconds per round on a 2-core CI runner and
// pushes them past their per-call timeouts. What they check (transaction
// atomicity, lock order) does not depend on the hashing cost.
func init() { bcryptCost = bcrypt.MinCost }
