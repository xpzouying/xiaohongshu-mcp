package account

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type FileCookieStore struct{ root string }

func NewFileCookieStore(root string) (*FileCookieStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, newError(CodePersistenceFailed, "数据目录无效", false, err)
	}
	return &FileCookieStore{root: abs}, nil
}

func (s *FileCookieStore) Path(accountID string) (string, error) {
	return accountPath(s.root, accountID, "cookies.json")
}

func (s *FileCookieStore) Load(ctx context.Context, accountID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, canceledError(err)
	}
	path, err := s.Path(accountID)
	if err != nil {
		return nil, err
	}
	if err := rejectSymlink(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, newError(CodeCookieNotFound, "账号 Cookie 不存在", false, nil)
	}
	if err != nil {
		return nil, newError(CodePersistenceFailed, "读取账号 Cookie 失败", true, err)
	}
	if !json.Valid(data) {
		return nil, newError(CodePersistenceFailed, "账号 Cookie 格式无效", false, nil)
	}
	return data, nil
}

func (s *FileCookieStore) Save(ctx context.Context, accountID string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	if !json.Valid(data) {
		return newError(CodePersistenceFailed, "Cookie 必须是合法 JSON", false, nil)
	}
	path, err := s.Path(accountID)
	if err != nil {
		return err
	}
	if err := atomicWrite(path, data); err != nil {
		return newError(CodePersistenceFailed, "保存账号 Cookie 失败", true, err)
	}
	return nil
}

func (s *FileCookieStore) Delete(ctx context.Context, accountID string) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	path, err := s.Path(accountID)
	if err != nil {
		return err
	}
	if err := rejectSymlink(path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return newError(CodePersistenceFailed, "删除账号 Cookie 失败", true, err)
	}
	return nil
}

func canceledError(err error) error {
	return newError(CodeOperationCanceled, "操作已取消", true, err)
}
