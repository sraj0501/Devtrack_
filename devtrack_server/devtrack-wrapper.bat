@echo off
REM DevTrack Docker Wrapper for Windows (Batch)
REM Simple batch wrapper that calls PowerShell script

SET SCRIPT_DIR=%~dp0
PowerShell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%devtrack-wrapper.ps1" %*
