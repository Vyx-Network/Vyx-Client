@echo off
REM Build Vyx Client - Console Version (With Console Window for Debugging)

echo =========================================
echo Building Vyx Client - Console Version
echo =========================================
echo.

REM Build for Windows with console window
echo Building Windows console executable...
go build -ldflags="-s -w" -o vyx-client-console.exe .

if %ERRORLEVEL% EQU 0 (
    echo.
    echo =========================================
    echo Build Successful!
    echo =========================================
    echo.
    echo Output: vyx-client-console.exe
    echo - Console window visible
    echo - Logs to stdout
    echo - Useful for debugging
    echo.
    echo Run: vyx-client-console.exe --console
    echo.
) else (
    echo.
    echo =========================================
    echo Build Failed!
    echo =========================================
    echo.
    exit /b 1
)
