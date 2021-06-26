package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fleetdm/fleet/server/authz"
	"github.com/fleetdm/fleet/server/fleet"

	hostctx "github.com/fleetdm/fleet/server/contexts/host"
	"github.com/fleetdm/fleet/server/mock"
	"github.com/fleetdm/fleet/server/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCarveBegin(t *testing.T) {
	host := fleet.Host{ID: 3}
	payload := fleet.CarveBeginPayload{
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	expectedMetadata := fleet.CarveMetadata{
		ID:         7,
		HostId:     host.ID,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
	}
	ms.NewCarveFunc = func(metadata *fleet.CarveMetadata) (*fleet.CarveMetadata, error) {
		metadata.ID = 7
		return metadata, nil
	}

	ctx := hostctx.NewContext(context.Background(), host)

	metadata, err := svc.CarveBegin(ctx, payload)
	require.NoError(t, err)
	assert.NotEmpty(t, metadata.SessionId)
	metadata.SessionId = ""          // Clear this before comparison
	metadata.Name = ""               // Clear this before comparison
	metadata.CreatedAt = time.Time{} // Clear this before comparison
	assert.Equal(t, expectedMetadata, *metadata)
}

func TestCarveBeginNewCarveError(t *testing.T) {
	host := fleet.Host{ID: 3}
	payload := fleet.CarveBeginPayload{
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.NewCarveFunc = func(metadata *fleet.CarveMetadata) (*fleet.CarveMetadata, error) {
		return nil, fmt.Errorf("ouch!")
	}

	ctx := hostctx.NewContext(context.Background(), host)

	_, err := svc.CarveBegin(ctx, payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ouch!")
}

func TestCarveBeginEmptyError(t *testing.T) {
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ctx := hostctx.NewContext(context.Background(), fleet.Host{})

	_, err := svc.CarveBegin(ctx, fleet.CarveBeginPayload{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carve_size must be greater than 0")
}

func TestCarveBeginMissingHostError(t *testing.T) {
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}

	_, err := svc.CarveBegin(context.Background(), fleet.CarveBeginPayload{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing host")
}

func TestCarveBeginBlockSizeMaxError(t *testing.T) {
	host := fleet.Host{ID: 3}
	payload := fleet.CarveBeginPayload{
		BlockCount: 10,
		BlockSize:  1024 * 1024 * 1024 * 1024,      // 1TB
		CarveSize:  10 * 1024 * 1024 * 1024 * 1024, // 10TB
		RequestId:  "carve_request",
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}

	ctx := hostctx.NewContext(context.Background(), host)

	_, err := svc.CarveBegin(ctx, payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block_size exceeds max")
}

func TestCarveBeginCarveSizeMaxError(t *testing.T) {
	host := fleet.Host{ID: 3}
	payload := fleet.CarveBeginPayload{
		BlockCount: 1024 * 1024,
		BlockSize:  10 * 1024 * 1024,               // 1TB
		CarveSize:  10 * 1024 * 1024 * 1024 * 1024, // 10TB
		RequestId:  "carve_request",
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}

	ctx := hostctx.NewContext(context.Background(), host)

	_, err := svc.CarveBegin(ctx, payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carve_size exceeds max")
}

func TestCarveBeginCarveSizeError(t *testing.T) {
	host := fleet.Host{ID: 3}
	payload := fleet.CarveBeginPayload{
		BlockCount: 7,
		BlockSize:  13,
		CarveSize:  7*13 + 1,
		RequestId:  "carve_request",
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ctx := hostctx.NewContext(context.Background(), host)

	// Too big
	_, err := svc.CarveBegin(ctx, payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carve_size does not match")

	// Too small
	payload.CarveSize = 6 * 13
	_, err = svc.CarveBegin(ctx, payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carve_size does not match")
}

func TestCarveCarveBlockGetCarveError(t *testing.T) {
	sessionId := "foobar"
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.CarveBySessionIdFunc = func(sessionId string) (*fleet.CarveMetadata, error) {
		return nil, fmt.Errorf("ouch!")
	}

	payload := fleet.CarveBlockPayload{
		Data:      []byte("this is the carve data :)"),
		SessionId: sessionId,
	}

	err := svc.CarveBlock(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ouch!")
}

func TestCarveCarveBlockRequestIdError(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.CarveBySessionIdFunc = func(sessionId string) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.SessionId, sessionId)
		return metadata, nil
	}

	payload := fleet.CarveBlockPayload{
		Data:      []byte("this is the carve data :)"),
		RequestId: "not_matching",
		SessionId: sessionId,
	}

	err := svc.CarveBlock(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request_id does not match")
}

func TestCarveCarveBlockBlockCountExceedError(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.CarveBySessionIdFunc = func(sessionId string) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.SessionId, sessionId)
		return metadata, nil
	}

	payload := fleet.CarveBlockPayload{
		Data:      []byte("this is the carve data :)"),
		RequestId: "carve_request",
		SessionId: sessionId,
		BlockId:   23,
	}

	err := svc.CarveBlock(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block_id exceeds expected max")
}

func TestCarveCarveBlockBlockCountMatchError(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
		MaxBlock:   3,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.CarveBySessionIdFunc = func(sessionId string) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.SessionId, sessionId)
		return metadata, nil
	}

	payload := fleet.CarveBlockPayload{
		Data:      []byte("this is the carve data :)"),
		RequestId: "carve_request",
		SessionId: sessionId,
		BlockId:   7,
	}

	err := svc.CarveBlock(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block_id does not match")
}

func TestCarveCarveBlockBlockSizeError(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  16,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
		MaxBlock:   3,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.CarveBySessionIdFunc = func(sessionId string) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.SessionId, sessionId)
		return metadata, nil
	}

	payload := fleet.CarveBlockPayload{
		Data:      []byte("this is the carve data :) TOO LONG!!!"),
		RequestId: "carve_request",
		SessionId: sessionId,
		BlockId:   4,
	}

	err := svc.CarveBlock(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded declared block size")
}

func TestCarveCarveBlockNewBlockError(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
		MaxBlock:   3,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.CarveBySessionIdFunc = func(sessionId string) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.SessionId, sessionId)
		return metadata, nil
	}
	ms.NewBlockFunc = func(carve *fleet.CarveMetadata, blockId int64, data []byte) error {
		return fmt.Errorf("kaboom!")
	}

	payload := fleet.CarveBlockPayload{
		Data:      []byte("this is the carve data :)"),
		RequestId: "carve_request",
		SessionId: sessionId,
		BlockId:   4,
	}

	err := svc.CarveBlock(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kaboom!")
}

func TestCarveCarveBlock(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
		MaxBlock:   3,
	}
	payload := fleet.CarveBlockPayload{
		Data:      []byte("this is the carve data :)"),
		RequestId: "carve_request",
		SessionId: sessionId,
		BlockId:   4,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms}
	ms.CarveBySessionIdFunc = func(sessionId string) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.SessionId, sessionId)
		return metadata, nil
	}
	ms.NewBlockFunc = func(carve *fleet.CarveMetadata, blockId int64, data []byte) error {
		assert.Equal(t, metadata, carve)
		assert.Equal(t, int64(4), blockId)
		assert.Equal(t, payload.Data, data)
		return nil
	}

	err := svc.CarveBlock(context.Background(), payload)
	require.NoError(t, err)
	assert.True(t, ms.NewBlockFuncInvoked)
}

func TestCarveGetBlock(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
		MaxBlock:   3,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms, authz: authz.Must()}
	ms.CarveFunc = func(carveId int64) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.ID, carveId)
		return metadata, nil
	}
	ms.GetBlockFunc = func(carve *fleet.CarveMetadata, blockId int64) ([]byte, error) {
		assert.Equal(t, metadata.ID, carve.ID)
		assert.Equal(t, int64(3), blockId)
		return []byte("foobar"), nil
	}

	data, err := svc.GetBlock(test.UserContext(test.UserAdmin), metadata.ID, 3)
	require.NoError(t, err)
	assert.Equal(t, []byte("foobar"), data)
}

func TestCarveGetBlockNotAvailableError(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
		MaxBlock:   3,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms, authz: authz.Must()}
	ms.CarveFunc = func(carveId int64) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.ID, carveId)
		return metadata, nil
	}

	// Block requested is great than max block
	_, err := svc.GetBlock(test.UserContext(test.UserAdmin), metadata.ID, 7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet available")
}

func TestCarveGetBlockGetBlockError(t *testing.T) {
	sessionId := "foobar"
	metadata := &fleet.CarveMetadata{
		ID:         2,
		HostId:     3,
		BlockCount: 23,
		BlockSize:  64,
		CarveSize:  23 * 64,
		RequestId:  "carve_request",
		SessionId:  sessionId,
		MaxBlock:   3,
	}
	ms := new(mock.Store)
	svc := &Service{carveStore: ms, authz: authz.Must()}
	ms.CarveFunc = func(carveId int64) (*fleet.CarveMetadata, error) {
		assert.Equal(t, metadata.ID, carveId)
		return metadata, nil
	}
	ms.GetBlockFunc = func(carve *fleet.CarveMetadata, blockId int64) ([]byte, error) {
		assert.Equal(t, metadata.ID, carve.ID)
		assert.Equal(t, int64(3), blockId)
		return nil, fmt.Errorf("yow!!")
	}

	// Block requested is greater than max block
	_, err := svc.GetBlock(test.UserContext(test.UserAdmin), metadata.ID, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yow!!")
}
