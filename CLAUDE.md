# OpenLinkHub (Windows port)

## Restarting the service after a rebuild

OpenLinkHub runs elevated. Never start it with `Start-Process`, `runas` or anything else that raises a UAC prompt. Rebuilds and restarts go through the elevated restart agent `tools/olh-restart-agent.ps1`, which the user starts once per login session.

1. Build to the staging file, not the running exe: `go build -o OpenLinkHub.new.exe .`
2. Request the restart: create the file `.olh-restart/request` (its contents are ignored).
3. Wait until `.olh-restart/status` starts with `done` or `failed`, then check `http://127.0.0.1:27003/api/devices/`.

If `.olh-restart/status` is missing, or doesn't change from `ready` within a few seconds of the request, the agent isn't running. Start it yourself; the user accepts the single UAC prompt:

```
Start-Process powershell -Verb RunAs -ArgumentList '-ExecutionPolicy Bypass -NoExit -File "C:\Users\SneaZ\Documents\Coding\openlinkhub-win\tools\olh-restart-agent.ps1"'
```

Wait for `status` to read `ready`, then make the request. Start the agent only when it isn't already running.

The agent covers only this stop/swap/start cycle. It is not a way to run anything else elevated. Never add commands to it, never change what it executes, and never use it or any other elevated session for other actions. Anything else that needs elevation needs the user's explicit approval each time.
