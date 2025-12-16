@echo off
chcp 65001 >nul
title xiaohongshu-mcp Single User

echo ========================================
echo   xiaohongshu-mcp Single User Mode
echo ========================================
echo.

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
echo [START] Service: http://127.0.0.1:18060
echo [TIP] MCP: http://127.0.0.1:18060/mcp
echo [TIP] API: http://127.0.0.1:18060/api/v1
echo [TIP] Press Ctrl+C to stop
echo.

xiaohongshu-mcp.exe -headless=true -port=:18060

pause
