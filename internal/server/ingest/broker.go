package ingest

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

const (
	bootstrapLock   int64 = 0x73766172696e6701
	markerKey             = "svarog.ingest"
	markerPreparing       = "preparing-commit-v1"
	markerReady           = "commit-v1"
)

// prepareBroker never recreates a stream or durable. The session lock spans all
// broker operations so simultaneous replicas cannot switch retention before the
// durable exists. Limits -> Interest requires explicit legacy migration consent.
func prepareBroker(ctx context.Context, opts Options) (jetstream.Consumer, error) {
	lock, err := opts.Pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire ingest bootstrap connection: %w", err)
	}
	locked := false
	defer func() {
		if lock == nil {
			return
		}
		if !locked {
			lock.Release()
			return
		}
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		if err := lock.QueryRow(unlockCtx, "SELECT pg_advisory_unlock($1)", bootstrapLock).Scan(&unlocked); err != nil || !unlocked {
			// Do not return a session that may still own the lock to the pool.
			_ = lock.Hijack().Close(unlockCtx)
			return
		}
		lock.Release()
	}()
	// If lock acquisition is canceled its outcome can be uncertain; discard the
	// session rather than risk returning a held advisory lock to the pool.
	if _, err := lock.Exec(ctx, "SELECT pg_advisory_lock($1)", bootstrapLock); err != nil {
		_ = lock.Hijack().Close(context.Background())
		lock = nil
		return nil, fmt.Errorf("lock ingest bootstrap: %w", err)
	}
	locked = true
	return configureBroker(ctx, opts)
}

func configureBroker(ctx context.Context, opts Options) (jetstream.Consumer, error) {
	var major, minor, patch int
	if _, err := fmt.Sscanf(opts.Connection.Conn.ConnectedServerVersion(), "%d.%d.%d", &major, &minor, &patch); err != nil || major < 2 || (major == 2 && (minor < 10 || (minor == 10 && patch < 4))) {
		return nil, errors.New("ingest requires NATS 2.10.4 or newer for TermWithReason")
	}
	if opts.Connection.Conn.MaxPayload() < maxMessageBytes {
		return nil, errors.New("ingest requires a NATS max_payload of at least 1 MiB")
	}
	js := opts.Connection.JetStream
	stream, err := js.Stream(ctx, streamName)
	if errors.Is(err, jetstream.ErrStreamNotFound) {
		limit := opts.StreamMaxBytes
		if limit == 0 {
			limit = 1 << 30
		}
		stream, err = js.CreateStream(ctx, jetstream.StreamConfig{
			Name: streamName, Subjects: []string{logSubject}, Storage: jetstream.FileStorage,
			Retention: jetstream.LimitsPolicy, Discard: jetstream.DiscardNew,
			MaxBytes: limit, MaxMsgs: -1, MaxMsgsPerSubject: -1, MaxMsgSize: maxMessageBytes,
			MaxConsumers: 1, Metadata: map[string]string{markerKey: markerPreparing},
		})
	}
	if err != nil {
		return nil, fmt.Errorf("load/create ingest stream: %w", err)
	}
	info := stream.CachedInfo()
	cfg := info.Config
	if err := validateStream(cfg); err != nil {
		return nil, err
	}
	marker := cfg.Metadata[markerKey]
	if marker != "" && marker != markerPreparing && marker != markerReady {
		return nil, fmt.Errorf("unknown LOGS ingest marker %q", marker)
	}
	if cfg.Retention != jetstream.LimitsPolicy && cfg.Retention != jetstream.InterestPolicy {
		return nil, errors.New("LOGS retention cannot be converted to InterestPolicy in place")
	}
	if cfg.Retention == jetstream.InterestPolicy && marker != markerReady {
		return nil, errors.New("refusing to adopt an unrecognized InterestPolicy LOGS stream")
	}
	if cfg.Retention == jetstream.LimitsPolicy && marker != markerPreparing && !opts.AllowRetentionMigration {
		return nil, errors.New("legacy LOGS retention migration disabled: back up stream and consumer state, stop all old writers, then explicitly set INGEST_ALLOW_RETENTION_MIGRATION=true")
	}
	names := stream.ConsumerNames(ctx)
	hasConsumer := false
	var unexpectedConsumer string
	for name := range names.Name() {
		if name != consumerName {
			unexpectedConsumer = name
			continue
		}
		hasConsumer = true
	}
	if err := names.Err(); err != nil {
		return nil, fmt.Errorf("list ingest consumers: %w", err)
	}
	if unexpectedConsumer != "" {
		return nil, fmt.Errorf("unexpected LOGS consumer %q; migration requires only %s", unexpectedConsumer, consumerName)
	}
	if !hasConsumer && marker != markerPreparing {
		return nil, errors.New("LOGS log-processor durable is missing; incident requires operator inspection, not automatic recreation")
	}
	limit := opts.StreamMaxBytes
	if limit == 0 {
		limit = cfg.MaxBytes
	}
	if limit <= 0 || uint64(limit) < info.State.Bytes {
		return nil, errors.New("INGEST_STREAM_MAX_BYTES must be finite and at least the current LOGS storage usage")
	}
	var consumer jetstream.Consumer
	cc := jetstream.ConsumerConfig{Durable: consumerName, FilterSubject: logSubject, AckPolicy: jetstream.AckExplicitPolicy}
	if hasConsumer {
		consumer, err = js.Consumer(ctx, streamName, consumerName)
		if err != nil {
			return nil, fmt.Errorf("load ingest consumer: %w", err)
		}
		cc = consumer.CachedInfo().Config
		if err := validateConsumer(cc); err != nil {
			return nil, err
		}
	}
	// Start from the existing config to preserve replication, placement, security,
	// compression and unrelated metadata. Never lower the limit by default.
	cfg.MaxBytes = limit
	cfg.MaxAge = 0
	cfg.MaxMsgs = -1
	cfg.MaxMsgsPerSubject = -1
	cfg.MaxMsgSize = maxMessageBytes
	cfg.Discard = jetstream.DiscardNew
	cfg.DiscardNewPerSubject = false
	cfg.MaxConsumers = 1
	cfg.ConsumerLimits.InactiveThreshold = 0
	cfg.Metadata = maps.Clone(cfg.Metadata)
	if cfg.Metadata == nil {
		cfg.Metadata = make(map[string]string)
	}
	// First protect the backlog without changing retention. A durable must exist
	// before InterestPolicy is enabled, including during interrupted bootstrap.
	if _, err := js.UpdateStream(ctx, cfg); err != nil {
		return nil, fmt.Errorf("protect ingest backlog: %w", err)
	}
	cc.AckWait = opts.AckWait
	cc.MaxAckPending = 1024
	cc.MaxDeliver = -1
	if hasConsumer {
		consumer, err = js.UpdateConsumer(ctx, streamName, cc)
	} else {
		consumer, err = js.CreateConsumer(ctx, streamName, cc)
	}
	if err != nil {
		return nil, fmt.Errorf("configure ingest durable: %w", err)
	}
	if consumer.CachedInfo().Config.InactiveThreshold != 0 {
		return nil, errors.New("log-processor must not expire while application replicas are offline")
	}
	cfg.Retention = jetstream.InterestPolicy
	cfg.Metadata[markerKey] = markerReady
	if _, err := js.UpdateStream(ctx, cfg); err != nil {
		return nil, fmt.Errorf("enable ingest InterestPolicy: %w", err)
	}
	return consumer, nil
}

func validateStream(cfg jetstream.StreamConfig) error {
	if len(cfg.Subjects) != 1 || cfg.Subjects[0] != logSubject || cfg.Storage != jetstream.FileStorage || cfg.NoAck || cfg.Sealed {
		return errors.New("LOGS requires exactly logs.>, file storage, publish acknowledgements and an unsealed stream")
	}
	if cfg.Mirror != nil || len(cfg.Sources) > 0 || cfg.SubjectTransform != nil || cfg.Template != "" {
		return errors.New("LOGS has incompatible mirror/source/subject transformation settings; inspect it before migration")
	}
	if cfg.AllowMsgTTL || cfg.AllowRollup || cfg.AllowMsgCounter || cfg.AllowMsgSchedules || cfg.ConsumerLimits.InactiveThreshold != 0 {
		return errors.New("LOGS cannot expire individual messages/consumers or allow rollup/counter/scheduled messages")
	}
	return nil
}

func validateConsumer(cfg jetstream.ConsumerConfig) error {
	if cfg.Durable != consumerName || cfg.FilterSubject != logSubject || len(cfg.FilterSubjects) > 0 || cfg.AckPolicy != jetstream.AckExplicitPolicy || cfg.DeliverSubject != "" || cfg.HeadersOnly {
		return errors.New("log-processor requires a durable pull consumer for logs.> with explicit acknowledgements and full payloads")
	}
	if len(cfg.BackOff) > 0 || cfg.InactiveThreshold != 0 || cfg.MemoryStorage || cfg.PauseUntil != nil || cfg.PriorityPolicy != jetstream.PriorityPolicyNone {
		return errors.New("log-processor has incompatible delivery/expiry/storage settings; refusing to reset its cursor")
	}
	if (cfg.MaxRequestBatch > 0 && cfg.MaxRequestBatch < fetchRows) || cfg.MaxRequestMaxBytes > 0 || (cfg.MaxRequestExpires > 0 && cfg.MaxRequestExpires < fetchWait) {
		return errors.New("log-processor pull limits do not allow the bounded ingest fetch")
	}
	return nil
}
