"""Installer checks without root, package downloads or real systemd services."""
import hashlib
import json
import tarfile
from pathlib import Path
import shlex
import subprocess
import tempfile
import unittest

INSTALLER = Path(__file__).resolve().parents[1] / 'install.sh'


class InstallerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)
        for directory in ('work', 'state/data', 'bin', 'units', 'source/deploy/systemd'):
            (self.root / directory).mkdir(parents=True, exist_ok=True)
        self.script = self.root / 'install.sh'
        self.script.write_text(INSTALLER.read_text().replace(
            '/run/lock/conso-dashboard-ctl.lock', str(self.root / 'ctl.lock')))
        self.setup = f'''
source {shlex.quote(str(self.script))}
work_dir={shlex.quote(str(self.root / 'work'))}
STATE_DIR={shlex.quote(str(self.root / 'state'))}
BIN_DIR={shlex.quote(str(self.root / 'bin'))}
UNIT_DIR={shlex.quote(str(self.root / 'units'))}
SOURCE_DIR={shlex.quote(str(self.root / 'source'))}
# Preserve install modes while avoiding real ownership changes.
install() {{
    local args=()
    while (( $# )); do
        case $1 in -o|-g) shift 2 ;; *) args+=("$1"); shift ;; esac
    done
    command install "${{args[@]}}"
}}
id() {{ if [[ ${{1:-}} == -u ]]; then echo 1234; fi; }}
read_credentials() {{ token=secret-test-token; prm=12345678901234; }}
systemctl() {{
    printf '%s\\n' "$*" >> {shlex.quote(str(self.root / 'calls'))}
    if [[ ${{1:-}} == show ]]; then
        if [[ ${{2:-}} == --property=LoadState ]]; then echo loaded; else echo active; fi
    fi
}}
'''

    def run_script(self, body):
        return subprocess.run(['bash', '-c', self.setup + body], text=True,
                              capture_output=True, timeout=10)

    def calls(self):
        path = self.root / 'calls'
        return path.read_text() if path.exists() else ''

    def test_config_is_private_and_secret_is_not_printed(self):
        result = self.run_script('configure')
        self.assertEqual(result.returncode, 0, result.stderr)
        config = self.root / 'state/.env'
        self.assertIn('CONSO_API_TOKEN=secret-test-token', config.read_text())
        self.assertEqual(config.stat().st_mode & 0o777, 0o640)
        self.assertNotIn('secret-test-token', result.stdout + result.stderr)

    def test_existing_config_and_database_preserved(self):
        config = self.root / 'state/.env'
        config.write_text('existing configuration\n')
        data = self.root / 'state/data/conso.duckdb'
        data.write_bytes(b'existing database')
        result = self.run_script('read_credentials() { exit 99; }; configure')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(config.read_text(), 'existing configuration\n')
        self.assertEqual(data.read_bytes(), b'existing database')

    def test_invalid_prm_does_not_create_config(self):
        result = self.run_script('read_credentials() { token=secret; prm=invalid; }; configure')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / 'state/.env').exists())

    def test_failed_import_does_not_activate_services(self):
        ctl = self.root / 'bin/conso-dashboard-ctl'
        ctl.write_text('#!/bin/sh\nexit 1\n')
        ctl.chmod(0o755)
        result = self.run_script('start_installation')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('enable', self.calls())

    def test_successful_import_activates_both_services(self):
        ctl = self.root / 'bin/conso-dashboard-ctl'
        ctl.write_text('#!/bin/sh\nexit 0\n')
        ctl.chmod(0o755)
        result = self.run_script('start_installation')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('enable --now conso-dashboard.service', self.calls())
        self.assertIn('enable --now conso-dashboard-fetch.timer', self.calls())

    def test_cleanup_restores_running_services_on_failure(self):
        result = self.run_script('restore_server=1; restore_timer=1; trap cleanup EXIT; exit 7')
        self.assertEqual(result.returncode, 7)
        self.assertIn('start conso-dashboard.service', self.calls())
        self.assertIn('start conso-dashboard-fetch.timer', self.calls())
        self.assertFalse((self.root / 'work').exists())

    def test_install_files_waits_for_daily_import_and_keeps_data(self):
        payload = self.root / 'source/deploy/systemd'
        for name in ('conso-dashboard.service', 'conso-dashboard-fetch.service',
                     'conso-dashboard-fetch.timer', 'conso-dashboard-ctl'):
            (payload / name).write_text('payload\n')
        (self.root / 'work/conso-dashboard').write_text('binary\n')
        data = self.root / 'state/data/conso.duckdb'
        data.write_bytes(b'existing database')
        result = self.run_script('''
active() {
    if [[ $1 == "$daily" ]]; then
        daily_checks=$(( ${daily_checks:-0} + 1 ))
        (( daily_checks < 3 ))
    else return 0; fi
}
sleep() { :; }
install_files
[[ $restore_server == 1 && $restore_timer == 1 ]]
''')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Attente', result.stdout)
        self.assertIn('stop conso-dashboard-fetch.timer', self.calls())
        self.assertIn('stop conso-dashboard.service', self.calls())
        self.assertIn('daemon-reload', self.calls())
        self.assertTrue((self.root / 'bin/conso-dashboard-ctl').exists())
        self.assertEqual(data.read_bytes(), b'existing database')

    def test_prebuilt_binary_skips_compilation(self):
        binary = self.root / 'source/conso-dashboard'
        binary.write_text('#!/bin/sh\necho conso-dashboard-test\n')
        result = self.run_script('apt-get() { :; }; curl() { exit 99; }; prepare_binary')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('conso-dashboard-test', result.stdout)

    def prepare_download(self, corrupt=False):
        fixture = self.root / 'fixture'
        (fixture / 'go/bin').mkdir(parents=True)
        compiler = fixture / 'go/bin/go'
        compiler.write_text("""#!/bin/bash
while (( $# )); do
    if [[ $1 == -o ]]; then
        shift
        printf '#!/bin/sh\\necho compiled-test\\n' > "$1"
        chmod +x "$1"
        exit 0
    fi
    shift
done
exit 1
""")
        compiler.chmod(0o755)
        archive = fixture / 'go1.26.9.linux-amd64.tar.gz'
        with tarfile.open(archive, 'w:gz') as tar:
            tar.add(fixture / 'go', arcname='go')
        digest = hashlib.sha256(archive.read_bytes()).hexdigest()
        (fixture / 'versions.json').write_text(json.dumps([{
            'version': 'go1.26.9', 'stable': True, 'files': [{
                'os': 'linux', 'arch': 'amd64', 'kind': 'archive',
                'filename': archive.name, 'sha256': '0' * 64 if corrupt else digest
            }]
        }]))
        (self.root / 'source/go.mod').touch()
        return f'''apt-get() {{ :; }}
go_arch=amd64
curl() {{
    local url= output=
    while (( $# )); do
        case $1 in
            -o) output=$2; shift 2 ;;
            https:*) url=$1; shift ;;
            *) shift ;;
        esac
    done
    if [[ $url == *mode=json* ]]; then
        cp {shlex.quote(str(fixture / 'versions.json'))} "$output"
    else
        cp {shlex.quote(str(archive))} "$output"
    fi
}}
prepare_binary
'''

    def test_source_build_uses_verified_archive(self):
        result = self.run_script(self.prepare_download())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('compiled-test', result.stdout)

    def test_corrupt_go_archive_stops_before_extraction(self):
        result = self.run_script(self.prepare_download(corrupt=True))
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / 'work/go').exists())


if __name__ == '__main__':
    unittest.main()
