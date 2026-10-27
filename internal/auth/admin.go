package auth

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"superlocal/internal/store"
)

const adminUsername = "admin"

// EnsureAdmin 在空库上创建管理员；凭证文件丢失时生成新密码并覆盖哈希。
// 文件仍在时返回其中的用户名和密码，供启动时打印。
func EnsureAdmin(db *store.DB, dataDir string) (string, string, error) {
	path := secretPath(dataDir)
	username, password, err := readAdminSecret(path)
	if err == nil {
		return username, password, nil
	}
	if !os.IsNotExist(err) {
		return "", "", err
	}
	password, err = randomAdminPassword()
	if err != nil {
		return "", "", err
	}
	if err := saveAdminPassword(db, adminUsername, password); err != nil {
		return "", "", err
	}
	if err := writeAdminSecret(path, adminUsername, password); err != nil {
		return "", "", err
	}
	return adminUsername, password, nil
}

func CheckAdmin(db *store.DB, username, password string) error {
	var hash string
	err := db.SQL().QueryRow(
		`SELECT password_hash FROM admin_user WHERE username = ?`, username,
	).Scan(&hash)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func ChangeAdminPassword(db *store.DB, dataDir, password string) error {
	path := secretPath(dataDir)
	username := adminUsername
	if existing, _, err := readAdminSecret(path); err == nil && existing != "" {
		username = existing
	}
	if err := saveAdminPassword(db, username, password); err != nil {
		return err
	}
	return writeAdminSecret(path, username, password)
}

func saveAdminPassword(db *store.DB, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return withLockRetry(db, func() error {
		_, execErr := db.SQL().Exec(
			`INSERT INTO admin_user (username, password_hash) VALUES (?, ?)
			 ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash`,
			username, string(hash),
		)
		return execErr
	})
}

func secretPath(dataDir string) string {
	return filepath.Join(dataDir, "admin.secret")
}

func readAdminSecret(path string) (string, string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 || lines[0] == "" || lines[1] == "" {
		return "", "", fmt.Errorf("admin.secret 格式无效")
	}
	return lines[0], lines[1], nil
}

func writeAdminSecret(path, username, password string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := fmt.Fprintf(f, "%s\n%s\n", username, password)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

func randomAdminPassword() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// withLockRetry 在遇到 database is locked 时重试 3 次，间隔 50 毫秒。
// store.withLockRetry 未导出，写入仍通过 (*store.DB).SQL()。
func withLockRetry(db *store.DB, fn func() error) error {
	err := fn()
	for i := 0; i < 3 && isDatabaseLocked(err); i++ {
		time.Sleep(50 * time.Millisecond)
		err = fn()
	}
	return err
}

func isDatabaseLocked(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}
