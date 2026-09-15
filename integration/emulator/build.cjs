// Cross-platform, isolated firmware build. Requires Arduino CLI 1.5.1 on PATH
// or an explicit ARDUINO_CLI executable path. No upload/flash operation is used.
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const work = path.join(__dirname, '.work');
const source = path.join(work, 'CommandStation-EX');
const commit = '822a54977263b621541a70a3546d38f387ac7294';
const cli = process.env.ARDUINO_CLI || 'arduino-cli';
function run(exe, args, capture = false) {
  const result = spawnSync(exe, args, { cwd: __dirname, encoding: 'utf8',
    stdio: capture ? 'pipe' : 'inherit', timeout: 300_000 });
  if (result.error || result.status !== 0) throw new Error(`${exe} failed: ${result.error || result.stderr || result.status}`);
  return result.stdout?.trim();
}
fs.mkdirSync(work, { recursive: true });
const version = run(cli, ['version'], true);
if (!/Version:\s*1\.5\.1\b/.test(version)) throw new Error(`Expected Arduino CLI 1.5.1; got ${version}`);
if (!fs.existsSync(source)) run('git', ['clone', '--depth', '1', '--branch', 'v5.6.1-Prod',
  'https://github.com/DCC-EX/CommandStation-EX.git', source]);
if (run('git', ['-C', source, 'rev-parse', 'HEAD'], true) !== commit) throw new Error('Firmware commit mismatch');
if (run('git', ['-C', source, 'status', '--porcelain', '--untracked-files=no'], true)) throw new Error('Tracked firmware files were modified');
// No other ignored user configuration may silently change the firmware build.
const allowedConfig = new Set(['config.h']);
const unknown = run('git', ['-C', source, 'ls-files', '--others'], true).split(/\r?\n/).filter(Boolean);
if (unknown.some(file => !allowedConfig.has(file))) throw new Error(`Unexpected source additions: ${unknown}`);
fs.copyFileSync(path.join(__dirname, 'config.h'), path.join(source, 'config.h'));
const config = path.join(work, 'arduino-cli.yaml');
const quote = value => JSON.stringify(value.replaceAll('\\', '/'));
fs.writeFileSync(config, `directories:\n  data: ${quote(path.join(work, 'data'))}\n  downloads: ${quote(path.join(work, 'downloads'))}\n  user: ${quote(path.join(work, 'sketchbook'))}\n`);
run(cli, ['core', 'update-index', '--config-file', config]);
run(cli, ['core', 'install', 'arduino:avr@1.8.6', '--config-file', config]);
run(cli, ['compile', '--fqbn', 'arduino:avr:mega:cpu=atmega2560', '--config-file', config,
  '--output-dir', path.join(work, 'build'), source]);
const hash = file => createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const provenance = { commit, tag: 'v5.6.1-Prod', cli: version, core: 'arduino:avr@1.8.6',
  fqbn: 'arduino:avr:mega:cpu=atmega2560', configSHA256: hash(path.join(source, 'config.h')),
  hexSHA256: hash(path.join(work, 'build/CommandStation-EX.ino.hex')) };
fs.writeFileSync(path.join(work, 'build/provenance.json'), JSON.stringify(provenance, null, 2) + '\n');
console.log(JSON.stringify(provenance));
