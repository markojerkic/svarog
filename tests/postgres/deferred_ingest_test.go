package postgres_test

import "testing"

func TestMassImport(t *testing.T) {
	t.Skip("push 1000 sequenced lines through the log aggregator, wait until PostgreSQL has all of them, then page newest-first and require Log line N-1, N-2, ... with no gaps")
}

func TestOutOfOrderInsert(t *testing.T) {
	t.Skip("push 10000 lines through the aggregator even indexes first and odd indexes second, then page newest-first and require sequence order rather than arrival order")
}

func TestFindLoglinePage(t *testing.T) {
	t.Skip("ingest 10000 lines, open ?logLine= for a line near the middle with page size 300, and require that page to contain the target and stay sorted by time, sequence, and id")
}

func TestArchiveLargeClient(t *testing.T) {
	t.Skip("save 10000 logs older than the policy cutoff and 1000 newer logs, archive the client, and require the zip to contain the old lines, the new lines to remain, and no log older than the cutoff to remain")
}
