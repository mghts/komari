"""Exercise the real shell installer with retained, isolated files and fake services."""
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

INSTALLER = Path(__file__).resolve().parents[1] / 'install-komari.sh'

class InstallerTests(unittest.TestCase):
    def exercise(self, mode):
        root = Path(tempfile.mkdtemp(prefix='komari-server-installer-'))
        (root / 'komari').write_text('old binary')
        checksum = hashlib.sha256(b'new binary').hexdigest()
        # All external interactions are shell functions: no real networking or services.
        harness = r'''
source "$INSTALLER"
INSTALL_DIR="$TEST_ROOT"
BINARY_PATH="$TEST_ROOT/komari"
INSTALL_VERSION=1.4.4-rc.2
select_version() { return 0; }
check_systemd() { return 0; }
install_dependencies() { return 0; }
detect_arch() { echo amd64; }
ui_msgbox() { :; }
curl() {
    echo "$2" >> "$TEST_ROOT/urls"
    case "$2" in
        */SHA256SUMS)
            if [[ "$TEST_MODE" = missing ]]; then echo 'invalid' > "$4";
            else echo "$TEST_CHECKSUM  komari-linux-amd64" > "$4"; fi ;;
        */komari-linux-amd64)
            if [[ "$TEST_MODE" = unavailable ]]; then return 22; fi
            if [[ "$TEST_MODE" = corrupt ]]; then printf corrupted > "$4";
            else printf 'new binary' > "$4"; fi ;;
        *) return 99 ;;
    esac
}
systemctl() {
    echo "$1" >> "$TEST_ROOT/services"
    if [[ "$TEST_MODE" = start_failure && "$1" = start && "$(cat "$BINARY_PATH")" = 'new binary' ]]; then return 1; fi
}
upgrade_komari
'''
        env = dict(os.environ, INSTALLER=str(INSTALLER), TEST_ROOT=str(root), TEST_MODE=mode,
                   TEST_CHECKSUM=checksum, TMPDIR=str(root))
        result = subprocess.run(['bash'], input=harness, text=True, capture_output=True, env=env)
        self.assertEqual(result.returncode == 0, mode == 'success', result.stdout + result.stderr)
        expected = 'new binary' if mode == 'success' else 'old binary'
        self.assertEqual((root / 'komari').read_text(), expected)
        urls = (root / 'urls').read_text().splitlines()
        self.assertTrue(all(url.startswith('https://github.com/mghts/komari/releases/download/1.4.4-rc.2/') for url in urls))
        if mode in {'corrupt', 'missing', 'unavailable'}:
            self.assertFalse((root / 'services').exists(), 'Download failure must not stop the service')
        else:
            backups = list(root.glob('komari.backup.*'))
            self.assertEqual(len(backups), 1)
            self.assertEqual(backups[0].read_text(), 'old binary')
        if mode == 'start_failure':
            self.assertEqual((root / 'services').read_text().splitlines(), ['stop', 'start', 'stop', 'start'])

    def test_success(self): self.exercise('success')
    def test_corrupt_download_preserves_running_service(self): self.exercise('corrupt')
    def test_missing_checksum_preserves_running_service(self): self.exercise('missing')
    def test_unavailable_release_preserves_running_service(self): self.exercise('unavailable')
    def test_failed_start_restores_binary(self): self.exercise('start_failure')

if __name__ == '__main__': unittest.main()
