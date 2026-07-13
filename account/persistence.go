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

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	if err := rejectSymlink(dir); err != nil {
		return err
	}
	if err := rejectSymlink(path); err != nil {
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
