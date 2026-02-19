# Windows service integration test

This test runs the Alloy Windows installer, verifies the Alloy service starts, then uninstalls. It does **not** use Docker or Windows containers.

**Requirements**

- Windows host (e.g. GitHub Actions `windows-latest` or a local Windows machine)
- Administrator privileges (installer and service need them)
- Built installer: set `ALLOY_INSTALLER_PATH` to the path of `alloy-installer-windows-amd64.exe`, or run via the Makefile target which builds it first

**Run**

From repo root on Windows:

```bash
make integration-test-windows-service
```

Or with an existing installer:

```powershell
cd integration-tests\windows-service
$env:ALLOY_INSTALLER_PATH = "..\..\dist\alloy-installer-windows-amd64.exe"
go test -v -timeout 5m -run TestWindowsService ./...
```

**Current behavior (scaffolding)**

1. Run installer silently with `/S` into a temp directory (`/D=...`).
2. Installer creates and starts the Alloy service.
3. Test verifies the service is present via `sc query Alloy`.
4. Uninstaller runs in defer (silent `/S`).

**TODO**

- Check metrics (e.g. HTTP GET to Alloy’s metrics endpoint).
- Check logs (e.g. Windows Event Log or configured log output).
- Make sure the uninstaller cleans up Alloy properly.
