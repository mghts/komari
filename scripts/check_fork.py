"""Check active fork distribution endpoints; preserve module paths and attribution."""
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
PUBLIC_MARKETS = {
    'https://raw.githubusercontent.com/komari-monitor/theme-market/main/v1.json',
}
for directory in ['cmd', 'internal', 'web', 'utils', 'deploy', '.github/actions', '.github/workflows']:
    for path in (ROOT / directory).rglob('*'):
        if not path.is_file() or path.suffix not in {'.go', '.sh', '.yml', '.yaml', '.json'}:
            continue
        text = path.read_text()
        for allowed in PUBLIC_MARKETS:
            text = text.replace(allowed, '')
        assert not re.search(r'(?:https?://[^\s"\']*|ghcr\.io/)komari-monitor', text), path
installer = (ROOT / 'install-komari.sh').read_text()
assert 'REPO="mghts/komari"' in installer
assert '/releases/latest/' not in installer
frontend = json.loads((ROOT / 'build/frontend.json').read_text())
agent = json.loads((ROOT / 'build/agent.json').read_text())
assert frontend['repository'] == 'mghts/komari-web'
assert re.fullmatch(r'[0-9a-f]{40}', frontend['commit'])
assert re.fullmatch(r'ghcr\.io/mghts/komari-agent@sha256:[0-9a-f]{64}', agent['image'])
print('Server active fork sources and locked repositories passed.')
