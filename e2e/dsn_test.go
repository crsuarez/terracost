package e2e

import "os"

// testDSN returns the MySQL DSN for e2e tests. It honors the
// TERRACOST_DSN environment variable so the same test suite can run
// against either the bundled Docker MySQL (default) or an external
// database without modifying source.
func testDSN() string {
	if v := os.Getenv("TERRACOST_DSN"); v != "" {
		return v
	}
	return "root:terracost@tcp(172.44.0.2:3306)/terracost_test?multiStatements=true"
}
