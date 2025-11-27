package ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sandwichfarm/nophr/internal/storage"
)

// BackupManager handles database backup operations
type BackupManager struct {
	storage *storage.Storage
	logger  *Logger
	dbPath  string
}

// NewBackupManager creates a new backup manager
func NewBackupManager(st *storage.Storage, logger *Logger, dbPath string) *BackupManager {
	return &BackupManager{
		storage: st,
		logger:  logger.WithComponent("backup"),
		dbPath:  dbPath,
	}
}

// Backup creates a backup of the database
func (b *BackupManager) Backup(ctx context.Context, destPath string) error {
	start := time.Now()

	b.logger.Info("starting database backup", "destination", destPath)

	// Get the database file path
	driver := b.storage.Driver()
	var sourcePath string

	switch driver {
	case "sqlite":
		sourcePath = b.dbPath
		if sourcePath == "" {
			b.logger.Error("database path not configured")
			return fmt.Errorf("database path not set")
		}
	case "lmdb":
		b.logger.Error("backup not yet implemented for LMDB")
		return fmt.Errorf("LMDB backup not implemented")
	default:
		return fmt.Errorf("unsupported driver: %s", driver)
	}

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		b.logger.LogBackupOperation("create directory", destPath, 0, err)
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy the database file
	size, err := b.copyFile(sourcePath, destPath)
	if err != nil {
		b.logger.LogBackupOperation("backup", destPath, size, err)
		return fmt.Errorf("failed to copy database: %w", err)
	}

	b.logger.LogBackupOperation("backup", destPath, size, nil)
	b.logger.Info("database backup completed",
		"destination", destPath,
		"size_mb", float64(size)/1024/1024,
		"duration_ms", time.Since(start).Milliseconds())

	return nil
}

// BackupWithConfig creates a backup using config paths
func (b *BackupManager) BackupWithConfig(ctx context.Context, sourcePath, destPath string) error {
	start := time.Now()

	b.logger.Info("starting database backup",
		"source", sourcePath,
		"destination", destPath)

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		b.logger.LogBackupOperation("create directory", destPath, 0, err)
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy the database file
	size, err := b.copyFile(sourcePath, destPath)
	if err != nil {
		b.logger.LogBackupOperation("backup", destPath, size, err)
		return fmt.Errorf("failed to copy database: %w", err)
	}

	b.logger.LogBackupOperation("backup", destPath, size, nil)
	b.logger.Info("database backup completed",
		"source", sourcePath,
		"destination", destPath,
		"size_mb", float64(size)/1024/1024,
		"duration_ms", time.Since(start).Milliseconds())

	return nil
}

// Restore restores a database from backup
func (b *BackupManager) Restore(ctx context.Context, backupPath, destPath string) error {
	start := time.Now()

	b.logger.Info("starting database restore",
		"backup", backupPath,
		"destination", destPath)

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupPath)
	}

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy the backup file to destination
	size, err := b.copyFile(backupPath, destPath)
	if err != nil {
		b.logger.LogBackupOperation("restore", destPath, size, err)
		return fmt.Errorf("failed to restore database: %w", err)
	}

	b.logger.LogBackupOperation("restore", destPath, size, nil)
	b.logger.Info("database restore completed",
		"backup", backupPath,
		"destination", destPath,
		"size_mb", float64(size)/1024/1024,
		"duration_ms", time.Since(start).Milliseconds())

	return nil
}

// copyFile copies a file from src to dst
func (b *BackupManager) copyFile(src, dst string) (int64, error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	size, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return size, fmt.Errorf("failed to copy file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := destFile.Sync(); err != nil {
		return size, fmt.Errorf("failed to sync file: %w", err)
	}

	return size, nil
}

// PeriodicBackup runs periodic backups
type PeriodicBackup struct {
	manager    *BackupManager
	sourcePath string
	destDir    string
	interval   time.Duration
	logger     *Logger
	stopChan   chan struct{}
}

// NewPeriodicBackup creates a new periodic backup handler
func NewPeriodicBackup(manager *BackupManager, sourcePath, destDir string, interval time.Duration, logger *Logger) *PeriodicBackup {
	return &PeriodicBackup{
		manager:    manager,
		sourcePath: sourcePath,
		destDir:    destDir,
		interval:   interval,
		logger:     logger.WithComponent("periodic-backup"),
		stopChan:   make(chan struct{}),
	}
}

// Start begins periodic backups
func (p *PeriodicBackup) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.logger.Info("periodic backup started",
		"source", p.sourcePath,
		"destination", p.destDir,
		"interval", p.interval)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("periodic backup stopped")
			return
		case <-p.stopChan:
			p.logger.Info("periodic backup stopped")
			return
		case <-ticker.C:
			p.logger.Debug("running periodic backup")

			// Generate timestamped backup filename
			timestamp := time.Now().Format("20060102-150405")
			backupPath := filepath.Join(p.destDir, fmt.Sprintf("nophr-backup-%s.db", timestamp))

			err := p.manager.BackupWithConfig(ctx, p.sourcePath, backupPath)
			if err != nil {
				p.logger.Error("periodic backup failed", "error", err)
			} else {
				p.logger.Info("periodic backup completed", "path", backupPath)
			}
		}
	}
}

// Stop stops the periodic backup
func (p *PeriodicBackup) Stop() {
	close(p.stopChan)
}

// CleanOldBackups removes backups older than the specified age
func CleanOldBackups(backupDir string, maxAge time.Duration, logger *Logger) error {
	logger.Info("cleaning old backups", "directory", backupDir, "max_age", maxAge)

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	var deleted int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if it's a backup file
		if !isBackupFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			logger.Warn("failed to get file info", "file", entry.Name(), "error", err)
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(backupDir, entry.Name())
			if err := os.Remove(path); err != nil {
				logger.Warn("failed to delete old backup", "file", path, "error", err)
			} else {
				logger.Info("deleted old backup", "file", path, "age", time.Since(info.ModTime()))
				deleted++
			}
		}
	}

	logger.Info("old backup cleanup completed", "deleted", deleted)
	return nil
}

// isBackupFile checks if a filename is a backup file
func isBackupFile(name string) bool {
	return filepath.Ext(name) == ".db" &&
		(len(name) > 14 && name[:14] == "nophr-backup-")
}
