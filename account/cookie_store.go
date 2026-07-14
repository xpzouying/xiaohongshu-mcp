package account

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type FileCookieStore struct{ root string }

type fileCookieRemoval struct {
	dirFD  int
	path   string
	staged string
}

func (r *fileCookieRemoval) Commit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	if r.dirFD < 0 {
		return nil
	}
	if err := unix.Fsync(r.dirFD); err != nil {
		return newError(CodePersistenceFailed, "提交 Cookie 删除失败", true, err)
	}
	return nil
}

func (r *fileCookieRemoval) Rollback(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return canceledError(err)
	}
	if r.dirFD < 0 || r.staged == "" {
		return nil
	}
	if err := unix.Renameat(r.dirFD, r.staged, r.dirFD, r.path); err != nil {
		return newError(CodePersistenceFailed, "恢复账号 Cookie 失败", true, err)
	}
	if err := unix.Fsync(r.dirFD); err != nil {
		return newError(CodePersistenceFailed, "恢复账号 Cookie 失败", true, err)
	}
	r.staged = ""
	return nil
}

func (r *fileCookieRemoval) Complete() error {
	if r.dirFD < 0 {
		return nil
	}
	defer func() {
		unix.Close(r.dirFD)
		r.dirFD = -1
	}()
	if r.staged != "" {
		if err := unix.Unlinkat(r.dirFD, r.staged, 0); err != nil && !errors.Is(err, unix.ENOENT) {
			return newError(CodePersistenceFailed, "清理账号 Cookie 失败", true, err)
		}
	}
	if err := unix.Fsync(r.dirFD); err != nil {
		return newError(CodePersistenceFailed, "清理账号 Cookie 失败", true, err)
	}
	r.staged = ""
	return nil
}

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
	data, err := readFileNoFollow(s.root, path, false)
	if err != nil && ErrorCode(err) != CodePersistenceFailed {
		staged := path + ".removing"
		data, err = readFileNoFollow(s.root, staged, false)
	}
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
	if err := atomicWrite(s.root, path, data); err != nil {
		return newError(CodePersistenceFailed, "保存账号 Cookie 失败", true, err)
	}
	return nil
}

func (s *FileCookieStore) Delete(ctx context.Context, accountID string) error {
	removal, err := s.StageRemove(ctx, accountID)
	if err != nil {
		return err
	}
	if err := removal.Commit(ctx); err != nil {
		return errors.Join(err, removal.Rollback(context.WithoutCancel(ctx)))
	}
	return removal.Complete()
}

func (s *FileCookieStore) StageRemove(ctx context.Context, accountID string) (CookieRemoval, error) {
	if err := ctx.Err(); err != nil {
		return nil, canceledError(err)
	}
	path, err := s.Path(accountID)
	if err != nil {
		return nil, err
	}
	dirFD, name, err := openParent(s.root, path, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &fileCookieRemoval{dirFD: -1}, nil
		}
		return nil, newError(CodePersistenceFailed, "检查账号 Cookie 失败", true, err)
	}
	staged := name + ".removing"
	stagedExists, err := pathExistsAt(dirFD, staged)
	if err != nil {
		unix.Close(dirFD)
		return nil, newError(CodePersistenceFailed, "检查暂存 Cookie 失败", true, err)
	}
	pathExists, err := pathExistsAt(dirFD, name)
	if err != nil {
		unix.Close(dirFD)
		return nil, newError(CodePersistenceFailed, "检查账号 Cookie 失败", true, err)
	}
	if stagedExists {
		if pathExists {
			unix.Close(dirFD)
			return nil, newError(CodePersistenceFailed, "暂存账号 Cookie 冲突", true, nil)
		}
		return &fileCookieRemoval{dirFD: dirFD, path: name, staged: staged}, nil
	}
	if err := unix.Renameat(dirFD, name, dirFD, staged); errors.Is(err, unix.ENOENT) {
		return &fileCookieRemoval{dirFD: dirFD, path: name}, nil
	} else if err != nil {
		unix.Close(dirFD)
		return nil, newError(CodePersistenceFailed, "暂存账号 Cookie 失败", true, err)
	}
	return &fileCookieRemoval{dirFD: dirFD, path: name, staged: staged}, nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}

func canceledError(err error) error {
	return newError(CodeOperationCanceled, "操作已取消", true, err)
}
