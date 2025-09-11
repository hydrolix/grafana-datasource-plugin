package datasource

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestQueryPK_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer db.Close()
	p := "id"
	rows := mock.NewRows([]string{"primary_key"}).AddRow(p)

	mock.ExpectQuery(fmt.Sprintf(PRIMARY_KEY_QUERY_STRING, "db1", "tbl1")).
		WillReturnRows(rows)

	ds := &HydrolixDatasource{
		Connector: &MockConnector{
			db:  db,
			uid: "uid-123",
		},
		rowLimit: defaultRowLimit,
	}
	provider := &MetaDataProvider{ds: ds}
	pk, err := provider.QueryPK("db1", "tbl1")

	if err != nil {
		t.Fatalf("QueryPK returned error: %v", err)
	}
	if pk != "id" {
		t.Fatalf("expected pk 'id', got %q", pk)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestQueryPK_NoRows_ReturnsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer db.Close()

	// Zero rows -> field.Len()==0 -> PRIMARY_KEY_NOT_FOUND_ERROR
	rows := sqlmock.NewRows([]string{"primary_key"})
	mock.ExpectQuery(fmt.Sprintf(PRIMARY_KEY_QUERY_STRING, "db2", "tbl2")).
		WillReturnRows(rows)

	ds := &HydrolixDatasource{
		Connector: &MockConnector{
			db:  db,
			uid: "uid-abc",
		},
		rowLimit: defaultRowLimit,
	}
	provider := &MetaDataProvider{ds: ds}

	_, err = provider.QueryPK("db2", "tbl2")
	if err == nil || err.Error() != PRIMARY_KEY_NOT_FOUND_ERROR.Error() {
		t.Fatalf("expected PRIMARY_KEY_NOT_FOUND_ERROR, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGetPK_UsesCache_AvoidsSecondQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}
	defer db.Close()

	// First call: cache miss -> DB hit
	rows := sqlmock.NewRows([]string{"primary_key"}).AddRow("event_id")
	mock.ExpectQuery(fmt.Sprintf(PRIMARY_KEY_QUERY_STRING, "analytics", "events")).
		WillReturnRows(rows)
	mc := &MockConnector{
		db:  db,
		uid: "uid-cache",
	}
	ds := &HydrolixDatasource{Connector: mc, rowLimit: defaultRowLimit}
	provider := NewMetaDataProvider(ds)

	ctx := context.Background()

	// First call populates cache
	pk1, err := provider.GetPK(ctx, "analytics", "events")
	if err != nil {
		t.Fatalf("GetPK (first) error: %v", err)
	}
	if pk1 != "event_id" {
		t.Fatalf("expected 'event_id', got %q", pk1)
	}

	// Second call should be a cache hit -> no new DB call
	pk2, err := provider.GetPK(ctx, "analytics", "events")
	if err != nil {
		t.Fatalf("GetPK (second) error: %v", err)
	}
	if pk2 != "event_id" {
		t.Fatalf("expected 'event_id' on cache hit, got %q", pk2)
	}

	if mc.connCalls != 1 {
		t.Fatalf("expected exactly 1 getDBConnection call, got %d", mc.connCalls)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
