@echo off
cls
rsrc_windows_amd64 -arch amd64 -manifest i04Dns.exe.manifest -ico icon.ico -o ../src/i04Dns.exe.syso
cd ..