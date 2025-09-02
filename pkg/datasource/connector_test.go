package datasource

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v4"
	"net/http"
	"sync/atomic"
	"testing"
)

// --- helpers ---

type stubDriver struct {
	settings     sqlds.DriverSettings
	connectDBs   []*sql.DB
	connectErrs  []error
	connectCalls int32
}

func (d *stubDriver) Settings(_ context.Context, _ backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	return d.settings
}

func (d *stubDriver) Connect(_ context.Context, _ backend.DataSourceInstanceSettings, _ json.RawMessage) (*sql.DB, error) {
	i := int(atomic.AddInt32(&d.connectCalls, 1)) - 1
	var db *sql.DB
	var err error
	if i < len(d.connectDBs) {
		db = d.connectDBs[i]
	}
	if i < len(d.connectErrs) {
		err = d.connectErrs[i]
	}
	// Fallback when arrays shorter: last provided
	if db == nil && len(d.connectDBs) > 0 {
		db = d.connectDBs[len(d.connectDBs)-1]
	}
	if err == nil && len(d.connectErrs) > 0 && i >= len(d.connectErrs) {
		err = d.connectErrs[len(d.connectErrs)-1]
	}
	return db, err
}
func (d *stubDriver) Macros() sqlutil.Macros {
	return make(sqlutil.Macros)
}
func (d *stubDriver) Converters() []sqlutil.Converter {
	return []sqlutil.Converter{}
}

func newSqlmockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

// Dummy instance settings
func inst(uid string) backend.DataSourceInstanceSettings {
	return backend.DataSourceInstanceSettings{UID: uid}
}

// --- tests ---

func TestShouldRetry(t *testing.T) {
	cases := []struct {
		retryOn []string
		err     string
		want    bool
	}{
		{[]string{"timeout", "deadlock"}, "query timeout occurred", true},
		{[]string{"temporary"}, "temporary network issue", true},
		{[]string{"temporary"}, "permanent failure", false},
		{nil, "anything", false},
	}
	for _, c := range cases {
		if got := shouldRetry(c.retryOn, c.err); got != c.want {
			t.Fatalf("shouldRetry(%v,%q)=%v want %v", c.retryOn, c.err, got, c.want)
		}
	}
}

func TestApplyHeaders(t *testing.T) {
	q := &sqlutil.Query{}
	h := http.Header{}
	h.Set("X-Auth", "abc")
	h.Add("X-Auth", "def")
	h.Set("X-User", "alice")

	out := applyHeaders(q, h)
	if string(out.ConnectionArgs) == "" {
		t.Fatalf("ConnectionArgs empty")
	}
	var args map[string]any
	if err := json.Unmarshal(out.ConnectionArgs, &args); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	raw, ok := args[HeaderKey]
	if !ok {
		t.Fatalf("expected %q key in ConnectionArgs", HeaderKey)
	}
	// http.Header marshals as map[string][]string
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected header map, got %T", raw)
	}
	if _, ok := m["X-Auth"]; !ok {
		t.Fatalf("missing X-Auth in headers")
	}
	if _, ok := m["X-User"]; !ok {
		t.Fatalf("missing X-User in headers")
	}
}

func TestReconnectClosesAndReplacesConnection(t *testing.T) {
	// initial connection (created by NewConnector)
	initDB, initMock := newSqlmockDB(t)
	initMock.ExpectClose().WillReturnError(nil)

	// new connection returned by Reconnect
	newDB, _ := newSqlmockDB(t)

	driver := &stubDriver{
		settings:   sqlds.DriverSettings{},
		connectDBs: []*sql.DB{initDB, newDB},
	}
	connector, err := NewConnector(context.Background(), driver, inst("uid3"), false)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	key := defaultKey(connector.GetUID())
	dbConn, _ := connector.getDBConnection(key)

	gotDB, err := connector.Reconnect(context.Background(), dbConn, &sqlutil.Query{}, key)
	if err != nil {
		t.Fatalf("Reconnect: %v", err)
	}
	if gotDB != newDB {
		t.Fatalf("Reconnect returned wrong db")
	}
	// Ensure close on old was called
	if err := initMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock expectations: %v", err)
	}
}

func TestGetConnectionFromQuery_DisabledMultiConn_WithArgsReturnsError(t *testing.T) {
	db, _ := newSqlmockDB(t)
	driver := &stubDriver{
		settings:   sqlds.DriverSettings{ForwardHeaders: false},
		connectDBs: []*sql.DB{db},
	}
	connector, err := NewConnector(context.Background(), driver, inst("uid6"), false)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}

	q := &sqlutil.Query{ConnectionArgs: []byte(`{"foo":"bar"}`)}
	_, _, err = connector.GetConnectionFromQuery(context.Background(), q)
	if err == nil {
		t.Fatalf("expected ErrorMissingMultipleConnectionsConfig, got nil")
	}
	if !errors.Is(err, ErrorMissingMultipleConnectionsConfig) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetConnectionFromQuery_NoArgs_ReturnsDefault(t *testing.T) {
	db, _ := newSqlmockDB(t)
	driver := &stubDriver{
		settings:   sqlds.DriverSettings{},
		connectDBs: []*sql.DB{db},
	}
	connector, err := NewConnector(context.Background(), driver, inst("uid7"), true /* enable multiple, but no args */)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}

	q := &sqlutil.Query{} // no args
	key, dbConn, err := connector.GetConnectionFromQuery(context.Background(), q)
	if err != nil {
		t.Fatalf("GetConnectionFromQuery: %v", err)
	}
	if key == "" {
		t.Fatalf("expected non-empty key")
	}
	if dbConn.db == nil {
		t.Fatalf("expected non-nil db")
	}
}

func TestGetConnectionFromQuery_NewArgs_CachesPerArgs(t *testing.T) {
	// initial connection created by NewConnector
	initDB, _ := newSqlmockDB(t)
	// two distinct new DBs for two distinct arg sets (only first used twice)
	dbA1, _ := newSqlmockDB(t)
	dbA2, _ := newSqlmockDB(t) // this should NOT be used because first is cached
	dbB, _ := newSqlmockDB(t)

	driver := &stubDriver{
		settings:   sqlds.DriverSettings{},
		connectDBs: []*sql.DB{initDB, dbA1, dbA2, dbB},
	}
	connector, err := NewConnector(context.Background(), driver, inst("uid8"), true)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}

	ctx := context.Background()
	qA := &sqlutil.Query{ConnectionArgs: []byte(`{"tenant":"A"}`)}
	qB := &sqlutil.Query{ConnectionArgs: []byte(`{"tenant":"B"}`)}

	// First time with A -> creates and caches
	keyA1, connA1, err := connector.GetConnectionFromQuery(ctx, qA)
	if err != nil {
		t.Fatalf("GetConnectionFromQuery A1: %v", err)
	}
	// Second time with same args -> should be cached (no extra Connect)
	keyA2, connA2, err := connector.GetConnectionFromQuery(ctx, qA)
	if err != nil {
		t.Fatalf("GetConnectionFromQuery A2: %v", err)
	}
	if keyA1 != keyA2 || connA1.db != connA2.db {
		t.Fatalf("expected cached connection for same args")
	}

	// Different args -> new connection
	keyB, connB, err := connector.GetConnectionFromQuery(ctx, qB)
	if err != nil {
		t.Fatalf("GetConnectionFromQuery B: %v", err)
	}
	if keyB == keyA1 || connB.db == connA1.db {
		t.Fatalf("expected different key/connection for different args")
	}
}

func TestDispose_ClosesAllAndClears(t *testing.T) {
	db1, mock1 := newSqlmockDB(t)
	db2, mock2 := newSqlmockDB(t)
	mock1.ExpectClose().WillReturnError(nil)
	mock2.ExpectClose().WillReturnError(nil)

	driver := &stubDriver{
		settings:   sqlds.DriverSettings{},
		connectDBs: []*sql.DB{db1},
	}
	connector, err := NewConnector(context.Background(), driver, inst("uid9"), true)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}

	// Manually store another connection to ensure both are closed
	connector.storeDBConnection("extra", dbConnection{db2, inst("uid9")})

	// Dispose should close both and clear map
	connector.Dispose()

	// Both closes must have been hit
	if err := mock1.ExpectationsWereMet(); err != nil {
		t.Fatalf("db1 expectations: %v", err)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Fatalf("db2 expectations: %v", err)
	}

	// After Clear, we shouldn't find previous keys
	if _, ok := connector.getDBConnection(defaultKey(connector.GetUID())); ok {
		t.Fatalf("expected connections map to be cleared")
	}
	if _, ok := connector.getDBConnection("extra"); ok {
		t.Fatalf("expected connections map to be cleared")
	}
}

type MockConnector struct {
	db        *sql.DB
	uid       string
	connCalls int
}

func (m *MockConnector) Connect(_ context.Context, _ http.Header) (*dbConnection, error) {
	return &dbConnection{db: m.db}, nil
}
func (m *MockConnector) connectWithRetries(_ context.Context, _ dbConnection, _ string, _ http.Header) error {
	return nil
}
func (m *MockConnector) connect(_ dbConnection) error { return nil }
func (m *MockConnector) ping(_ dbConnection) error    { return nil }

func (m *MockConnector) Reconnect(_ context.Context, _ dbConnection, _q *sqlutil.Query, _ string) (*sql.DB, error) {
	return m.db, nil
}

func (m *MockConnector) getDBConnection(_ string) (dbConnection, bool) {
	m.connCalls++
	return dbConnection{db: m.db}, true
}

func (m *MockConnector) storeDBConnection(_ string, _ dbConnection) {}

func (m *MockConnector) Dispose() {}

func (m *MockConnector) GetConnectionFromQuery(_ context.Context, _ *sqlutil.Query) (string, dbConnection, error) {
	return "key", dbConnection{db: m.db}, nil
}

func (m *MockConnector) GetDriver() sqlds.Driver { return nil }

func (m *MockConnector) GetUID() string { return m.uid }

func (m *MockConnector) getDriverSettings() sqlds.DriverSettings { return sqlds.DriverSettings{} }

func (m *MockConnector) getInstanceSettings() backend.DataSourceInstanceSettings {
	return backend.DataSourceInstanceSettings{}
}
