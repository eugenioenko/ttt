#!/usr/bin/env bash
set -euo pipefail
python3 -m venv "$DAP_TOOLS/debugpy"
"$DAP_TOOLS/debugpy/bin/python" -m pip install --disable-pip-version-check debugpy==1.8.22
cat > "$DAP_TOOLS/bin/dap-python" <<'SH'
#!/usr/bin/env bash
exec "$DAP_TOOLS/debugpy/bin/python" "$@"
SH
chmod +x "$DAP_TOOLS/bin/dap-python"
