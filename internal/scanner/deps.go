package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/clarion-dev/clarion/internal/facts"
)

// modInfo holds parsed information from a go.mod file.
type modInfo struct {
	moduleName   string
	datastores   []facts.Datastore
	integrations []facts.ExternalIntegration
	frameworks   []string // detected HTTP framework names
}

// knownDBDrivers maps go.mod import path prefixes to concrete Datastore drivers.
// Only include packages that unambiguously identify a specific database engine.
// ORM/query-builder packages (gorm.io/gorm, sqlx) are intentionally excluded
// because they do not identify which underlying database is in use; the specific
// driver package (e.g. gorm.io/driver/postgres) provides that information.
var knownDBDrivers = map[string]string{
	"github.com/lib/pq":              "postgres",
	"github.com/go-sql-driver/mysql": "mysql",
	"github.com/mattn/go-sqlite3":    "sqlite3",
	"go.mongodb.org/mongo-driver":    "mongodb",
	"github.com/jackc/pgx":           "postgres",
	"github.com/jackc/pgconn":        "postgres",
	"gorm.io/driver/postgres":        "postgres",
	"gorm.io/driver/mysql":           "mysql",
	"gorm.io/driver/sqlite":          "sqlite3",
}

// knownHTTPFrameworks maps import path prefixes to framework names.
var knownHTTPFrameworks = map[string]string{
	"github.com/gin-gonic/gin":  "gin",
	"github.com/go-chi/chi":     "chi",
	"github.com/labstack/echo":  "echo",
	"github.com/gorilla/mux":    "gorilla",
	"github.com/valyala/fasthttp": "fasthttp",
}

// parseGoMod reads and parses a go.mod file, returning detected module
// metadata, database drivers, and HTTP frameworks. Returns nil if the
// file doesn't exist or cannot be parsed.
func parseGoMod(path string) (*modInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	f, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, err
	}

	info := &modInfo{
		moduleName: f.Module.Mod.Path,
	}

	goModFile := path
	goModDir := filepath.Dir(path)
	_ = goModDir

	for _, req := range f.Require {
		importPath := req.Mod.Path

		// Check for database drivers.
		for prefix, driver := range knownDBDrivers {
			if strings.HasPrefix(importPath, prefix) {
				name := driver + "-datastore"
				info.datastores = append(info.datastores, facts.Datastore{
					Name:   name,
					Driver: driver,
					Evidence: facts.Evidence{
						SourceFiles:     []string{goModFile},
						ConfidenceScore: facts.ConfidenceDirect,
						Inferred:        false,
					},
				})
				break
			}
		}

		// Check for HTTP frameworks.
		for prefix, framework := range knownHTTPFrameworks {
			if strings.HasPrefix(importPath, prefix) {
				info.frameworks = append(info.frameworks, framework)
				break
			}
		}

		// Record all direct (non-indirect) dependencies as potential integrations.
		if !req.Indirect {
			// Only record external (non-stdlib) packages as integrations if
			// they look like external services (heuristic: has >= 2 path segments
			// and is not a well-known Go tooling package).
			if looksLikeExternalService(importPath) {
				info.integrations = append(info.integrations, facts.ExternalIntegration{
					Name: importPath,
					Evidence: facts.Evidence{
						SourceFiles:     []string{goModFile},
						ConfidenceScore: facts.ConfidenceDirect,
						Inferred:        false,
					},
				})
			}
		}
	}

	return info, nil
}

// looksLikeExternalService returns true for import paths that represent
// external service SDKs (cloud providers, comms services, etc.) rather
// than generic Go libraries.
func looksLikeExternalService(importPath string) bool {
	serviceIndicators := []string{
		"aws.amazon.com",
		"cloud.google.com",
		"azure.com",
		"stripe.com",
		"twilio.com",
		"sendgrid.com",
		"mailgun.com",
		"firebase.google.com",
		"datadog",
		"newrelic",
		"segment.com",
		"sentry.io",
		"github.com/aws/aws-sdk-go",
		"github.com/azure/azure-sdk-for-go",
		"google.golang.org/api",
		"cloud.google.com/go",
	}
	for _, indicator := range serviceIndicators {
		if strings.Contains(importPath, indicator) {
			return true
		}
	}
	return false
}
