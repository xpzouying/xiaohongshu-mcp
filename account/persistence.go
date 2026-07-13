package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return newError(CodePersistenceFailed, "拒绝符号链接路径", false, nil)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func securePath(root, path string, createParents bool) error {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return newError(CodePersistenceFailed, "持久化路径越界", false, err)
	}
	current := root
	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		last := i == len(parts)-1
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return newError(CodePersistenceFailed, "拒绝符号链接路径", false, nil)
			}
			if !last && !info.IsDir() {
				return newError(CodePersistenceFailed, "持久化父路径不是目录", false, nil)
			}
			continue
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		if !createParents || last {
			continue
		}
		if err := os.Mkdir(current, 0700); err != nil {
			return err
		}
	}
	return nil
}

func atomicWrite(root, path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := securePath(root, path, true); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	if err := securePath(root, path, false); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Chmod(tmp, 0600); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
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
