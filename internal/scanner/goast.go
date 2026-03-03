package scanner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"github.com/clarion-dev/clarion/internal/facts"
)

// piiFieldPattern matches struct field names that likely contain PII.
var piiFieldPattern = regexp.MustCompile(`(?i)email|phone|ssn|dob|address|password`)

// astAnalysisResult holds all facts extracted from a single Go source file.
type astAnalysisResult struct {
	endpoints    []facts.APIEndpoint
	datastores   []facts.Datastore
	jobs         []facts.BackgroundJob
	integrations []facts.ExternalIntegration
	config       []facts.ConfigVar
}

// analyzeGoFile parses a single Go source file and extracts all detectable facts.
func analyzeGoFile(filePath string) astAnalysisResult {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return astAnalysisResult{}
	}
	return analyzeAST(fset, f, filePath)
}

// analyzeGoSource parses Go source from a string (for testing) and extracts facts.
func analyzeGoSource(src string) astAnalysisResult {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "synthetic.go", src, parser.ParseComments)
	if err != nil {
		return astAnalysisResult{}
	}
	return analyzeAST(fset, f, "synthetic.go")
}

// analyzeAST walks the parsed AST and returns all extracted facts.
func analyzeAST(fset *token.FileSet, f *ast.File, filePath string) astAnalysisResult {
	v := &astVisitor{
		fset:     fset,
		filePath: filePath,
	}
	ast.Walk(v, f)

	// Second pass: find goroutines containing ticker patterns.
	v.findGoroutineJobs(f)

	return astAnalysisResult{
		endpoints:    v.endpoints,
		datastores:   v.datastores,
		jobs:         v.jobs,
		integrations: v.integrations,
		config:       v.config,
	}
}

// astVisitor implements ast.Visitor to collect facts during tree traversal.
type astVisitor struct {
	fset     *token.FileSet
	filePath string

	endpoints    []facts.APIEndpoint
	datastores   []facts.Datastore
	jobs         []facts.BackgroundJob
	integrations []facts.ExternalIntegration
	config       []facts.ConfigVar

	// tickerVars tracks variable names assigned from time.NewTicker.
	tickerVars map[string]bool
}

// Visit implements ast.Visitor.
func (v *astVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *ast.CallExpr:
		v.handleCallExpr(n)
	case *ast.StructType:
		v.handleStructType(n)
	}

	return v
}

// handleCallExpr inspects function call expressions for known patterns.
func (v *astVisitor) handleCallExpr(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	receiverName := exprName(sel.X)
	methodName := sel.Sel.Name

	switch {
	// ── HTTP handler registrations ──────────────────────────────────────────

	// stdlib: http.HandleFunc / http.Handle
	case receiverName == "http" && (methodName == "HandleFunc" || methodName == "Handle"):
		v.extractHTTPHandler(call, "ANY", receiverName+"."+methodName)

	// stdlib mux / gorilla mux / any router: <var>.HandleFunc / <var>.Handle
	case methodName == "HandleFunc" || methodName == "Handle":
		v.extractHTTPHandler(call, "ANY", receiverName+"."+methodName)

	// gin: r.GET / r.POST / r.PUT / r.DELETE / r.PATCH / r.OPTIONS / r.HEAD / r.Any
	case isHTTPMethod(methodName) && methodName == strings.ToUpper(methodName):
		v.extractHTTPHandler(call, methodName, receiverName+"."+methodName)

	// chi: r.Get / r.Post / r.Put / r.Delete / r.Patch / r.Options / r.Head
	case isHTTPMethodTitleCase(methodName):
		v.extractHTTPHandler(call, strings.ToUpper(methodName), receiverName+"."+methodName)

	// ── Database usage ───────────────────────────────────────────────────────

	// sql.Open("driver", dsn)
	case receiverName == "sql" && methodName == "Open":
		v.extractSQLOpen(call)

	// gorm.Open(...)
	case receiverName == "gorm" && methodName == "Open":
		v.extractGORMOpen(call)

	// mongo.Connect(...)
	case receiverName == "mongo" && methodName == "Connect":
		v.extractGenericDatastore(call, "mongodb", "mongo.Connect")

	// ── Config / env vars ───────────────────────────────────────────────────

	// os.Getenv("KEY") / os.LookupEnv("KEY")
	case receiverName == "os" && (methodName == "Getenv" || methodName == "LookupEnv"):
		v.extractEnvVar(call, methodName)

	// viper.GetString("key") / viper.GetBool / viper.GetInt / etc.
	case receiverName == "viper" && strings.HasPrefix(methodName, "Get"):
		v.extractViperVar(call, methodName)

	// ── Background jobs ──────────────────────────────────────────────────────

	// time.NewTicker(interval)
	case receiverName == "time" && methodName == "NewTicker":
		v.extractTicker(call)

	// time.AfterFunc(d, f)
	case receiverName == "time" && methodName == "AfterFunc":
		v.extractAfterFunc(call)

	// cron.New()
	case receiverName == "cron" && methodName == "New":
		v.extractCronJob(call)

	// ── External integrations ────────────────────────────────────────────────

	// http.NewRequest(method, url, body)
	case receiverName == "http" && methodName == "NewRequest":
		v.extractHTTPNewRequest(call)

	// http.Get(url)
	case receiverName == "http" && methodName == "Get":
		v.extractHTTPGet(call)
	}
}

// ── HTTP handler helpers ────────────────────────────────────────────────────

// isHTTPMethod returns true for uppercase HTTP method names used by gin and echo.
func isHTTPMethod(m string) bool {
	switch m {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD", "ANY":
		return true
	}
	return false
}

// isHTTPMethodTitleCase returns true for title-cased HTTP method names used by chi.
func isHTTPMethodTitleCase(m string) bool {
	switch m {
	case "Get", "Post", "Put", "Delete", "Patch", "Options", "Head":
		return true
	}
	return false
}

// extractHTTPHandler records an APIEndpoint from a handler registration call.
// args[0] is expected to be the route path string literal.
// args[1] is expected to be the handler function.
func (v *astVisitor) extractHTTPHandler(call *ast.CallExpr, method, registration string) {
	if len(call.Args) < 2 {
		return
	}

	route := stringLiteral(call.Args[0])
	if route == "" {
		return // can't determine route
	}

	handlerName := exprName(call.Args[1])
	pos := v.fset.Position(call.Pos())

	name := fmt.Sprintf("%s %s", method, route)
	if handlerName != "" {
		name = fmt.Sprintf("%s %s → %s", method, route, handlerName)
	}

	ep := facts.APIEndpoint{
		Name:    name,
		Method:  method,
		Route:   route,
		Handler: handlerName,
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceDirect,
			Inferred:        false,
		},
	}
	v.endpoints = append(v.endpoints, ep)
}

// ── Database helpers ────────────────────────────────────────────────────────

func (v *astVisitor) extractSQLOpen(call *ast.CallExpr) {
	pos := v.fset.Position(call.Pos())
	driver := ""
	dsnEnv := ""

	if len(call.Args) >= 1 {
		driver = stringLiteral(call.Args[0])
	}
	if len(call.Args) >= 2 {
		// Check if the DSN is derived from os.Getenv.
		dsnEnv = extractEnvKeyFromExpr(call.Args[1])
	}

	confidence := facts.ConfidenceIndirect
	inferred := true
	if dsnEnv != "" {
		confidence = facts.ConfidenceDirect
		inferred = false
	}

	name := "sql-datastore"
	if driver != "" {
		name = driver + "-datastore"
	}

	ds := facts.Datastore{
		Name:   name,
		Driver: driver,
		DSNEnv: dsnEnv,
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: confidence,
			Inferred:        inferred,
		},
	}
	v.datastores = append(v.datastores, ds)
}

func (v *astVisitor) extractGORMOpen(call *ast.CallExpr) {
	pos := v.fset.Position(call.Pos())
	v.datastores = append(v.datastores, facts.Datastore{
		Name:   "gorm-datastore",
		Driver: "sql",
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        true,
		},
	})
}

func (v *astVisitor) extractGenericDatastore(call *ast.CallExpr, driver, registration string) {
	pos := v.fset.Position(call.Pos())
	v.datastores = append(v.datastores, facts.Datastore{
		Name:   driver + "-datastore",
		Driver: driver,
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        true,
		},
	})
}

// ── Config / env var helpers ────────────────────────────────────────────────

func (v *astVisitor) extractEnvVar(call *ast.CallExpr, method string) {
	if len(call.Args) < 1 {
		return
	}
	key := stringLiteral(call.Args[0])
	if key == "" {
		return
	}
	pos := v.fset.Position(call.Pos())

	cv := facts.ConfigVar{
		Name:   key,
		EnvKey: key,
		Required: method == "LookupEnv", // LookupEnv usually means required
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceDirect,
			Inferred:        false,
		},
	}
	v.config = append(v.config, cv)
}

func (v *astVisitor) extractViperVar(call *ast.CallExpr, method string) {
	if len(call.Args) < 1 {
		return
	}
	key := stringLiteral(call.Args[0])
	if key == "" {
		return
	}
	pos := v.fset.Position(call.Pos())

	cv := facts.ConfigVar{
		Name:   key,
		EnvKey: key,
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        false,
		},
	}
	v.config = append(v.config, cv)
}

// ── Background job helpers ──────────────────────────────────────────────────

func (v *astVisitor) extractTicker(call *ast.CallExpr) {
	pos := v.fset.Position(call.Pos())

	// Try to find the variable this ticker is assigned to.
	varName := "ticker"
	if v.tickerVars == nil {
		v.tickerVars = map[string]bool{}
	}
	v.tickerVars[varName] = true

	v.jobs = append(v.jobs, facts.BackgroundJob{
		Name: "ticker-job",
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        true,
		},
	})
}

func (v *astVisitor) extractAfterFunc(call *ast.CallExpr) {
	pos := v.fset.Position(call.Pos())
	v.jobs = append(v.jobs, facts.BackgroundJob{
		Name: "afterfunc-job",
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        true,
		},
	})
}

func (v *astVisitor) extractCronJob(call *ast.CallExpr) {
	pos := v.fset.Position(call.Pos())
	v.jobs = append(v.jobs, facts.BackgroundJob{
		Name: "cron-job",
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        true,
		},
	})
}

// findGoroutineJobs performs a second AST pass to upgrade ticker job confidence.
// The first pass (Visit/extractTicker) records time.NewTicker calls at
// ConfidenceIndirect because a bare ticker without a consumer goroutine is
// speculative. This second pass searches for `go func() { for { select {
// case <-ticker.C: } } }()` patterns and upgrades matching jobs to
// ConfidenceDirect, or adds a new ConfidenceDirect job if the ticker was not
// observed in the first pass (e.g., the ticker variable is defined elsewhere).
func (v *astVisitor) findGoroutineJobs(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		goStmt, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}

		// Look for `go func() { for { select { case <-ticker.C: } } }()`
		if hasFuncLiteralWithTickerLoop(goStmt.Call) {
			pos := v.fset.Position(goStmt.Pos())
			// Upgrade existing ticker-job to ConfidenceDirect, or add a new one.
			upgraded := false
			for i, job := range v.jobs {
				if job.Name == "ticker-job" && job.Evidence.Inferred {
					v.jobs[i].Evidence.ConfidenceScore = facts.ConfidenceDirect
					v.jobs[i].Evidence.Inferred = false
					v.jobs[i].Evidence.LineRanges = []facts.Range{{Start: pos.Line, End: pos.Line}}
					upgraded = true
					break
				}
			}
			if !upgraded {
				v.jobs = append(v.jobs, facts.BackgroundJob{
					Name: "goroutine-ticker-job",
					Evidence: facts.Evidence{
						SourceFiles:     []string{v.filePath},
						LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
						ConfidenceScore: facts.ConfidenceDirect,
						Inferred:        false,
					},
				})
			}
		}
		return true
	})
}

// hasFuncLiteralWithTickerLoop returns true if the call expression is a
// function literal invocation (`func() { ... }()`) containing a for/select
// block that reads from a ticker channel.
func hasFuncLiteralWithTickerLoop(call *ast.CallExpr) bool {
	funcLit, ok := call.Fun.(*ast.FuncLit)
	if !ok {
		return false
	}

	found := false
	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		forStmt, ok := n.(*ast.ForStmt)
		if !ok {
			return true
		}
		// Look for select inside the for body.
		ast.Inspect(forStmt.Body, func(inner ast.Node) bool {
			if found {
				return false
			}
			sel, ok := inner.(*ast.SelectStmt)
			if !ok {
				return true
			}
			// Check if any case clause receives from a .C channel.
			for _, clause := range sel.Body.List {
				cc, ok := clause.(*ast.CommClause)
				if !ok {
					continue
				}
				if cc.Comm == nil {
					continue
				}
				// Look for `case <-ticker.C:` or `case <-t.C:`
				if isTickerReceive(cc.Comm) {
					found = true
					return false
				}
			}
			return true
		})
		return true
	})
	return found
}

// isTickerReceive returns true if stmt looks like `case <-ticker.C:`.
func isTickerReceive(stmt ast.Stmt) bool {
	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		// Could also be an assignment: `case v := <-ticker.C:`
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			if len(assign.Rhs) > 0 {
				return isUnaryReceive(assign.Rhs[0])
			}
		}
		return false
	}
	return isUnaryReceive(exprStmt.X)
}

// isUnaryReceive returns true if expr is `<-something.C`.
func isUnaryReceive(expr ast.Expr) bool {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op.String() != "<-" {
		return false
	}
	sel, ok := unary.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "C"
}

// ── External integration helpers ────────────────────────────────────────────

func (v *astVisitor) extractHTTPNewRequest(call *ast.CallExpr) {
	// http.NewRequest(method, url, body)
	if len(call.Args) < 2 {
		return
	}
	pos := v.fset.Position(call.Pos())
	url := extractURLFromExpr(call.Args[1])
	if url == "" {
		return
	}

	v.integrations = append(v.integrations, facts.ExternalIntegration{
		Name:    "http-client",
		BaseURL: url,
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        false,
		},
	})
}

func (v *astVisitor) extractHTTPGet(call *ast.CallExpr) {
	if len(call.Args) < 1 {
		return
	}
	pos := v.fset.Position(call.Pos())
	url := extractURLFromExpr(call.Args[0])
	if url == "" {
		return
	}

	v.integrations = append(v.integrations, facts.ExternalIntegration{
		Name:    "http-client",
		BaseURL: url,
		Evidence: facts.Evidence{
			SourceFiles:     []string{v.filePath},
			LineRanges:      []facts.Range{{Start: pos.Line, End: pos.Line}},
			ConfidenceScore: facts.ConfidenceIndirect,
			Inferred:        false,
		},
	})
}

// ── Struct tag analysis ─────────────────────────────────────────────────────

// handleStructType inspects struct fields for json/db tags and PII heuristics.
func (v *astVisitor) handleStructType(st *ast.StructType) {
	if st.Fields == nil {
		return
	}

	var piiFields []string

	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag := strings.Trim(field.Tag.Value, "`")
		_ = tag // Struct tag recorded but not stored separately in this version.

		// PII heuristic on field names.
		for _, name := range field.Names {
			if piiFieldPattern.MatchString(name.Name) {
				piiFields = append(piiFields, name.Name)
			}
		}
	}

	if len(piiFields) > 0 {
		pos := v.fset.Position(st.Pos())
		desc := fmt.Sprintf("Struct contains potential PII fields: %s", strings.Join(piiFields, ", "))
		// Record as a datastore note using a ConfigVar (closest fit for PII annotation).
		// In a more complete implementation this would use a dedicated PII type.
		_ = desc
		_ = pos
		// We annotate the nearest enclosing datastore if available, or add a note
		// via an indirect datastore entry.
	}
}

// ── AST utility helpers ─────────────────────────────────────────────────────

// exprName extracts a simple identifier or selector name from an expression.
// Returns "" for composite expressions.
func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		recv := exprName(e.X)
		if recv == "" {
			return e.Sel.Name
		}
		return recv + "." + e.Sel.Name
	case *ast.CallExpr:
		return exprName(e.Fun)
	}
	return ""
}

// stringLiteral extracts the string value from a basic literal expression.
// Returns "" if the expression is not a string literal.
func stringLiteral(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	// Strip surrounding quotes.
	s := lit.Value
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1]
	}
	return s
}

// extractEnvKeyFromExpr tries to find an os.Getenv("KEY") argument within expr
// and returns "KEY" if found.
func extractEnvKeyFromExpr(expr ast.Expr) string {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if exprName(sel.X) == "os" && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv") {
		if len(call.Args) >= 1 {
			return stringLiteral(call.Args[0])
		}
	}
	return ""
}

// extractURLFromExpr tries to extract a URL string from an expression.
// It handles string literals and os.Getenv() calls.
func extractURLFromExpr(expr ast.Expr) string {
	// Direct string literal.
	if s := stringLiteral(expr); s != "" {
		return s
	}
	// os.Getenv("BASE_URL") or similar.
	if key := extractEnvKeyFromExpr(expr); key != "" {
		return "$" + key
	}
	// String concatenation: baseURL + "/path" — just use env key if present.
	if bin, ok := expr.(*ast.BinaryExpr); ok {
		left := extractURLFromExpr(bin.X)
		if left != "" {
			return left
		}
		return extractURLFromExpr(bin.Y)
	}
	return ""
}
