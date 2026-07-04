@echo off
REM Run this .bat as your user to (re)register scheduled tasks.
schtasks /Create /F /TN "automation\tmp-test" /SC DAILY /ST 07:30 /TR "/usr/bin/python3" "/sessions/practical-bold-planck/mnt/automation-monorepo/tools/auto" run tmp-test
schtasks /Create /F /TN "automation\tmp-cron-test" /SC DAILY /ST 08:00 /TR "/usr/bin/python3" "/sessions/practical-bold-planck/mnt/automation-monorepo/tools/auto" run tmp-cron-test
