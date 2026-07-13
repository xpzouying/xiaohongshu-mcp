package account

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type MigrationOptions struct {
	Root       string
	Candidates []string
	Clock      func() time.Time
}

type MigrationResult struct {
	Migrated       bool
	AccountID      string
	ChecksumPrefix string
}

func MigrateLegacy(ctx context.Context, options MigrationOptions) (MigrationResult, error) {
	if err := ctx.Err(); err != nil {
		return MigrationResult{}, canceledError(err)
	}
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return MigrationResult{}, newError(CodePersistenceFailed, "数据目录无效", false, err)
	}
	registryPath := filepath.Join(root, "accounts.json")
	if _, err := os.Stat(registryPath); err == nil {
		return MigrationResult{}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return MigrationResult{}, newError(CodePersistenceFailed, "检查注册表失败", true, err)
	}

	var sourceData []byte
	var sourceHash [sha256.Size]byte
	found := false
	for _, candidate := range options.Candidates {
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(candidate)
		if err != nil {
			return MigrationResult{}, newError(CodePersistenceFailed, "读取旧 Cookie 失败", true, err)
		}
		if !json.Valid(data) {
			return MigrationResult{}, newError(CodePersistenceFailed, "旧 Cookie 格式无效", false, nil)
		}
		hash := sha256.Sum256(data)
		if found && hash != sourceHash {
			return MigrationResult{}, newError(CodeLegacyCookieAmbiguous, "检测到内容不同的旧 Cookie", false, nil)
		}
		if !found {
			sourceData, sourceHash = data, hash
			found = true
		}
	}
	if !found {
		doc := registryDocument{SchemaVersion: registrySchemaVersion, Accounts: []Account{}}
		data, _ := json.MarshalIndent(doc, "", "  ")
		if err := atomicWrite(root, registryPath, append(data, '\n')); err != nil {
			return MigrationResult{}, newError(CodePersistenceFailed, "创建空注册表失败", true, err)
		}
		return MigrationResult{}, nil
	}
	store, err := NewFileCookieStore(root)
	if err != nil {
		return MigrationResult{}, err
	}
	if err := store.Save(ctx, "default", sourceData); err != nil {
		return MigrationResult{}, err
	}
	stored, err := store.Load(ctx, "default")
	if err != nil || sha256.Sum256(stored) != sourceHash {
		return MigrationResult{}, newError(CodePersistenceFailed, "迁移 Cookie 校验失败", true, err)
	}
	now := time.Now().UTC()
	if options.Clock != nil {
		now = options.Clock().UTC()
	}
	defaultID := "default"
	doc := registryDocument{SchemaVersion: registrySchemaVersion, DefaultAccountID: &defaultID, Accounts: []Account{{ID: defaultID, DisplayName: "Default Account", Status: StatusNeedsLogin, CreatedAt: now, UpdatedAt: now}}}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return MigrationResult{}, newError(CodePersistenceFailed, "编码迁移注册表失败", true, err)
	}
	if err := atomicWrite(root, registryPath, append(data, '\n')); err != nil {
		return MigrationResult{}, newError(CodePersistenceFailed, "保存迁移注册表失败", true, err)
	}
	return MigrationResult{Migrated: true, AccountID: defaultID, ChecksumPrefix: string([]byte{hexDigit(sourceHash[0] >> 4), hexDigit(sourceHash[0] & 15), hexDigit(sourceHash[1] >> 4), hexDigit(sourceHash[1] & 15)})}, nil
}

func hexDigit(value byte) byte {
	if value < 10 {
		return '0' + value
	}
	return 'a' + value - 10
}
