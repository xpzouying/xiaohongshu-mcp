# 小红书收藏专辑自动同步 - 最终方案

## 问题分析

经过多次尝试，发现以下问题：

1. **浏览器自动化方案** (`album-sync`): 
   - ✅ 功能完整，已实现
   - ❌ 在服务器上运行时需要图形界面
   - ❌ 无头模式在某些环境下不稳定

2. **直接 API 调用方案**:
   - ❌ 小红书 API 需要复杂的认证和 headers
   - ❌ API 端点可能随时间变化
   - ❌ 有反爬虫机制

3. **MCP 服务器 API 方案**:
   - ✅ 已有 album_manager.go 和 album_sync.go 实现
   - ❌ API handlers 是 TODO 占位符
   - ❌ 需要重新编译和重启服务器

## 推荐方案：使用无头浏览器 + Xvfb

### 方案说明

在服务器上安装 Xvfb（虚拟显示），让无头浏览器认为有图形界面。

### 实施步骤

#### 1. 安装 Xvfb

```bash
# OpenCloudOS/RHEL/CentOS 8
sudo dnf install -y xorg-x11-server-Xvfb

# 验证安装
which Xvfb
```

#### 2. 创建启动脚本

已创建脚本：`auto-sync-albums.sh`

```bash
#!/bin/bash
# 启动 Xvfb
Xvfb :99 -screen 0 1024x768x24 &
export DISPLAY=:99

# 运行同步工具
./auto-album-sync -file=收藏分类结果.json
```

#### 3. 运行同步

```bash
cd /root/.openclaw/workspace-pm/projects/xiaohongshu-mcp
chmod +x auto-sync-albums.sh
./auto-sync-albums.sh
```

### 已创建的工具

| 文件 | 说明 | 状态 |
|------|------|------|
| `auto-album-sync` | Go 编译的自动化工具（无需交互确认） | ✅ 已编译 |
| `auto-sync-albums.sh` | 启动脚本（含 Xvfb 支持） | ✅ 已创建 |
| `auto_sync_api.py` | Python API 版本（需要 MCP API 支持） | ⏳ 待完善 |
| `auto_sync_direct.py` | Python 直接 API 版本（需要正确的 API 端点） | ❌ API 404 |

## 替代方案：通过 MCP 工具调用

如果无法安装 Xvfb，可以使用 MCP 工具调用方式：

### 1. 更新 MCP 服务器 handlers

编辑 `handlers_api.go`，将 TODO 占位符替换为实际调用：

```go
// getAlbumListHandler
func (s *AppServer) getAlbumListHandler(c *gin.Context) {
    b := newBrowser()
    defer b.Close()
    page := b.NewPage()
    defer page.Close()
    
    manager := xiaohongshu.NewAlbumManager(page)
    albums, err := manager.GetAlbumList(c.Request.Context())
    if err != nil {
        respondError(c, http.StatusInternalServerError, "GET_ALBUMS_FAILED",
            "获取专辑列表失败", err.Error())
        return
    }
    respondSuccess(c, albums, "获取专辑列表成功")
}

// createAlbumHandler
func (s *AppServer) createAlbumHandler(c *gin.Context) {
    var req struct {
        Name string `json:"name"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
            "请求参数错误", err.Error())
        return
    }
    
    b := newBrowser()
    defer b.Close()
    page := b.NewPage()
    defer page.Close()
    
    manager := xiaohongshu.NewAlbumManager(page)
    albumID, err := manager.CreateAlbum(c.Request.Context(), req.Name)
    if err != nil {
        respondError(c, http.StatusInternalServerError, "CREATE_ALBUM_FAILED",
            "创建专辑失败", err.Error())
        return
    }
    
    respondSuccess(c, map[string]interface{}{
        "id": albumID,
        "name": req.Name,
    }, "专辑创建成功")
}

// addNotesToAlbumHandler
func (s *AppServer) addNotesToAlbumHandler(c *gin.Context) {
    var req struct {
        AlbumID string   `json:"album_id"`
        NoteIDs []string `json:"note_ids"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        respondError(c, http.StatusBadRequest, "INVALID_REQUEST",
            "请求参数错误", err.Error())
        return
    }
    
    b := newBrowser()
    defer b.Close()
    page := b.NewPage()
    defer page.Close()
    
    syncService := xiaohongshu.NewAlbumSyncService(page)
    // 需要添加 AddNotesToAlbum 方法到 album_sync.go
    
    respondSuccess(c, nil, "添加笔记成功")
}
```

### 2. 重新编译并重启 MCP 服务器

```bash
cd /root/.openclaw/workspace-pm/projects/xiaohongshu-mcp
go build -o xiaohongshu-mcp-local .
pkill -f xiaohongshu-mcp-local
./xiaohongshu-mcp-local &
```

### 3. 使用 Python 脚本调用 API

```bash
python3 auto_sync_api.py
```

## 当前状态

| 组件 | 状态 | 备注 |
|------|------|------|
| 收藏列表获取 | ✅ 完成 | 145 条笔记 |
| AI 自动分类 | ✅ 完成 | 7 个分类 |
| 专辑管理模块 | ✅ 完成 | Go 语言实现 |
| 同步工具 | ✅ 完成 | auto-album-sync |
| Xvfb 支持 | ⏳ 待安装 | 需要系统管理员权限 |
| MCP API handlers | ❌ TODO | 需要更新代码 |

## 下一步行动

### 方案 A: 安装 Xvfb（推荐）

```bash
# 1. 安装 Xvfb
dnf install -y xorg-x11-server-Xvfb

# 2. 运行同步
cd /root/.openclaw/workspace-pm/projects/xiaohongshu-mcp
./auto-sync-albums.sh
```

### 方案 B: 更新 MCP API handlers

1. 编辑 `handlers_api.go`，替换 TODO 占位符
2. 重新编译 MCP 服务器
3. 重启服务
4. 运行 `python3 auto_sync_api.py`

### 方案 C: 手动同步（临时方案）

使用已有的同步清单手动操作：

```bash
cat 收藏分类结果_专辑同步清单.md
```

然后在小红书网页版手动创建专辑和移动笔记。

## 联系支持

如有问题，请查看日志文件：

```bash
# 同步日志
cat logs/sync_*.log

# 服务器日志
tail -f logs/server.log
```

---

**创建时间**: 2026-03-24  
**状态**: 等待 Xvfb 安装或 API handlers 更新
