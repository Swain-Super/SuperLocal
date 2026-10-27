package cleanup

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"superlocal/internal/store"
)

const retention = 48 * time.Hour

// 过期文件：消息的 file_id 指向它，或它的 message_id 指向过期消息，
// 或尚未绑定且自身已过期。created_at 为 Unix 毫秒，刚好 48 小时保留。
const expiredFilesWhere = `
	id IN (
		SELECT file_id FROM messages WHERE created_at < ? AND file_id != ''
	)
	OR message_id IN (
		SELECT id FROM messages WHERE created_at < ?
	)
	OR (message_id = '' AND created_at < ?)
`

// Run 删除早于 now-48h 的消息，以及这些消息关联的文件和磁盘内容。
// 关联包括 messages.file_id 和 files.message_id。
// message_id 为空且早于 now-48h 的文件一并删除。
// 不删除用户、成员关系，也不删除 avatars 目录下的文件。
func Run(db *store.DB, dataDir string, now time.Time) error {
	if strings.TrimSpace(dataDir) == "" {
		return fmt.Errorf("数据目录为空")
	}
	cutoff := now.Add(-retention).UnixMilli()
	paths, err := expiredPaths(db.SQL(), cutoff)
	if err != nil {
		return err
	}
	for _, rel := range paths {
		if err := removeStoredFile(dataDir, rel); err != nil {
			return err
		}
	}
	tx, err := db.SQL().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM files WHERE `+expiredFilesWhere, cutoff, cutoff, cutoff); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE created_at < ?`, cutoff); err != nil {
		return err
	}
	return tx.Commit()
}

func expiredPaths(sqlDB *sql.DB, cutoff int64) ([]string, error) {
	rows, err := sqlDB.Query(`SELECT path FROM files WHERE `+expiredFilesWhere, cutoff, cutoff, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			return nil, err
		}
		paths = append(paths, rel)
	}
	return paths, rows.Err()
}

// removeStoredFile 只删除数据目录内的普通文件。头像和越界路径留在磁盘上。
func removeStoredFile(dataDir, stored string) error {
	if stored == "" || filepath.IsAbs(stored) {
		return nil
	}
	clean := filepath.Clean(filepath.FromSlash(stored))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil
	}
	if isAvatarPath(clean) {
		return nil
	}
	root, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}
	dest := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || isAvatarPath(rel) {
		return nil
	}
	info, err := os.Lstat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isAvatarPath(p string) bool {
	slash := filepath.ToSlash(p)
	return slash == "avatars" || strings.HasPrefix(slash, "avatars/")
}
