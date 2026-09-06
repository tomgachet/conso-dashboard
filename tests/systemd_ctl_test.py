"""Exercise the administration command without systemd, root or API calls."""
import os
import fcntl
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "deploy/systemd/conso-dashboard-ctl"

class ImportCommandTests(unittest.TestCase):
    def run_case(self, running, failed=False, daily=False, args=(), locked=False, interrupted=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            script = root / "ctl"
            script.write_text(SCRIPT.read_text().replace(
                "(( EUID != 0 ))", "(( 0 != 0 ))"
            ).replace("/run/lock/conso-dashboard-ctl.lock", str(root / "lock")))
            mock = root / "mock"
            mock.write_text(r"""#!/usr/bin/python3
import os, sys
from pathlib import Path
root = Path(os.environ['MOCK_ROOT'])
args = sys.argv[1:]
with (root/'calls').open('a') as f:
    f.write(Path(sys.argv[0]).name + ' ' + repr(args) + '\n')
if Path(sys.argv[0]).name == 'systemd-run':
    if os.environ['INTERRUPTED'] == '1':
        import signal
        os.kill(os.getppid(), signal.SIGTERM)
    sys.exit(int(os.environ['IMPORT_FAILURE']))
if args[0] == 'show':
    if '--property=LoadState' in args:
        print('loaded')
    elif args[-1] == 'conso-dashboard-fetch.service' and (root/'daily').exists():
        (root/'daily').unlink()
        print('activating')
    else:
        print('active' if os.environ['RUNNING'] == '1' and args[-1] in
              ('conso-dashboard.service', 'conso-dashboard-fetch.timer') else 'inactive')
""")
            mock.chmod(0o755)
            for command in ('systemctl', 'systemd-run'):
                (root/command).symlink_to(mock)
            if daily:
                (root/'daily').touch()
            env = dict(os.environ, PATH=str(root)+':'+os.environ['PATH'],
                       MOCK_ROOT=str(root), RUNNING=str(int(running)),
                       IMPORT_FAILURE=str(int(failed)), INTERRUPTED=str(int(interrupted)))
            lock = (root / "lock").open("w")
            if locked:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            result = subprocess.run(['bash', str(script), 'fetch', *args],
                                    env=env, capture_output=True, text=True)
            lock.close()
            return result, (root/'calls').read_text() if (root/'calls').exists() else ''

    def test_running_services_restored_after_success(self):
        result, calls = self.run_case(True, args=('-start', '2026-01-01', '-end', '2026-09-01'))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("'fetch', '-start', '2026-01-01', '-end', '2026-09-01'", calls)
        self.assertLess(calls.index("['stop', 'conso-dashboard-fetch.timer']"), calls.index('systemd-run'))
        self.assertLess(calls.index("['stop', 'conso-dashboard.service']"), calls.index('systemd-run'))
        self.assertIn("['start', 'conso-dashboard.service']", calls)
        self.assertIn("['start', 'conso-dashboard-fetch.timer']", calls)

    def test_failure_is_reported_and_services_restored(self):
        result, calls = self.run_case(True, failed=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("['start', 'conso-dashboard.service']", calls)
        self.assertIn("['start', 'conso-dashboard-fetch.timer']", calls)

    def test_initial_import_keeps_services_stopped(self):
        result, calls = self.run_case(False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("['start',", calls)

    def test_daily_import_implies_server_restart(self):
        result, calls = self.run_case(False, daily=True, args=('yesterday',))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("['start', 'conso-dashboard.service']", calls)
        self.assertNotIn("['start', 'conso-dashboard-fetch.timer']", calls)

    def test_second_manual_import_refused(self):
        result, calls = self.run_case(True, locked=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('déjà en cours', result.stderr)
        self.assertEqual(calls, '')

    def test_interruption_restores_services(self):
        result, calls = self.run_case(True, interrupted=True)
        self.assertEqual(result.returncode, 143, result.stderr)
        self.assertIn("['start', 'conso-dashboard.service']", calls)
        self.assertIn("['start', 'conso-dashboard-fetch.timer']", calls)

if __name__ == '__main__':
    unittest.main()
