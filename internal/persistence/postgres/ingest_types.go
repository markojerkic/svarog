package postgres

type IngestLogInsert struct {
	EventKey [32]byte
	Log      LogInsert
}

type InsertedIngestLog struct {
	Ordinal int64 // One-based position in the input batch.
	ID      int64
}
