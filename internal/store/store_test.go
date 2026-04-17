package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/dshills/atlas/internal/db"
	"github.com/dshills/atlas/internal/diag"
	"github.com/dshills/atlas/internal/extractor"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return New(d)
}

func sampleFile(path string) *FileRow {
	return &FileRow{
		Path:        path,
		Language:    "go",
		ContentHash: "deadbeef",
		SizeBytes:   42,
		ParseStatus: "ok",
	}
}

func TestUpsertFileInsertAndUpdate(t *testing.T) {
	s := testStore(t)

	id1, err := s.UpsertFile(s.DB, sampleFile("a.go"))
	if err != nil {
		t.Fatalf("initial insert: %v", err)
	}
	if id1 == 0 {
		t.Fatal("expected nonzero id")
	}

	// Second call with same path should update, not insert.
	row := sampleFile("a.go")
	row.ContentHash = "cafebabe"
	row.SizeBytes = 99
	id2, err := s.UpsertFile(s.DB, row)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if id2 != id1 {
		t.Errorf("expected same id %d, got %d", id1, id2)
	}

	got, err := s.GetFileByPath("a.go")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.ContentHash != "cafebabe" || got.SizeBytes != 99 {
		t.Errorf("update did not persist: %+v", got)
	}
}

func TestUpsertFileInsideTx(t *testing.T) {
	s := testStore(t)

	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.UpsertFile(tx, sampleFile("tx.go"))
	if err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// After rollback the row must not be visible.
	if _, err := s.GetFileByPath("tx.go"); err != sql.ErrNoRows {
		t.Errorf("expected row to be rolled back, got id=%d err=%v", id, err)
	}
}

func TestSetParseStatus(t *testing.T) {
	s := testStore(t)

	id, err := s.UpsertFile(s.DB, sampleFile("p.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetParseStatus(s.DB, id, "error"); err != nil {
		t.Fatalf("set parse status: %v", err)
	}
	got, err := s.GetFileByPath("p.go")
	if err != nil {
		t.Fatal(err)
	}
	if got.ParseStatus != "error" {
		t.Errorf("parse_status = %q, want error", got.ParseStatus)
	}
}

func TestUpsertSymbolsReplacesPriorRows(t *testing.T) {
	s := testStore(t)
	fileID, err := s.UpsertFile(s.DB, sampleFile("sym.go"))
	if err != nil {
		t.Fatal(err)
	}

	first := []extractor.SymbolRecord{
		{Name: "A", QualifiedName: "pkg.A", SymbolKind: "function", Visibility: "exported", StableID: "pkg.A#1"},
		{Name: "B", QualifiedName: "pkg.B", SymbolKind: "function", Visibility: "unexported", StableID: "pkg.B#1"},
	}
	n, err := s.UpsertSymbols(s.DB, fileID, 0, first)
	if err != nil {
		t.Fatalf("initial upsert: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 inserted, got %d", n)
	}

	// Upserting a smaller set must delete the old rows.
	second := []extractor.SymbolRecord{
		{Name: "C", QualifiedName: "pkg.C", SymbolKind: "function", Visibility: "exported", StableID: "pkg.C#1"},
	}
	if _, err := s.UpsertSymbols(s.DB, fileID, 0, second); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE file_id = ?`, fileID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 symbol after replace, got %d", count)
	}
}

func TestUpsertSymbolsEmptySliceStillDeletes(t *testing.T) {
	s := testStore(t)
	fileID, err := s.UpsertFile(s.DB, sampleFile("empty.go"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpsertSymbols(s.DB, fileID, 0, []extractor.SymbolRecord{
		{Name: "X", QualifiedName: "pkg.X", SymbolKind: "function", Visibility: "exported", StableID: "pkg.X#1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Empty upsert must still clear prior rows for the file.
	n, err := s.UpsertSymbols(s.DB, fileID, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 inserted, got %d", n)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE file_id = ?`, fileID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 symbols after empty upsert, got %d", count)
	}
}

func TestUpsertReferencesReplacesPriorRows(t *testing.T) {
	s := testStore(t)
	fileID, err := s.UpsertFile(s.DB, sampleFile("refs.go"))
	if err != nil {
		t.Fatal(err)
	}

	refs := []extractor.ReferenceRecord{
		{ReferenceKind: "imports", Confidence: "exact", RawTargetText: "fmt", Line: 3},
		{ReferenceKind: "calls", Confidence: "likely", RawTargetText: "fmt.Println", Line: 5},
	}
	n, err := s.UpsertReferences(s.DB, fileID, refs)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 refs inserted, got %d", n)
	}

	// Upsert again with one ref; the other should be removed.
	_, err = s.UpsertReferences(s.DB, fileID, refs[:1])
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM "references" WHERE from_file_id = ?`, fileID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 ref after replace, got %d", count)
	}
}

func TestUpsertArtifactsInsertsAndClears(t *testing.T) {
	s := testStore(t)
	fileID, err := s.UpsertFile(s.DB, sampleFile("arts.go"))
	if err != nil {
		t.Fatal(err)
	}

	arts := []extractor.ArtifactRecord{
		{ArtifactKind: "route", Name: "GET /users", DataJSON: `{"method":"GET"}`, Confidence: "exact"},
		{ArtifactKind: "sql_query", Name: "SELECT users", DataJSON: `{}`, Confidence: "heuristic"},
	}
	n, err := s.UpsertArtifacts(s.DB, fileID, arts)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 artifacts inserted, got %d", n)
	}

	// Empty upsert clears.
	if _, err := s.UpsertArtifacts(s.DB, fileID, nil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE file_id = ?`, fileID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 artifacts after clear, got %d", count)
	}
}

func TestUpsertPackageAndLink(t *testing.T) {
	s := testStore(t)
	fileID, err := s.UpsertFile(s.DB, sampleFile("pkg.go"))
	if err != nil {
		t.Fatal(err)
	}

	pkgID, err := s.UpsertPackage(s.DB, &PackageRow{
		Name:          "pkg",
		ImportPath:    sql.NullString{String: "example.com/pkg", Valid: true},
		DirectoryPath: "pkg",
		Language:      "go",
	})
	if err != nil {
		t.Fatalf("upsert package: %v", err)
	}
	if pkgID == 0 {
		t.Fatal("expected nonzero package id")
	}

	// Re-upserting the same directory_path returns the same id.
	pkgID2, err := s.UpsertPackage(s.DB, &PackageRow{
		Name:          "pkg",
		DirectoryPath: "pkg",
		Language:      "go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pkgID2 != pkgID {
		t.Errorf("expected reuse of pkg id, got %d vs %d", pkgID2, pkgID)
	}

	if err := s.LinkFileToPackage(s.DB, fileID, pkgID); err != nil {
		t.Fatalf("link: %v", err)
	}
	// Linking twice should not error (INSERT OR IGNORE).
	if err := s.LinkFileToPackage(s.DB, fileID, pkgID); err != nil {
		t.Fatalf("relink: %v", err)
	}

	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM package_files WHERE file_id = ? AND package_id = ?`, fileID, pkgID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 package_files row, got %d", count)
	}
}

func TestUpsertSymbolsInTxIsAtomic(t *testing.T) {
	s := testStore(t)
	fileID, err := s.UpsertFile(s.DB, sampleFile("atomic.go"))
	if err != nil {
		t.Fatal(err)
	}

	tx, err := s.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpsertSymbols(tx, fileID, 0, []extractor.SymbolRecord{
		{Name: "Z", QualifiedName: "pkg.Z", SymbolKind: "function", Visibility: "exported", StableID: "pkg.Z#1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// After rollback the symbol must not be visible.
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE name = 'Z'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected symbols rolled back, got %d", count)
	}
}

func TestAllFilePathsAndFileHashMap(t *testing.T) {
	s := testStore(t)
	for _, path := range []string{"a.go", "b.go", "c.go"} {
		if _, err := s.UpsertFile(s.DB, sampleFile(path)); err != nil {
			t.Fatal(err)
		}
	}

	paths, err := s.AllFilePaths()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(paths))
	}
	for _, want := range []string{"a.go", "b.go", "c.go"} {
		if paths[want] == 0 {
			t.Errorf("missing %q in AllFilePaths", want)
		}
	}

	hashes, err := s.FileHashMap()
	if err != nil {
		t.Fatal(err)
	}
	if hashes["a.go"] != "deadbeef" {
		t.Errorf("hash for a.go = %q", hashes["a.go"])
	}
}

func TestRunLifecycle(t *testing.T) {
	s := testStore(t)

	runID, err := s.InsertRun("full", "abc123")
	if err != nil {
		t.Fatalf("insert run: %v", err)
	}
	if runID == 0 {
		t.Fatal("expected nonzero run id")
	}

	run := &RunRow{
		ID:             runID,
		Status:         "success",
		FilesScanned:   10,
		FilesChanged:   4,
		SymbolsWritten: 42,
		ErrorCount:     1,
	}
	if err := s.FinishRun(run); err != nil {
		t.Fatalf("finish run: %v", err)
	}

	latest, err := s.LatestRun()
	if err != nil {
		t.Fatalf("latest run: %v", err)
	}
	if latest.ID != runID || latest.Status != "success" || latest.FilesChanged != 4 {
		t.Errorf("LatestRun returned unexpected row: %+v", latest)
	}
	if !latest.GitCommit.Valid || latest.GitCommit.String != "abc123" {
		t.Errorf("git_commit = %+v, want abc123", latest.GitCommit)
	}
}

func TestInsertRunNoGitCommit(t *testing.T) {
	s := testStore(t)
	runID, err := s.InsertRun("incremental", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE index_runs SET finished_at = '2024-01-01T00:00:00Z', status = 'success' WHERE id = ?`, runID); err != nil {
		t.Fatal(err)
	}
	latest, err := s.LatestRun()
	if err != nil {
		t.Fatal(err)
	}
	if latest.GitCommit.Valid {
		t.Errorf("git_commit should be NULL when empty, got %+v", latest.GitCommit)
	}
}

func TestPersistDiagnostics(t *testing.T) {
	s := testStore(t)
	runID, err := s.InsertRun("full", "")
	if err != nil {
		t.Fatal(err)
	}
	fileID, err := s.UpsertFile(s.DB, sampleFile("d.go"))
	if err != nil {
		t.Fatal(err)
	}

	diags := []diag.Diagnostic{
		{Severity: diag.SeverityError, Code: "E1", Message: "boom", FileID: fileID, Line: 3, ColumnStart: 1, ColumnEnd: 5, DetailsJSON: `{"x":1}`},
		{Severity: diag.SeverityWarning, Code: "W1", Message: "careful"},
	}
	if err := s.PersistDiagnostics(runID, diags); err != nil {
		t.Fatalf("persist: %v", err)
	}

	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM diagnostics WHERE run_id = ?`, runID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 diagnostics, got %d", count)
	}

	// Verify the file-scoped one kept its file_id and line.
	var fid sql.NullInt64
	var line sql.NullInt64
	err = s.DB.QueryRow(`SELECT file_id, line FROM diagnostics WHERE code = 'E1'`).Scan(&fid, &line)
	if err != nil {
		t.Fatal(err)
	}
	if !fid.Valid || fid.Int64 != fileID || !line.Valid || line.Int64 != 3 {
		t.Errorf("file_id=%+v line=%+v, want %d and 3", fid, line, fileID)
	}
}

func TestDeleteFileCascades(t *testing.T) {
	s := testStore(t)
	fileID, err := s.UpsertFile(s.DB, sampleFile("del.go"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpsertSymbols(s.DB, fileID, 0, []extractor.SymbolRecord{
		{Name: "X", QualifiedName: "pkg.X", SymbolKind: "function", Visibility: "exported", StableID: "pkg.X#1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteFile(fileID); err != nil {
		t.Fatal(err)
	}
	var symCount int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM symbols WHERE file_id = ?`, fileID).Scan(&symCount); err != nil {
		t.Fatal(err)
	}
	if symCount != 0 {
		t.Errorf("expected symbols cascaded on file delete, got %d", symCount)
	}
}
