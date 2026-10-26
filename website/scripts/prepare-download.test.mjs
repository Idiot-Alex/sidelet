import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, readFile, writeFile, rm, access } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
import { prepareDownload } from './prepare-download.mjs';

async function fixture(t) {
  const root = await mkdtemp(join(tmpdir(), 'sidelet-download-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const website = join(root, 'website');
  const target = join(website, 'public/downloads');
  await mkdir(join(root, 'packaging/macos'), { recursive: true });
  await mkdir(join(root, 'build/releases'), { recursive: true });
  await mkdir(target, { recursive: true });
  const info = { version: '0.2.0', build: '46', minimumSystemVersion: '13.0', bundleIdentifier: 'io.sidelet.mygo-lab' };
  await writeFile(join(root, 'packaging/macos/app.json'), JSON.stringify(info));
  const filename = 'Sidelet-0.2.0-build46-local-arm64.dmg';
  const source = join(root, 'build/releases', filename);
  const data = Buffer.from('synthetic installer bytes, never mounted or executed');
  const receipt = {
    schemaVersion: 1, platform: 'macOS', architecture: 'arm64', filename,
    version: info.version, build: info.build, minimumOS: '13.0', bundleIdentifier: info.bundleIdentifier,
    bytes: data.length, sha256: createHash('sha256').update(data).digest('hex'),
    binarySHA256: 'b'.repeat(64), sourceCommit: 'a'.repeat(40), sourceDirty: true,
  };
  await writeFile(source, data);
  await writeFile(`${source}.json`, JSON.stringify(receipt));
  await writeFile(join(target, 'release.json'), JSON.stringify({ available: true }));
  await writeFile(join(target, 'Sidelet-0.2.0-local-arm64.dmg'), 'old download');
  return { root, website, target, source, filename, data, receipt };
}

test('verified package publishes matching bytes and removes the old download', async t => {
  const f = await fixture(t);
  const manifest = await prepareDownload(f.root, f.website);
  assert.equal(manifest.available, true);
  assert.equal(manifest.build, '46');
  assert.equal(manifest.sha256, f.receipt.sha256);
  assert.deepEqual(await readFile(join(f.target, f.filename)), f.data);
  assert.equal(await readFile(join(f.target, `${f.filename}.sha256`), 'utf8'), `${f.receipt.sha256}  ${f.filename}\n`);
  await assert.rejects(access(join(f.target, 'Sidelet-0.2.0-local-arm64.dmg')), { code: 'ENOENT' });
});

test('only an old-build package present means no current download', async t => {
  const f = await fixture(t);
  await rm(f.source);
  await writeFile(join(f.root, 'build/releases/Sidelet-0.2.0-local-arm64.dmg'), f.data);
  const manifest = await prepareDownload(f.root, f.website);
  assert.equal(manifest.available, false);
  assert.equal(manifest.url, null);
  await assert.rejects(access(join(f.target, 'Sidelet-0.2.0-local-arm64.dmg')), { code: 'ENOENT' });
});

for (const [name, mutate] of [
  ['old version renamed to current filename', f => { f.receipt.version = '0.1.0'; }],
  ['old build renamed to current filename', f => { f.receipt.build = '45'; }],
  ['wrong architecture', f => { f.receipt.architecture = 'amd64'; }],
  ['wrong bundle', f => { f.receipt.bundleIdentifier = 'other.app'; }],
  ['wrong minimum OS', f => { f.receipt.minimumOS = '14.0'; }],
  ['same-size replacement bytes', async f => { await writeFile(f.source, Buffer.alloc(f.data.length, 1)); }],
  ['truncated installer', async f => { await writeFile(f.source, f.data.subarray(0, 5)); }],
]) {
  test(`${name} is rejected and stale availability is cleared`, async t => {
    const f = await fixture(t);
    await mutate(f);
    await writeFile(`${f.source}.json`, JSON.stringify(f.receipt));
    await assert.rejects(prepareDownload(f.root, f.website), /verification mismatch/);
    const manifest = JSON.parse(await readFile(join(f.target, 'release.json'), 'utf8'));
    assert.equal(manifest.available, false);
    await assert.rejects(access(join(f.target, f.filename)), { code: 'ENOENT' });
  });
}

for (const malformed of [false, true]) {
  test(`${malformed ? 'malformed' : 'missing'} receipt fails closed`, async t => {
    const f = await fixture(t);
    if (malformed) await writeFile(`${f.source}.json`, '{broken');
    else await rm(`${f.source}.json`);
    await assert.rejects(prepareDownload(f.root, f.website));
    assert.equal(JSON.parse(await readFile(join(f.target, 'release.json'), 'utf8')).available, false);
  });
}
