// A byte-stream adapter, not a DCC-EX command interpreter. All replies originate
// from the unmodified firmware running on the emulated CPU.
const net = require('node:net');
const path = require('node:path');
const { createMega } = require('./mega.cjs');
const firmware = process.env.DCCEX_FIRMWARE || path.join(__dirname, '.work/build/CommandStation-EX.ino.hex');
let uartOutput = [], input = [], socket = null, ended = false, closed = false, settle = 0;
let shuttingDown = false, pumpTimer;
const log = (direction, text) => process.stderr.write(JSON.stringify({ direction, text }) + '\n');
const report = event => process.stdout.write(JSON.stringify(event) + '\n');
const mega = createMega(firmware, byte => uartOutput.push(byte));
mega.runFor(3);
const boot = Buffer.from(uartOutput).toString('ascii');
log('boot', boot);
if (!boot.includes('DCC-EX v5.6.1') || !boot.includes('"Ready"')) {
  throw new Error('Expected pinned firmware startup and Ready indication');
}
uartOutput = [];

const server = net.createServer({ allowHalfOpen: true }, connection => {
  if (socket || shuttingDown) { connection.destroy(); return; }
  socket = connection;
  ended = false;
  closed = false;
  input = [];
  connection.setNoDelay(true);
  connection.on('error', error => log('socket-error', error.message));
  connection.on('data', bytes => {
    if (input.length + bytes.length > 65536) {
      connection.destroy(new Error('UART input queue limit exceeded'));
      return;
    }
    log('tx', bytes.toString('ascii'));
    input.push(...bytes);
  });
  // Drain accepted bytes into the UART after TCP FIN, including shutdown <0>.
  connection.on('end', () => { ended = true; settle = mega.cycles + 1_600_000; });
  connection.on('close', () => {
    // A client that closes without reading pending replies can cause a TCP
    // reset. Still deliver bytes already accepted into our UART queue before
    // allowing another connection to share the firmware's serial parser.
    if (socket === connection) {
      closed = true;
      ended = true;
      settle = mega.cycles + 1_600_000;
    }
  });
});
function pump() {
  if (shuttingDown) return;
  if (input.length && mega.serial.rxEnable && !mega.serial.rxBusy) {
    if (!mega.serial.writeByte(input.shift())) throw new Error('UART rejected queued byte');
    if (ended) settle = mega.cycles + 1_600_000;
  }
  mega.runFor(0.002);
  if (uartOutput.length) {
    const bytes = Buffer.from(uartOutput);
    uartOutput = [];
    log('rx', bytes.toString('ascii'));
    if (socket && !socket.destroyed && !socket.writableEnded) {
      socket.write(bytes);
      if (socket.writableLength > 65536) socket.destroy(new Error('Slow consumer'));
    }
  }
  if (ended && input.length === 0 && !mega.serial.rxBusy && mega.cycles >= settle && socket) {
    if (closed) {
      socket = null;
      ended = false;
      report({ closed: true });
    } else if (!socket.writableEnded) socket.end();
  }
  pumpTimer = setTimeout(pump, 1);
}
function shutdown() {
  if (shuttingDown) return;
  shuttingDown = true;
  clearTimeout(pumpTimer);
  clearTimeout(lifetime);
  socket?.destroy();
  server.close();
  process.stdin.destroy();
}
// Hard bound even if the test process disappears or never connects.
const lifetime = setTimeout(() => { log('error', '120 second lifetime exceeded'); process.exitCode = 1; shutdown(); }, 120_000);
server.on('error', error => { log('error', error.message); process.exitCode = 1; shutdown(); });
server.listen(0, '127.0.0.1', () => {
  report({ port: server.address().port });
  pump();
});
// Dedicated process-control channel, not commands exposed over the TCP port.
process.stdin.setEncoding('utf8');
let control = '';
process.stdin.on('data', text => {
  control += text;
  if (control.length > 1024) { process.exitCode = 1; shutdown(); return; }
  let newline;
  while ((newline = control.indexOf('\n')) >= 0) {
    const command = control.slice(0, newline).trim();
    control = control.slice(newline + 1);
    if (command === 'quit') shutdown();
    else if (command === 'disconnect') socket?.destroy();
    else { log('error', 'Unknown process control command'); process.exitCode = 1; shutdown(); }
  }
});
process.stdin.on('end', shutdown);
process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);
