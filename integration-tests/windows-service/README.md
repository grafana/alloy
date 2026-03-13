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
