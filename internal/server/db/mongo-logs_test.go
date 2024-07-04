package db

import "testing"

func TestRepositoryImplementsLogRepository(t *testing.T) {
	var _ LogRepository = &MongoLogRepository{}
}
