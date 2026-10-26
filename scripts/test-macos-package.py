import importlib.util
import json
from pathlib import Path
import plistlib
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('macos_package', Path(__file__).with_name('macos-package.py'))
package = importlib.util.module_from_spec(spec)
spec.loader.exec_module(package)


class PackageTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='sidelet-package-test-')
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.bundle = self.root / 'Sidelet.app'
        self.expected = dict(version='0.2.0', build='46', bundleIdentifier='io.sidelet.mygo-lab',
                             minimumSystemVersion='13.0', executable='sidelet')
        (self.bundle / 'Contents/MacOS').mkdir(parents=True)
        (self.bundle / 'Contents/Resources').mkdir()
        self.binary = self.bundle / 'Contents/MacOS/sidelet'
        self.binary.write_bytes(b'test binary, never executed')
        self.plist = self.bundle / 'Contents/Info.plist'
        self.info = dict(CFBundleShortVersionString='0.2.0', CFBundleVersion='46',
                         CFBundleIdentifier='io.sidelet.mygo-lab', LSMinimumSystemVersion='13.0', CFBundleExecutable='sidelet')
        self.plist.write_bytes(plistlib.dumps(self.info))
        self.provenance = dict(version='0.2.0', build='46', bundleIdentifier='io.sidelet.mygo-lab', minimumOS='13.0',
                               architecture='arm64', sourceCommit='a'*40, sourceDirty=True)
        (self.bundle / 'Contents/Resources/build-info.json').write_text(json.dumps(self.provenance))

    def test_old_embedded_build_rejected_even_if_outer_name_is_new(self):
        self.info['CFBundleVersion'] = '45'
        self.plist.write_bytes(plistlib.dumps(self.info))
        with self.assertRaisesRegex(ValueError, 'metadata'):
            package.verify_bundle(self.bundle, self.expected, 'arm64')

    def test_inconsistent_provenance_rejected(self):
        self.provenance['build'] = '45'
        (self.bundle / 'Contents/Resources/build-info.json').write_text(json.dumps(self.provenance))
        with self.assertRaisesRegex(ValueError, 'provenance mismatch'):
            package.verify_bundle(self.bundle, self.expected, 'arm64')

    def test_signed_matching_bundle_and_actual_architecture(self):
        with patch.object(package.subprocess, 'run') as run, patch.object(package.subprocess, 'check_output', return_value='arm64\n') as output:
            self.assertEqual(package.verify_bundle(self.bundle, self.expected, 'arm64'),
                             {**self.provenance, 'binarySHA256': package.sha256(self.binary)})
            run.assert_called_once_with(['codesign', '--verify', '--deep', '--strict', str(self.bundle)], check=True)
            output.return_value = 'x86_64\n'
            with self.assertRaisesRegex(ValueError, 'architectures'):
                package.verify_bundle(self.bundle, self.expected, 'arm64')

    def test_signature_failure_rejected(self):
        with patch.object(package.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'codesign')):
            with self.assertRaises(subprocess.CalledProcessError):
                package.verify_bundle(self.bundle, self.expected, 'arm64')

    def test_failed_image_check_detaches_and_emits_no_receipt(self):
        image = self.root / 'Sidelet-0.2.0-build46-local-arm64.dmg'
        image.write_bytes(b'fixture')
        with patch.object(package.subprocess, 'run') as run, patch.object(package, 'verify_bundle', side_effect=ValueError('old app')):
            with self.assertRaisesRegex(ValueError, 'old app'):
                package.verify_image(image, self.expected, 'arm64')
            self.assertEqual(run.call_args.args[0][:2], ['hdiutil', 'detach'])
        self.assertFalse(image.with_name(image.name + '.json').exists())


if __name__ == '__main__':
    unittest.main()
