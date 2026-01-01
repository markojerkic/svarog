package db

import (
	"context"
	"fmt"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

// TestGetLogsReturnsCorrectCursors tests that the forward and backward cursors
// are correctly populated in the response
func (suite *LogsCollectionRepositorySuite) TestGetLogsReturnsCorrectCursors() {
	t := suite.T()
	ctx := context.Background()

	// Create 10 logs with known timestamps and sequence numbers
	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 10)
	for i := 0; i < 10; i++ {
		logs[i] = types.StoredLog{
			Client: types.StoredClient{
				ClientId:  "test-client",
				IpAddress: "::1",
			},
			Timestamp:      baseTime.Add(time.Duration(i) * time.Second),
			SequenceNumber: i,
			LogLine:        fmt.Sprintf("Log line %d", i),
		}
	}

	err := suite.logService.SaveLogs(ctx, logs)
	assert.NoError(t, err)

	// Fetch first page (no cursor) - returns newest logs first (descending)
	logPage, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.NotNil(t, logPage)
	assert.Equal(t, 5, len(logPage.Logs))

	// Logs should be in descending order (newest first): 9, 8, 7, 6, 5
	assert.Equal(t, "Log line 9", logPage.Logs[0].LogLine)
	assert.Equal(t, "Log line 5", logPage.Logs[4].LogLine)

	assert.Nil(t, logPage.ForwardCursor, "Forward cursor should be nil on first page")
	assert.NotNil(t, logPage.BackwardCursor, "Backward cursor should not be nil on first page")
	lastLog := logPage.Logs[len(logPage.Logs)-1]
	assert.Equal(t, lastLog.Timestamp.UnixMilli(), logPage.BackwardCursor.Timestamp.UnixMilli())
	assert.Equal(t, lastLog.SequenceNumber, logPage.BackwardCursor.SequenceNumber)
	assert.True(t, logPage.BackwardCursor.IsBackward, "BackwardCursor should have IsBackward=true to get older logs")
}

// TestGetLogsForwardPagination tests paginating forward (scrolling down = getting older logs)
func (suite *LogsCollectionRepositorySuite) TestGetLogsForwardPagination() {
	t := suite.T()
	ctx := context.Background()

	// Create 20 logs
	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 20)
	for i := 0; i < 20; i++ {
		logs[i] = types.StoredLog{
			Client: types.StoredClient{
				ClientId:  "test-client",
				IpAddress: "::1",
			},
			Timestamp:      baseTime.Add(time.Duration(i) * time.Second),
			SequenceNumber: i,
			LogLine:        fmt.Sprintf("Log line %d", i),
		}
	}

	err := suite.logService.SaveLogs(ctx, logs)
	assert.NoError(t, err)

	// Fetch first page - should get logs 19, 18, 17, 16, 15 (newest first, descending)
	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1.Logs))
	// Verify descending order (newest first)
	assert.Equal(t, "Log line 19", page1.Logs[0].LogLine)
	assert.Equal(t, "Log line 15", page1.Logs[4].LogLine)

	// Use BackwardCursor to get next page (older logs - scrolling down)
	// BackwardCursor points to the last log of the page (oldest on page) with IsBackward=true
	assert.NotNil(t, page1.BackwardCursor, "BackwardCursor should be set on first page")
	assert.True(t, page1.BackwardCursor.IsBackward, "BackwardCursor should have IsBackward=true to get older logs")

	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page2.Logs))
	// Should get logs 14, 13, 12, 11, 10 (older than page1)
	assert.Equal(t, "Log line 14", page2.Logs[0].LogLine)
	assert.Equal(t, "Log line 10", page2.Logs[4].LogLine)

	// ForwardCursor should now be set (we came from a previous page)
	assert.NotNil(t, page2.ForwardCursor)
	assert.False(t, page2.ForwardCursor.IsBackward, "ForwardCursor should have IsBackward=false to get newer logs")

	// Continue to page 3 using BackwardCursor
	page3, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page2.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page3.Logs))
	// Should get logs 9, 8, 7, 6, 5 (older than page2)
	assert.Equal(t, "Log line 9", page3.Logs[0].LogLine)
	assert.Equal(t, "Log line 5", page3.Logs[4].LogLine)
}

// TestGetLogsBackwardPagination tests paginating backward (scrolling up = getting newer logs)
// ForwardCursor is used to scroll up (get newer logs) - it has IsBackward=false
func (suite *LogsCollectionRepositorySuite) TestGetLogsBackwardPagination() {
	t := suite.T()
	ctx := context.Background()

	// Create 20 logs
	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 20)
	for i := 0; i < 20; i++ {
		logs[i] = types.StoredLog{
			Client: types.StoredClient{
				ClientId:  "test-client",
				IpAddress: "::1",
			},
			Timestamp:      baseTime.Add(time.Duration(i) * time.Second),
			SequenceNumber: i,
			LogLine:        fmt.Sprintf("Log line %d", i),
		}
	}

	err := suite.logService.SaveLogs(ctx, logs)
	assert.NoError(t, err)

	// Navigate to page 2 first (so we have somewhere to go back to)
	// Use BackwardCursor to scroll down to older logs
	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, "Log line 19", page1.Logs[0].LogLine) // Newest

	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor, // Use BackwardCursor to get older logs
	})
	assert.NoError(t, err)
	assert.NotNil(t, page2.ForwardCursor, "ForwardCursor should be set on page2")
	assert.Equal(t, "Log line 14", page2.Logs[0].LogLine) // First log of page2

	// Now go backward using the ForwardCursor (scroll up = get newer logs)
	backPage, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page2.ForwardCursor, // Use ForwardCursor to get newer logs
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(backPage.Logs))

	// Should get logs newer than page2's first log (14)
	// All logs should have sequence number > 14
	for _, log := range backPage.Logs {
		assert.True(t, log.SequenceNumber > page2.Logs[0].SequenceNumber,
			"Expected log %d to be newer than %d", log.SequenceNumber, page2.Logs[0].SequenceNumber)
	}

	// Should include the newest logs (19, 18, 17, 16, 15)
	assert.Equal(t, "Log line 19", backPage.Logs[0].LogLine)
	assert.Equal(t, "Log line 15", backPage.Logs[4].LogLine)
}

// TestGetLogsEmptyResult tests that empty results have correct cursor values
func (suite *LogsCollectionRepositorySuite) TestGetLogsEmptyResult() {
	t := suite.T()
	ctx := context.Background()

	// Query for non-existent client
	logPage, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "non-existent-client",
		PageSize: 10,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.NotNil(t, logPage)
	assert.Equal(t, 0, len(logPage.Logs))
	assert.Nil(t, logPage.ForwardCursor)
	assert.Nil(t, logPage.BackwardCursor)
	assert.True(t, logPage.IsLastPage)
}

// TestGetLogsLastPage tests that we can paginate through all logs to the end
func (suite *LogsCollectionRepositorySuite) TestGetLogsLastPage() {
	t := suite.T()
	ctx := context.Background()

	// Create exactly 7 logs
	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 7)
	for i := 0; i < 7; i++ {
		logs[i] = types.StoredLog{
			Client: types.StoredClient{
				ClientId:  "test-client",
				IpAddress: "::1",
			},
			Timestamp:      baseTime.Add(time.Duration(i) * time.Second),
			SequenceNumber: i,
			LogLine:        fmt.Sprintf("Log line %d", i),
		}
	}

	err := suite.logService.SaveLogs(ctx, logs)
	assert.NoError(t, err)

	// Fetch first page of 5 (logs 6, 5, 4, 3, 2)
	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1.Logs))
	assert.Equal(t, "Log line 6", page1.Logs[0].LogLine)
	assert.Equal(t, "Log line 2", page1.Logs[4].LogLine)

	// Fetch second page using BackwardCursor - should only have 2 logs left (logs 1, 0)
	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor, // Use BackwardCursor to get older logs
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(page2.Logs))

	// Verify we got the remaining logs (1 and 0)
	assert.Equal(t, "Log line 1", page2.Logs[0].LogLine)
	assert.Equal(t, "Log line 0", page2.Logs[1].LogLine)
}

// TestCursorWithSameTimestamp tests pagination when multiple logs have the same timestamp
func (suite *LogsCollectionRepositorySuite) TestCursorWithSameTimestamp() {
	t := suite.T()
	ctx := context.Background()

	// Create 10 logs with the SAME timestamp but different sequence numbers
	sameTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 10)
	for i := 0; i < 10; i++ {
		logs[i] = types.StoredLog{
			Client: types.StoredClient{
				ClientId:  "test-client",
				IpAddress: "::1",
			},
			Timestamp:      sameTime,
			SequenceNumber: i,
			LogLine:        fmt.Sprintf("Log line %d", i),
		}
	}

	err := suite.logService.SaveLogs(ctx, logs)
	assert.NoError(t, err)

	// Fetch first page
	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1.Logs))

	// Should be sorted by sequence number descending when timestamps are equal
	assert.Equal(t, 9, page1.Logs[0].SequenceNumber)
	assert.Equal(t, 5, page1.Logs[4].SequenceNumber)

	// Fetch second page using cursor
	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page2.Logs))

	// Should get remaining logs with lower sequence numbers
	assert.Equal(t, 4, page2.Logs[0].SequenceNumber)
	assert.Equal(t, 0, page2.Logs[4].SequenceNumber)

	// Verify no overlap between pages
	page1SeqNums := make(map[int]bool)
	for _, log := range page1.Logs {
		page1SeqNums[log.SequenceNumber] = true
	}
	for _, log := range page2.Logs {
		assert.False(t, page1SeqNums[log.SequenceNumber],
			"Sequence number %d should not appear in both pages", log.SequenceNumber)
	}
}

// TestFullPaginationCycle tests a complete forward and backward pagination cycle
// BackwardCursor (IsBackward=true) is used to scroll down (get older logs)
// ForwardCursor (IsBackward=false) is used to scroll up (get newer logs)
func (suite *LogsCollectionRepositorySuite) TestFullPaginationCycle() {
	t := suite.T()
	ctx := context.Background()

	// Create 15 logs
	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 15)
	for i := 0; i < 15; i++ {
		logs[i] = types.StoredLog{
			Client: types.StoredClient{
				ClientId:  "test-client",
				IpAddress: "::1",
			},
			Timestamp:      baseTime.Add(time.Duration(i) * time.Second),
			SequenceNumber: i,
			LogLine:        fmt.Sprintf("Log line %d", i),
		}
	}

	err := suite.logService.SaveLogs(ctx, logs)
	assert.NoError(t, err)

	// Page 1: Get newest 5 logs (14, 13, 12, 11, 10)
	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Nil(t, page1.ForwardCursor, "First page should have no forward cursor (nothing to scroll up to)")
	assert.NotNil(t, page1.BackwardCursor, "First page should have backward cursor to scroll down")
	assert.Equal(t, "Log line 14", page1.Logs[0].LogLine)
	assert.Equal(t, "Log line 10", page1.Logs[4].LogLine)

	// Page 2: Get next 5 logs (9, 8, 7, 6, 5) - scroll down using BackwardCursor
	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor, // Use BackwardCursor to get older logs
	})
	assert.NoError(t, err)
	assert.NotNil(t, page2.ForwardCursor, "Second page should have forward cursor to scroll back up")
	assert.Equal(t, "Log line 9", page2.Logs[0].LogLine)
	assert.Equal(t, "Log line 5", page2.Logs[4].LogLine)

	// Page 3: Get last 5 logs (4, 3, 2, 1, 0) - scroll down using BackwardCursor
	page3, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page2.BackwardCursor, // Use BackwardCursor to get older logs
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page3.Logs))
	assert.Equal(t, "Log line 4", page3.Logs[0].LogLine)
	assert.Equal(t, "Log line 0", page3.Logs[4].LogLine)

	// Now go backward from page 3 - scroll up using ForwardCursor
	// ForwardCursor points to first log of page3 (log 4), $gt gets logs > 4
	// With descending sort, we get the newest 5 logs that are > 4: 14, 13, 12, 11, 10
	backFromPage3, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page3.ForwardCursor, // Use ForwardCursor to get newer logs
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(backFromPage3.Logs))

	// Should get newest 5 logs that are newer than page3's first log (4)
	assert.Equal(t, "Log line 14", backFromPage3.Logs[0].LogLine)
	assert.Equal(t, "Log line 10", backFromPage3.Logs[4].LogLine)
}
