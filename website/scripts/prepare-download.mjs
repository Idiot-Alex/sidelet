import { mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { resolve } from 'node:path';

const digest = data => createHash('sha256').update(data).digest('hex');

export function validateReceipt(receipt, info, filename, data) {
  const expected = {
    schemaVersion: 1, platform: 'macOS', architecture: 'arm64', filename,
    version: info.version, build: info.build, minimumOS: info.minimumSystemVersion,
    bundleIdentifier: info.bundleIdentifier, bytes: data.byteLength, sha256: digest(data),
  };
  for (const [key, value] of Object.entries(expected)) {
    if (receipt?.[key] !== value) throw new Error(`Installer verification mismatch: ${key}. Re-run package-macos.sh.`);
  }
  if (data.byteLength === 0 || !/^[a-f0-9]{64}$/.test(receipt.binarySHA256)
    || !/^[a-f0-9]{40,64}$/.test(receipt.sourceCommit) || typeof receipt.sourceDirty !== 'boolean') {
    throw new Error('Installer verification record is incomplete. Re-run package-macos.sh.');
  }
}

export async function prepareDownload(projectRoot, websiteRoot) {
  const info = JSON.parse(await readFile(resolve(projectRoot, 'packaging/macos/app.json'), 'utf8'));
  if (!/^\d+\.\d+\.\d+$/.test(info.version) || !/^\d+$/.test(info.build)) throw new Error('Invalid release configuration');
  const filename = `Sidelet-${info.version}-build${info.build}-local-arm64.dmg`;
  const source = resolve(projectRoot, 'build/releases', filename);
  const target = resolve(websiteRoot, 'public/downloads');
  await mkdir(target, { recursive: true });
  const manifest = {
    version: info.version, build: info.build, platform: 'macOS', architecture: 'arm64',
    minimumOS: info.minimumSystemVersion, available: false, filename, url: null, bytes: 0, sha256: null,
  };
  // Fail closed before reading the artifact: an interrupted or invalid build
  // cannot leave yesterday's successful download advertised as today's.
  await writeFile(resolve(target, 'release.json'), `${JSON.stringify(manifest, null, 2)}\n`);
  for (const name of await readdir(target)) {
    if (/^Sidelet-[\w.-]+\.dmg(?:\.sha256|\.json)?$/.test(name)) await rm(resolve(target, name), { force: true });
  }
  let data;
  try {
    data = await readFile(source);
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
    return manifest;
  }
  // This receipt is created only after mounting and checking the actual DMG
  // on macOS. Its digest binds that check to these exact bytes on any build OS.
  const receipt = JSON.parse(await readFile(`${source}.json`, 'utf8'));
  validateReceipt(receipt, info, filename, data);
  await writeFile(resolve(target, filename), data);
  await writeFile(resolve(target, `${filename}.sha256`), `${receipt.sha256}  ${filename}\n`);
  Object.assign(manifest, {
    available: true, url: `downloads/${filename}`, bytes: receipt.bytes, sha256: receipt.sha256,
    binarySHA256: receipt.binarySHA256, sourceCommit: receipt.sourceCommit, sourceDirty: receipt.sourceDirty,
  });
  await writeFile(resolve(target, 'release.json'), `${JSON.stringify(manifest, null, 2)}\n`);
  return manifest;
}

if (process.argv[1] && pathToFileURL(resolve(process.argv[1])).href === import.meta.url) {
  const websiteRoot = fileURLToPath(new URL('..', import.meta.url));
  const manifest = await prepareDownload(resolve(websiteRoot, '..'), websiteRoot);
  console.log(manifest.available
    ? `Prepared verified macOS download: ${manifest.filename}`
    : 'No verified package for this build. Website will show release progress instead of a download.');
}
