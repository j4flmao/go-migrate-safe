// Package studio provides a Prisma-Studio-like web UI to browse the
// connected database. It is launched via `gms studio`.
package studio

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

// Server is a minimal HTTP server that exposes a JSON API plus a single
// embedded HTML page that mimics Prisma Studio.
type Server struct {
	db          *sql.DB
	driverName  string // "postgres" | "mysql" | "sqlite" | "mssql"
	schema      string
	driver      migrate.Driver
	openBrowser bool
}

// Options configures a Studio server.
type Options struct {
	DB          *sql.DB
	DriverName  string
	Schema      string
	Driver      migrate.Driver
	OpenBrowser bool
}

// New constructs a new Studio server. db must be non-nil and must be the
// underlying *sql.DB for the connected provider. Mongo / memory drivers are
// not supported.
func New(opts Options) (*Server, error) {
	if opts.DB == nil {
		return nil, fmt.Errorf("studio: nil *sql.DB (driver %q is not supported)", opts.DriverName)
	}
	switch opts.DriverName {
	case "postgres", "mysql", "sqlite", "mssql":
	default:
		return nil, fmt.Errorf("studio: unsupported driver %q", opts.DriverName)
	}
	if opts.Schema == "" {
		opts.Schema = defaultSchema(opts.DriverName)
	}
	return &Server{
		db:          opts.DB,
		driverName:  opts.DriverName,
		schema:      opts.Schema,
		driver:      opts.Driver,
		openBrowser: opts.OpenBrowser,
	}, nil
}

func defaultSchema(driver string) string {
	switch driver {
	case "postgres":
		return "public"
	case "mssql":
		return "dbo"
	default:
		return ""
	}
}

// Listen starts the HTTP server on addr (e.g. ":4488") and blocks.
// If the port is busy, it automatically tries the next port (up to 100 attempts).
func (s *Server) Listen(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/tables", s.handleTables)
	mux.HandleFunc("/api/table/", s.handleTableAPI)

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	basePort, _ := strconv.Atoi(portStr)
	if basePort == 0 {
		basePort = 4488
	}

	for i := 0; i < 100; i++ {
		listenAddr := fmt.Sprintf("%s:%d", host, basePort+i)
		ln, err := net.Listen("tcp", listenAddr)
		if err == nil {
			url := fmt.Sprintf("http://%s", ln.Addr().String())
			log.Printf("gms studio is running at %s", url)
			log.Printf("  driver=%s schema=%s", s.driverName, s.schema)
			if s.openBrowser {
				go openBrowser(url)
			}
			return http.Serve(ln, mux)
		}
	}
	return fmt.Errorf("studio: could not find a free port after 100 attempts")
}

// ─────────────────────────── HANDLERS ───────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dbVersion := ""
	if s.driver != nil {
		if v, err := s.driver.DatabaseVersion(ctx); err == nil {
			dbVersion = v
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"driver":  s.driverName,
		"schema":  s.schema,
		"version": dbVersion,
	})
}

type tableInfo struct {
	Name     string `json:"name"`
	RowCount int64  `json:"rowCount"`
}

func (s *Server) handleTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sm, err := s.driver.ReadSchema(ctx, s.schema)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read schema: "+err.Error())
		return
	}
	out := make([]tableInfo, 0, len(sm.Tables))
	for name := range sm.Tables {
		// internal history table is interesting too but tuck it last
		count, _ := s.countRows(ctx, name)
		out = append(out, tableInfo{Name: name, RowCount: count})
	}
	sort.Slice(out, func(i, j int) bool {
		// push internal table to end
		if strings.HasPrefix(out[i].Name, "_") != strings.HasPrefix(out[j].Name, "_") {
			return !strings.HasPrefix(out[i].Name, "_")
		}
		return out[i].Name < out[j].Name
	})
	writeJSON(w, http.StatusOK, out)
}

type columnDef struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Nullable      bool     `json:"nullable"`
	IsPK          bool     `json:"isPK"`
	AutoIncrement bool     `json:"autoIncrement"`
	EnumValues    []string `json:"enumValues,omitempty"`
}

type tableData struct {
	Name        string                              `json:"name"`
	Columns     []columnDef                         `json:"columns"`
	Rows        [][]any                             `json:"rows"`
	Total       int64                               `json:"total"`
	Limit       int                                 `json:"limit"`
	Offset      int                                 `json:"offset"`
	Took        string                              `json:"took"`
	Indexes     []indexSummary                      `json:"indexes"`
	Constraints map[string]*migrate.ConstraintModel `json:"constraints,omitempty"`
}

type indexSummary struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
}

func (s *Server) handleTableAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/table/")
	parts := strings.SplitN(path, "/", 2)
	name := parts[0]

	if name == "" || strings.ContainsAny(name, "?\\") {
		writeError(w, http.StatusBadRequest, "invalid table name")
		return
	}

	switch r.Method {
	case "GET":
		s.handleTableData(w, r, name)
	case "POST":
		action := ""
		if len(parts) > 1 {
			action = parts[1]
		}
		switch action {
		case "insert":
			s.handleInsert(w, r, name)
		case "update":
			s.handleUpdate(w, r, name)
		case "delete":
			s.handleDelete(w, r, name)
		default:
			writeError(w, http.StatusBadRequest, "unknown action: "+action)
		}
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleTableData(w http.ResponseWriter, r *http.Request, name string) {
	limit := atoiDefault(r.URL.Query().Get("limit"), 100)
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	ctx := r.Context()
	sm, err := s.driver.ReadSchema(ctx, s.schema)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read schema: "+err.Error())
		return
	}
	tbl, ok := sm.Tables[name]
	if !ok {
		writeError(w, http.StatusNotFound, "table not found")
		return
	}

	cols := make([]columnDef, 0, len(tbl.ColumnOrder))
	colOrder := tbl.ColumnOrder
	if len(colOrder) == 0 {
		for n := range tbl.Columns {
			colOrder = append(colOrder, n)
		}
		sort.Strings(colOrder)
	}
	for _, cn := range colOrder {
		c := tbl.Columns[cn]
		if c == nil {
			continue
		}
		enumVals := s.enumValuesForCol(ctx, c)
		cols = append(cols, columnDef{
			Name:          c.Name,
			Type:          c.SQLType,
			Nullable:      c.Nullable,
			IsPK:          c.IsPK,
			AutoIncrement: c.AutoIncrement,
			EnumValues:    enumVals,
		})
	}

	idx := make([]indexSummary, 0, len(tbl.Indexes))
	for _, ix := range tbl.Indexes {
		idx = append(idx, indexSummary{Name: ix.Name, Columns: ix.Columns, Unique: ix.Unique})
	}
	sort.Slice(idx, func(i, j int) bool { return idx[i].Name < idx[j].Name })

	total, _ := s.countRows(ctx, name)
	start := time.Now()
	rows, err := s.fetchRows(ctx, name, colOrder, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch rows: "+err.Error())
		return
	}
	took := time.Since(start)

	writeJSON(w, http.StatusOK, tableData{
		Name:        name,
		Columns:     cols,
		Rows:        rows,
		Total:       total,
		Limit:       limit,
		Offset:      offset,
		Took:        took.String(),
		Indexes:     idx,
		Constraints: tbl.Constraints,
	})
}

// ─────────────────────────── CRUD HANDLERS ───────────────────────────

type insertRequest struct {
	Values map[string]any `json:"values"`
}

func (s *Server) handleInsert(w http.ResponseWriter, r *http.Request, name string) {
	ctx := r.Context()

	var body insertRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	sm, err := s.driver.ReadSchema(ctx, s.schema)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	tbl, ok := sm.Tables[name]
	if !ok {
		writeError(w, http.StatusNotFound, "table not found")
		return
	}

	colOrder := tbl.ColumnOrder
	if len(colOrder) == 0 {
		for n := range tbl.Columns {
			colOrder = append(colOrder, n)
		}
		sort.Strings(colOrder)
	}
	cols := make([]string, 0)
	vals := make([]any, 0)
	for _, cn := range colOrder {
		c := tbl.Columns[cn]
		if c == nil {
			continue
		}
		if c.AutoIncrement {
			continue
		}
		v, ok := body.Values[cn]
		if !ok {
			continue
		}
		cols = append(cols, s.qIdent(cn))
		vals = append(vals, v)
	}

	if len(cols) == 0 {
		writeError(w, http.StatusBadRequest, "no values provided")
		return
	}

	placeholders := make([]string, len(vals))
	for j := range vals {
		placeholders[j] = s.placeholder(j + 1)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		s.qualifiedTable(name),
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "))

	_, err = s.db.ExecContext(ctx, query, vals...)
	if err != nil {
		writeError(w, http.StatusConflict, friendlySQLError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type updateRequest struct {
	Values map[string]any `json:"values"`
	PK     map[string]any `json:"pk"`
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request, name string) {
	ctx := r.Context()

	var body updateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if len(body.PK) == 0 {
		writeError(w, http.StatusBadRequest, "no PK provided")
		return
	}

	setCols := make([]string, 0)
	vals := make([]any, 0)
	i := 1
	for cn, cv := range body.Values {
		setCols = append(setCols, fmt.Sprintf("%s = %s", s.qIdent(cn), s.placeholder(i)))
		vals = append(vals, cv)
		i++
	}

	if len(setCols) == 0 {
		writeError(w, http.StatusBadRequest, "no values to update")
		return
	}

	whereCols := make([]string, 0)
	for cn, cv := range body.PK {
		whereCols = append(whereCols, fmt.Sprintf("%s = %s", s.qIdent(cn), s.placeholder(i)))
		vals = append(vals, cv)
		i++
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		s.qualifiedTable(name),
		strings.Join(setCols, ", "),
		strings.Join(whereCols, " AND "))

	_, err := s.db.ExecContext(ctx, query, vals...)
	if err != nil {
		writeError(w, http.StatusConflict, friendlySQLError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type deleteRequest struct {
	PKs []map[string]any `json:"pks"`
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, name string) {
	ctx := r.Context()

	var body deleteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if len(body.PKs) == 0 {
		writeError(w, http.StatusBadRequest, "no PKs provided")
		return
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tx: "+err.Error())
		return
	}
	defer tx.Rollback()

	for _, pk := range body.PKs {
		whereCols := make([]string, 0)
		vals := make([]any, 0)
		i := 1
		for cn, cv := range pk {
			whereCols = append(whereCols, fmt.Sprintf("%s = %s", s.qIdent(cn), s.placeholder(i)))
			vals = append(vals, cv)
			i++
		}
		if len(whereCols) == 0 {
			continue
		}
		query := fmt.Sprintf("DELETE FROM %s WHERE %s",
			s.qualifiedTable(name),
			strings.Join(whereCols, " AND "))

		if _, err := tx.ExecContext(ctx, query, vals...); err != nil {
			writeError(w, http.StatusConflict, friendlySQLError(err))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusConflict, friendlySQLError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"deleted": len(body.PKs),
	})
}

func (s *Server) enumValuesForCol(ctx context.Context, col *migrate.ColumnModel) []string {
	// MySQL embeds enum values in SQLType: enum('a','b','c')
	if strings.HasPrefix(col.SQLType, "enum(") || strings.HasPrefix(col.SQLType, "ENUM(") {
		raw := col.SQLType
		start := strings.IndexByte(raw, '(')
		end := strings.LastIndexByte(raw, ')')
		if start > 0 && end > start {
			content := raw[start+1 : end]
			parts := strings.Split(content, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				p = strings.Trim(p, "'\"")
				out = append(out, p)
			}
			return out
		}
	}
	// Postgres: query pg_enum for the custom type name
	if s.driverName == "postgres" && s.schema != "" {
		typeName := col.SQLType
		if typeName != "" {
			rows, err := s.db.QueryContext(ctx,
				`SELECT e.enumlabel
				 FROM pg_enum e
				 JOIN pg_type t ON t.oid = e.enumtypid
				 JOIN pg_namespace n ON n.oid = t.typnamespace
				 WHERE t.typname = $1 AND n.nspname = $2
				 ORDER BY e.enumsortorder`,
				typeName, s.schema)
			if err == nil {
				defer rows.Close()
				out := make([]string, 0)
				for rows.Next() {
					var label string
					if err := rows.Scan(&label); err == nil {
						out = append(out, label)
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}
	return nil
}

func friendlySQLError(err error) string {
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "unique") || strings.Contains(low, "duplicate"):
		return "Duplicate value — this record conflicts with an existing unique constraint"
	case strings.Contains(low, "foreign key") && (strings.Contains(low, "insert") || strings.Contains(low, "update")):
		return "Cannot save — related record not found (foreign key constraint)"
	case strings.Contains(low, "foreign key") && strings.Contains(low, "delete"):
		return "Cannot delete — this record is referenced by other records"
	case strings.Contains(low, "not null"):
		return "Value cannot be empty (NOT NULL constraint)"
	case strings.Contains(low, "too long") || strings.Contains(low, "value too long"):
		return "Value is too long for this column"
	case strings.Contains(low, "cannot delete"):
		return "Cannot delete — the record may be referenced elsewhere or have constraints"
	default:
		return msg
	}
}

func (s *Server) placeholder(n int) string {
	switch s.driverName {
	case "postgres":
		return fmt.Sprintf("$%d", n)
	case "mssql":
		return fmt.Sprintf("@p%d", n)
	default: // mysql, sqlite
		return "?"
	}
}

// ─────────────────────────── DB HELPERS ───────────────────────────

func (s *Server) qIdent(name string) string {
	switch s.driverName {
	case "mysql":
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	case "mssql":
		return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
	default: // postgres, sqlite
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
}

func (s *Server) qualifiedTable(name string) string {
	if s.driverName == "postgres" || s.driverName == "mssql" {
		if s.schema != "" {
			return s.qIdent(s.schema) + "." + s.qIdent(name)
		}
	}
	return s.qIdent(name)
}

func (s *Server) countRows(ctx context.Context, table string) (int64, error) {
	q := "SELECT COUNT(*) FROM " + s.qualifiedTable(table)
	var n int64
	err := s.db.QueryRowContext(ctx, q).Scan(&n)
	return n, err
}

func (s *Server) fetchRows(ctx context.Context, table string, colOrder []string, limit, offset int) ([][]any, error) {
	if len(colOrder) == 0 {
		return nil, fmt.Errorf("no columns")
	}
	cols := make([]string, len(colOrder))
	for i, c := range colOrder {
		cols[i] = s.qIdent(c)
	}
	var query string
	switch s.driverName {
	case "mssql":
		// Use OFFSET/FETCH; requires ORDER BY. Order by first column for determinism.
		query = fmt.Sprintf("SELECT %s FROM %s ORDER BY %s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
			strings.Join(cols, ", "), s.qualifiedTable(table), cols[0], offset, limit)
	default:
		query = fmt.Sprintf("SELECT %s FROM %s LIMIT %d OFFSET %d",
			strings.Join(cols, ", "), s.qualifiedTable(table), limit, offset)
	}

	rs, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	colTypes, err := rs.ColumnTypes()
	if err != nil {
		return nil, err
	}
	out := make([][]any, 0)
	for rs.Next() {
		holder := make([]any, len(colTypes))
		ptrs := make([]any, len(colTypes))
		for i := range holder {
			ptrs[i] = &holder[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]any, len(holder))
		for i, v := range holder {
			row[i] = normalize(v)
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

func normalize(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		// Try as utf-8 string; fall back to base64-ish marker.
		s := string(t)
		if isPrintable(s) {
			return s
		}
		return fmt.Sprintf("<%d bytes>", len(t))
	case time.Time:
		return t.Format(time.RFC3339)
	default:
		return v
	}
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// ─────────────────────────── UTIL ───────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func openBrowser(url string) {
	time.Sleep(300 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
