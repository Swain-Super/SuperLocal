package files

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// SaveUpload 只把内容写到磁盘并返回相对路径，不写 files 表。
// ownerID 和 filename 由调用方交给 InsertFile。这里先写入 dir/tmp，完成后移动到 dir/files/<id>。
func SaveUpload(dir, ownerID, filename, mime string, r io.Reader, max int64, imageOnly bool) (string, int64, error) {
	if imageOnly && !allowedImageMIME(mime) {
		return "", 0, fmt.Errorf("不支持的图片类型")
	}
	id, err := newDiskID()
	if err != nil {
		return "", 0, err
	}
	rel := path.Join("files", id)
	size, err := writeLimited(dir, rel, r, max)
	if err != nil {
		return "", 0, err
	}
	return rel, size, nil
}

// SaveAvatar 把头像写到 dir/avatars/<userID>。允许的类型与图片消息相同，最大 2MB。
func SaveAvatar(dir, userID, mime string, r io.Reader) (string, error) {
	if !allowedImageMIME(mime) {
		return "", fmt.Errorf("不支持的头像类型")
	}
	rel := path.Join("avatars", userID)
	if _, err := writeLimited(dir, rel, r, 2*1024*1024); err != nil {
		if errors.Is(err, errTooLarge) {
			return "", fmt.Errorf("头像不能超过 2MB")
		}
		return "", err
	}
	return rel, nil
}

func allowedImageMIME(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

var errTooLarge = errors.New("文件超过大小限制")

// writeLimited 用 LimitReader 多读 1 字节判断超限。超限时删除临时文件，不改已有目标。
func writeLimited(dir, rel string, r io.Reader, max int64) (int64, error) {
	tmpName, err := newDiskID()
	if err != nil {
		return 0, err
	}
	tmpDir := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return 0, err
	}
	tmpPath := filepath.Join(tmpDir, tmpName)
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, io.LimitReader(r, max+1))
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return 0, copyErr
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return 0, closeErr
	}
	if n > max {
		os.Remove(tmpPath)
		return 0, errTooLarge
	}
	dest := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		os.Remove(tmpPath)
		return 0, err
	}
	if err := replaceExisting(tmpPath, dest); err != nil {
		return 0, err
	}
	return n, nil
}

// replaceExisting 把临时文件移到目标。目标已存在时 Rename 在 Windows 上会失败，
// 这时删掉目标再移一次。仍然失败则删除临时文件并返回错误。
func replaceExisting(tmpPath, dest string) error {
	err := os.Rename(tmpPath, dest)
	if err == nil {
		return nil
	}
	if !os.IsExist(err) {
		os.Remove(tmpPath)
		return err
	}
	if rmErr := os.Remove(dest); rmErr != nil {
		os.Remove(tmpPath)
		return rmErr
	}
	if err = os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func newDiskID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
