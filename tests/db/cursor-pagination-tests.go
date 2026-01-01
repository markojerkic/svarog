package db

import (
	"context"
	"fmt"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (suite *LogsCollectionRepositorySuite) TestGetLogsReturnsCorrectCursors() {
	t := suite.T()
	ctx := context.Background()

	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 10)
	for i := range 10 {
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

	logPage, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.NotNil(t, logPage)
	assert.Equal(t, 5, len(logPage.Logs))

	assert.Equal(t, "Log line 9", logPage.Logs[0].LogLine)
	assert.Equal(t, "Log line 5", logPage.Logs[4].LogLine)

	assert.Nil(t, logPage.ForwardCursor, "Forward cursor should be nil on first page")
	assert.NotNil(t, logPage.BackwardCursor, "Backward cursor should not be nil on first page")
	lastLog := logPage.Logs[len(logPage.Logs)-1]
	assert.Equal(t, lastLog.Timestamp.UnixMilli(), logPage.BackwardCursor.Timestamp.UnixMilli())
	assert.Equal(t, lastLog.SequenceNumber, logPage.BackwardCursor.SequenceNumber)
	assert.True(t, logPage.BackwardCursor.IsBackward)
	assert.True(t, logPage.IsLastPage, "First page should be the last page")
}

func (suite *LogsCollectionRepositorySuite) TestGetLogsBackwardPagination() {
	t := suite.T()
	ctx := context.Background()

	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 20)
	for i := range 20 {
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

	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, "Log line 19", page1.Logs[0].LogLine)

	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Nil(t, page2.ForwardCursor)
	assert.NotNil(t, page2.BackwardCursor)
	assert.False(t, page2.IsLastPage, "Page2 should not be the last page")
	assert.Equal(t, "Log line 14", page2.Logs[0].LogLine)

	for _, log := range page2.Logs {
		assert.True(t, page1.BackwardCursor.SequenceNumber > log.SequenceNumber,
			"Expected log %d to be newer than %d", page1.BackwardCursor.SequenceNumber, log.SequenceNumber)
	}
}

func (suite *LogsCollectionRepositorySuite) TestGetLogsForwardPagination() {
	t := suite.T()
	ctx := context.Background()

	baseTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 20)
	for i := range 20 {
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

	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1.Logs))
	assert.Equal(t, "Log line 19", page1.Logs[0].LogLine)
	assert.Equal(t, "Log line 15", page1.Logs[4].LogLine)

	assert.NotNil(t, page1.BackwardCursor)
	assert.True(t, page1.BackwardCursor.IsBackward)

	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page2.Logs))
	assert.Equal(t, "Log line 14", page2.Logs[0].LogLine)
	assert.Equal(t, "Log line 10", page2.Logs[4].LogLine)

	assert.Nil(t, page2.ForwardCursor, "ForwardCursor should be nil when scrolling forward")
	assert.NotNil(t, page2.BackwardCursor)

	page3, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page2.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page3.Logs))
	assert.Equal(t, "Log line 9", page3.Logs[0].LogLine)
	assert.Equal(t, "Log line 5", page3.Logs[4].LogLine)
}

func (suite *LogsCollectionRepositorySuite) TestGetLogsEmptyResult() {
	t := suite.T()
	ctx := context.Background()

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

func (suite *LogsCollectionRepositorySuite) TestGetLogsLastPage() {
	t := suite.T()
	ctx := context.Background()

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

	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1.Logs))
	assert.Equal(t, "Log line 6", page1.Logs[0].LogLine)
	assert.Equal(t, "Log line 2", page1.Logs[4].LogLine)

	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(page2.Logs))
	assert.Equal(t, "Log line 1", page2.Logs[0].LogLine)
	assert.Equal(t, "Log line 0", page2.Logs[1].LogLine)
}

func (suite *LogsCollectionRepositorySuite) TestCursorWithSameTimestamp() {
	t := suite.T()
	ctx := context.Background()

	sameTime := time.Now().Truncate(time.Millisecond)
	logs := make([]types.StoredLog, 10)
	for i := range 10 {
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

	page1, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page1.Logs))
	assert.Equal(t, 9, page1.Logs[0].SequenceNumber)
	assert.Equal(t, 5, page1.Logs[4].SequenceNumber)

	page2, err := suite.logService.GetLogs(ctx, db.LogPageRequest{
		ClientId: "test-client",
		PageSize: 5,
		Cursor:   page1.BackwardCursor,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, len(page2.Logs))
	assert.Equal(t, 4, page2.Logs[0].SequenceNumber)
	assert.Equal(t, 0, page2.Logs[4].SequenceNumber)

	page1SeqNums := make(map[int]bool)
	for _, log := range page1.Logs {
		page1SeqNums[log.SequenceNumber] = true
	}
	for _, log := range page2.Logs {
		assert.False(t, page1SeqNums[log.SequenceNumber],
			"Sequence number %d should not appear in both pages", log.SequenceNumber)
	}
}
