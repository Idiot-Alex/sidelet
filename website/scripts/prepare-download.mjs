import { access, copyFile, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const websiteRoot = fileURLToPath(new URL('..', import.meta.url));
const projectRoot = resolve(websiteRoot, '..');
const info = JSON.parse(await readFile(resolve(projectRoot, 'packaging/macos/app.json'), 'utf8'));
const filename = `Sidelet-${info.version}-local-arm64.dmg`;
const source = resolve(projectRoot, 'build/releases', filename);
const target = resolve(websiteRoot, 'public/downloads');
await mkdir(target, { recursive: true });
let available = true;
try { await access(source); } catch { available = false; }
const manifest = {
  version: info.version,
  build: info.build,
  platform: 'macOS',
  architecture: 'arm64',
  minimumOS: info.minimumSystemVersion,
  available,
  filename,
  url: available ? `downloads/${filename}` : null,
  bytes: 0,
  sha256: null,
};
if (available) {
  const data = await readFile(source);
  manifest.bytes = data.byteLength;
  manifest.sha256 = createHash('sha256').update(data).digest('hex');
  await copyFile(source, resolve(target, filename));
  await writeFile(resolve(target, `${filename}.sha256`), `${manifest.sha256}  ${filename}\n`);
} else {
  await rm(resolve(target, filename), { force: true });
  await rm(resolve(target, `${filename}.sha256`), { force: true });
}
await writeFile(resolve(target, 'release.json'), `${JSON.stringify(manifest, null, 2)}\n`);
console.log(available ? `Prepared macOS download: ${filename}` : 'No local installer found. Website will show release progress instead of a download.');
