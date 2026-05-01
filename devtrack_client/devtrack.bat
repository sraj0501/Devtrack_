@echo off
rem Compatibility wrapper: forwards 'devtrack' commands to devtrack-cli.
rem Install alongside devtrack-cli.exe in the same directory and add to PATH.
"%~dp0devtrack-cli.exe" %*
