package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Advisory lock keys for migration (two int32 keys for pg_advisory_lock).
const (
	migrationLockKey1 = 8837291
	migrationLockKey2 = 20260710
)

// RunMigrateWithLock acquires a PostgreSQL advisory lock, runs AutoMigrate, then releases.
func RunMigrateWithLock(ctx context.Context, db *gorm.DB, timeout time.Duration, run func(*gorm.DB) error) error {
	if db == nil {
		return fmt.Errorf("migration lock: db is nil")
	}
	if run == nil {
		return fmt.Errorf("migration lock: run func is nil")
	}
	driver := ""
	if db.Dialector != nil {
		driver = db.Dialector.Name()
	}
	if driver != "postgres" {
		// MySQL / others: run without advisory lock (single-instance convention).
		return run(db)
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	lockCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// PostgreSQL advisory locks belong to the session. Keep the session checked
	// out until the migration and unlock have both finished.
	return db.WithContext(lockCtx).Connection(func(conn *gorm.DB) (err error) {
		for {
			acquired, lockErr := tryAdvisoryLock(lockCtx, conn)
			if lockErr != nil {
				return fmt.Errorf("migration lock: acquire: %w", lockErr)
			}
			if acquired {
				break
			}
			select {
			case <-lockCtx.Done():
				return fmt.Errorf("migration lock: wait: %w", lockCtx.Err())
			case <-time.After(500 * time.Millisecond):
			}
		}
		defer func() {
			unlockCtx, cancelUnlock := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancelUnlock()
			if unlockErr := releaseAdvisoryLock(unlockCtx, conn); unlockErr != nil {
				err = errors.Join(err, unlockErr)
			}
		}()
		return run(conn.WithContext(db.Statement.Context))
	})
}

func tryAdvisoryLock(ctx context.Context, db *gorm.DB) (bool, error) {
	var ok bool
	err := db.WithContext(ctx).Raw(
		"SELECT pg_try_advisory_lock(?, ?)", migrationLockKey1, migrationLockKey2,
	).Scan(&ok).Error
	return ok, err
}

func releaseAdvisoryLock(ctx context.Context, db *gorm.DB) error {
	var released bool
	if err := db.WithContext(ctx).Raw("SELECT pg_advisory_unlock(?, ?)", migrationLockKey1, migrationLockKey2).Scan(&released).Error; err != nil {
		return fmt.Errorf("migration lock: release: %w", err)
	}
	if !released {
		return errors.New("migration lock: the database session did not hold the lock")
	}
	return nil
}
