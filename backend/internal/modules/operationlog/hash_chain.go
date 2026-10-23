package operationlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// hashChainVersion is the audit integrity schema version.
const hashChainVersion = 1

var chainPartitionLocks sync.Map

// appendHashChain computes prev_hash / entry_hash for a new log row. It
// returns a release function so in-process partition locks remain held until
// the caller has inserted the row and committed its transaction.
func (s *Service) appendHashChain(tx *gorm.DB, row *OperationLog) (func(), error) {
	if row == nil {
		return func() {}, nil
	}
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	row.CreatedAt = row.CreatedAt.UTC().Truncate(time.Microsecond)
	row.HashVersion = hashChainVersion
	lockedPartitions := make(map[string]struct{})
	unlockers := make([]func(), 0, 2)
	release := func() {
		for i := len(unlockers) - 1; i >= 0; i-- {
			unlockers[i]()
		}
		unlockers = nil
	}
	completed := false
	defer func() {
		if !completed {
			release()
		}
	}()

	for {
		partition := chainPartition(row.TenantID, row.CreatedAt)
		row.ChainPartition = partition
		if _, ok := lockedPartitions[partition]; !ok {
			// PostgreSQL holds its advisory lock until transaction completion.
			// Taking a process mutex here would let a waiting transaction hold
			// that mutex while the lock owner tries to append its next audit row.
			if !isPostgres(tx) {
				unlockers = append(unlockers, lockLocalHashChainScope(partition))
			}
			lockedPartitions[partition] = struct{}{}
		}
		if err := lockHashChainScope(tx, partition); err != nil {
			return nil, err
		}

		var prev OperationLog
		query := tx.Where("chain_partition = ?", partition).Order("created_at DESC, id DESC")
		if isPostgres(tx) {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		err := query.First(&prev).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row.PrevHash = ""
			row.EntryHash = computeEntryHash(row.PrevHash, row)
			completed = true
			return release, nil
		}
		if err != nil {
			return nil, err
		}
		if row.CreatedAt.After(prev.CreatedAt) {
			row.PrevHash = strings.TrimSpace(prev.EntryHash)
			row.EntryHash = computeEntryHash(row.PrevHash, row)
			completed = true
			return release, nil
		}

		row.CreatedAt = prev.CreatedAt.Add(time.Microsecond).UTC()
		if chainPartition(row.TenantID, row.CreatedAt) == partition {
			row.PrevHash = strings.TrimSpace(prev.EntryHash)
			row.EntryHash = computeEntryHash(row.PrevHash, row)
			completed = true
			return release, nil
		}
		// The monotonic timestamp crossed UTC midnight. Continue under the new
		// partition's lock and derive its own predecessor instead of linking
		// independent daily chains together.
	}
}

func lockHashChainScope(tx *gorm.DB, partition string) error {
	if tx == nil || !isPostgres(tx) {
		return nil
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?)::bigint)", partition).Error
}

func lockLocalHashChainScope(partition string) func() {
	actual, _ := chainPartitionLocks.LoadOrStore(partition, &sync.Mutex{})
	mu := actual.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func isPostgres(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres"
}

func chainPartition(tenantID int64, at time.Time) string {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return fmt.Sprintf("t%d:%s", tenantID, at.UTC().Format("2006-01-02"))
}

func computeEntryHash(prevHash string, row *OperationLog) string {
	summary := sha256.Sum256([]byte(strings.TrimSpace(row.Message)))
	payload := strings.Join([]string{
		prevHash,
		fmt.Sprintf("%d", row.TenantID),
		row.Action,
		row.Resource,
		row.ResourceID,
		row.Status,
		row.RequestID,
		row.CreatedAt.UTC().Format(time.RFC3339Nano),
		hex.EncodeToString(summary[:]),
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// VerifyChain checks hash chain integrity for a partition/date range.
func (s *Service) VerifyChain(ctx context.Context, tenantID int64, from, to time.Time) (int, *time.Time, error) {
	if s == nil || s.DB == nil {
		return 0, nil, fmt.Errorf("operationlog: no db")
	}
	tx := s.DB.WithContext(ctx).Model(&OperationLog{}).
		Where("tenant_id = ? AND created_at >= ? AND created_at <= ?", tenantID, from, to).
		Order("created_at ASC, id ASC")
	var rows []OperationLog
	if err := tx.Find(&rows).Error; err != nil {
		return 0, nil, err
	}
	prev := ""
	prevPartition := ""
	for i, row := range rows {
		partition := strings.TrimSpace(row.ChainPartition)
		if partition == "" {
			partition = chainPartition(row.TenantID, row.CreatedAt)
		}
		if partition != prevPartition {
			// Each tenant/day partition starts from its own genesis. A date
			// range may contain multiple independent chains.
			prev = ""
			prevPartition = partition
		}
		expected := computeEntryHash(prev, &row)
		if strings.TrimSpace(row.EntryHash) == "" {
			continue
		}
		if row.EntryHash != expected || (i > 0 && row.PrevHash != prev) {
			t := row.CreatedAt
			return i, &t, fmt.Errorf("audit chain mismatch at index %d", i)
		}
		prev = row.EntryHash
	}
	return len(rows), nil, nil
}
