@echo off
chcp 65001 >nul
title xiaohongshu-mcp Manager

echo ========================================
echo   xiaohongshu-mcp Manager
echo ========================================
echo.

if not exist "manager.exe" (
    echo [ERROR] manager.exe not found, building...
    go build -o manager.exe ./cmd/manager/
    if errorlevel 1 (
        echo [ERROR] Build failed!
        pause
        exit /b 1
    )
    echo [OK] Build complete
)

if not exist "xiaohongshu-mcp.exe" (
    echo [ERROR] xiaohongshu-mcp.exe not found, building...
    go build -o xiaohongshu-mcp.exe .
    if errorlevel 1 (
        echo [ERROR] Build failed!
        pause
        exit /b 1
    )
    echo [OK] Build complete
)

echo.
echo [START] Manager: http://127.0.0.1:18050
echo [TIP] Open browser to manage users
echo [TIP] Press Ctrl+C to stop
echo.

start http://127.0.0.1:18050
manager.exe -listen=127.0.0.1:18050 -store=./data/manager/users.json

pause
