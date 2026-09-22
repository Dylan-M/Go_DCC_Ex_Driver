const { test } = require('node:test');
const assert = require('node:assert/strict');
const { mkdtempSync, writeFileSync, rmSync } = require('node:fs');
const { tmpdir } = require('node:os');
const { join } = require('node:path');
const { readHex } = require('./mega.cjs');
const { CPU, usart0Config } = require('avr8js');
const { createUSART } = require('./usart.cjs');

test('transmit interrupt changes preserve an unread received command byte', () => {
  for (const txInterrupt of [0x20, 0x40]) {
    const cpu = new CPU(new Uint16Array(128));
    const serial = createUSART(cpu, usart0Config, 16_000_000);
    cpu.writeData(usart0Config.UCSRB, 0x98); // RX/TX on, RX interrupt enabled.
    assert.equal(cpu.data[usart0Config.UCSRA] & 0x80, 0);
    serial.writeByte('1'.charCodeAt(0), true);
    assert.equal(cpu.data[usart0Config.UCSRA] & 0x80, 0x80);
    cpu.writeData(usart0Config.UCSRB, 0x98 | txInterrupt);
    assert.equal(cpu.data[usart0Config.UCSRA] & 0x80, 0x80);
    assert.equal(cpu.nextInterrupt, usart0Config.rxCompleteInterrupt);
    cpu.writeData(usart0Config.UCSRB, 0x98);
    assert.equal(cpu.data[usart0Config.UCSRA] & 0x80, 0x80);
    assert.equal(cpu.readData(usart0Config.UDR), '1'.charCodeAt(0));
    assert.equal(cpu.data[usart0Config.UCSRA] & 0x80, 0);
    cpu.writeData(usart0Config.UCSRB, 0x98 | txInterrupt);
    assert.equal(cpu.data[usart0Config.UCSRA] & 0x80, 0); // Never fabricate input.
  }
});

test('clocked receive survives interrupt masking and reaches the Mega RX vector', () => {
  const config = { ...usart0Config, rxCompleteInterrupt: 50,
    dataRegisterEmptyInterrupt: 52, txCompleteInterrupt: 54 };
  const cpu = new CPU(new Uint16Array(128));
  const serial = createUSART(cpu, config, 16_000_000);
  cpu.writeData(config.UCSRB, 0x18); // RX/TX on; receive interrupt masked.
  assert.equal(serial.writeByte(0x31), true);
  assert.equal(serial.rxBusy, true);
  cpu.cycles += serial.cyclesPerChar;
  cpu.tick();
  assert.equal(serial.rxBusy, false);
  assert.equal(cpu.data[config.UCSRA] & 0x80, 0x80);
  assert.equal(cpu.nextInterrupt, -1);
  cpu.writeData(config.UCSRB, 0x98); // Unmask the pending receive interrupt.
  assert.equal(cpu.nextInterrupt, 50);
  cpu.writeData(config.UCSRB, 0x18); // Masking does not consume the byte.
  assert.equal(cpu.nextInterrupt, -1);
  assert.equal(cpu.data[config.UCSRA] & 0x80, 0x80);
  cpu.writeData(config.UCSRB, 0x98);
  cpu.data[0x5f] |= 0x80; // Global interrupts enabled.
  cpu.tick();
  assert.equal(cpu.pc, 50);
  assert.equal(cpu.readData(config.UDR), 0x31);
  assert.equal(cpu.data[config.UCSRA] & 0x80, 0);
  assert.equal(cpu.nextInterrupt, -1);
});

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
