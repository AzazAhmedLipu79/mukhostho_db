@echo off
REM Mukhostho DB Windows Batch Script

REM Set the title of the command window
title Mukhostho DB

REM Check if the executable exists
if not exist "build\mukhostho.exe" (
    echo Error: mukhostho.exe not found in build directory
    echo Please run: go build -o build\mukhostho.exe cmd\mukhostho\main.go
    pause
    exit /b 1
)

REM Create data directory if it doesn't exist
if not exist "data" mkdir data

REM Run the application
echo Starting Mukhostho DB...
echo ================================
echo Server will be available at: http://localhost:8080
echo Health check: http://localhost:8080/health
echo API documentation: See README.md
echo ================================
echo.

build\mukhostho.exe

REM Keep the window open if there's an error
if errorlevel 1 (
    echo.
    echo Application exited with error code %errorlevel%
    pause
)
