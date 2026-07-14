package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

var errNotRegular = errors.New("敏感文件不是普通文件")

func accountPath(root, accountID, name string) (string, error) {
	if err := ValidateAccountID(accountID); err != nil {
		return "", err
	}
	accountsRoot, err := filepath.Abs(filepath.Join(root, "accounts"))
	if err != nil {
		return "", newError(CodeInvalidAccountID, "账号目录无效", false, err)
	}
	candidate, err := filepath.Abs(filepath.Join(accountsRoot, accountID, name))
	if err != nil {
		return "", newError(CodeInvalidAccountID, "账号路径无效", false, err)
	}
	rel, err := filepath.Rel(accountsRoot, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", newError(CodeInvalidAccountID, "账号路径越界", false, err)
	}
	return candidate, nil
}

func relativePath(root, path string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", newError(CodePersistenceFailed, "持久化路径越界", false, err)
	}
	return rel, nil
}

// openTrustedDir 从根目录逐级打开目录，避免祖先路径在检查后被替换。
func openTrustedDir(path string, create bool) (int, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return -1, err
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(abs, string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil && create && errors.Is(openErr, unix.ENOENT) {
			if mkdirErr := unix.Mkdirat(fd, part, 0700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				unix.Close(fd)
				return -1, mkdirErr
			}
			next, openErr = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		}
		unix.Close(fd)
		if openErr != nil {
			return -1, openErr
		}
		fd = next
	}
	return fd, nil
}

func openParent(root, path string, create bool) (int, string, error) {
	rel, err := relativePath(root, path)
	if err != nil {
		return -1, "", err
	}
	fd, err := openTrustedDir(root, false)
	if err != nil {
		return -1, "", err
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil && create && errors.Is(openErr, unix.ENOENT) {
			if mkdirErr := unix.Mkdirat(fd, part, 0700); mkdirErr != nil && !errors.Is(mkdirErr, unix.EEXIST) {
				unix.Close(fd)
				return -1, "", mkdirErr
			}
			next, openErr = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		}
		unix.Close(fd)
		if openErr != nil {
			return -1, "", openErr
		}
		fd = next
	}
	return fd, parts[len(parts)-1], nil
}

func atomicWrite(root, path string, data []byte) error {
	dirFD, name, err := openParent(root, path, true)
	if err != nil {
		return err
	}
	defer unix.Close(dirFD)
	if err := unix.Fchmod(dirFD, 0700); err != nil {
		return err
	}

	var tmp string
	var fd int
	for i := 0; i < 100; i++ {
		tmp = fmt.Sprintf(".tmp-%d-%d", os.Getpid(), i)
		fd, err = unix.Openat(dirFD, tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
		if !errors.Is(err, unix.EEXIST) {
			break
		}
	}
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), tmp)
	ok := false
	defer func() {
		if !ok {
			_ = unix.Unlinkat(dirFD, tmp, 0)
		}
	}()
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = unix.Renameat(dirFD, tmp, dirFD, name); err != nil {
		return err
	}
	ok = true
	return unix.Fsync(dirFD)
}

func strictDecode(data []byte, target any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("存在额外 JSON 内容")
	}
	return nil
}

func readFileNoFollow(root, path string, requirePrivate bool) ([]byte, error) {
	dirFD, name, err := openParent(root, path, false)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dirFD)
	fd, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, newError(CodePersistenceFailed, "拒绝符号链接路径", false, err)
		}
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errNotRegular
	}
	if requirePrivate && info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("敏感文件权限过宽: %o", info.Mode().Perm())
	}
	return io.ReadAll(file)
}

func pathExistsAt(dirFD int, name string) (bool, error) {
	var stat unix.Stat_t
	err := unix.Fstatat(dirFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		if stat.Mode&unix.S_IFMT == unix.S_IFLNK {
			return false, newError(CodePersistenceFailed, "拒绝符号链接路径", false, nil)
		}
		return true, nil
	}
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	return false, err
}
