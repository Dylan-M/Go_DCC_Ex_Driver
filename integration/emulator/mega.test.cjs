const { test } = require('node:test');
const assert = require('node:assert/strict');
const { mkdtempSync, writeFileSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { join } = require('node:path');
const { readHex } = require('./mega.cjs');

test('Intel HEX validation prevents corrupt firmware from silently executing', t => {
  const dir = mkdtempSync(join(tmpdir(), 'dccex-hex-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  function load(text) { const file=join(dir,'firmware.hex');writeFileSync(file,text);return readHex(file); }
  const result=load(':0400000001020304F2\n:00000001FF\n');
  assert.equal(result.length,128*1024);
  assert.equal(result[0],0x0201);
  assert.throws(()=>load(':0400000001020304F3\n:00000001FF'),/checksum/);
  assert.throws(()=>load(':0400000001020304F2'),/EOF/);
  assert.throws(()=>load(':00000001FF\n:00000001FF'),/Invalid/);
  assert.throws(()=>load(':020000040004F6\n:0400000001020304F2\n:00000001FF'),/outside/);
  assert.throws(()=>load('not hex'),/Invalid/);
});
