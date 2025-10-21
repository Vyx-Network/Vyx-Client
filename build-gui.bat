@echo off
REM Build Vyx Client - GUI Version (No Console Window)

echo =========================================
echo Building Vyx Client - GUI Version
echo =========================================
echo.

REM Build for Windows with GUI flag (hides console window)
echo Building Windows GUI executable...
go build -ldflags="-H windowsgui -s -w" -o vyx-client-gui.exe .

if %ERRORLEVEL% EQU 0 (
    echo.
    echo =========================================
    echo Build Successful!
    echo =========================================
    echo.
    echo Output: vyx-client-gui.exe
    echo - No console window
    echo - Logs to: %%APPDATA%%\Vyx\logs\
    echo - System tray only
    echo.
    echo Run: vyx-client-gui.exe
    echo.
) else (
    echo.
    echo =========================================
    echo Build Failed!
    echo =========================================
    echo.
    exit /b 1
)
