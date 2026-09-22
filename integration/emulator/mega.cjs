// Execute real Mega machine code; no DCC-EX response is fabricated here.
// Peripheral vectors are word addresses (2 * iomxx0_1.h vector numbers).
const fs = require('node:fs');
const assert = require('node:assert/strict');
const avr = require('avr8js');
const { createUSART } = require('./usart.cjs');

function readHex(file) {
  const flash = new Uint8Array(256 * 1024).fill(0xff);
  let base = 0, ended = false;
  for (const line of fs.readFileSync(file, 'utf8').trim().split(/\r?\n/)) {
    assert(!ended && /^:(?:[0-9a-f]{2})+$/i.test(line), 'Invalid Intel HEX record');
    const bytes = Buffer.from(line.slice(1), 'hex');
    assert.equal(bytes.length, bytes[0] + 5, 'HEX length');
    assert.equal(bytes.reduce((sum, n) => sum + n, 0) & 255, 0, 'HEX checksum');
    const address = bytes.readUInt16BE(1), kind = bytes[3], data = bytes.subarray(4, -1);
    if (kind === 0) {
      assert(base + address + data.length <= flash.length, 'HEX outside Mega flash');
      flash.set(data, base + address);
    } else if (kind === 1) {
      assert.equal(data.length, 0); ended = true;
    } else if (kind === 2 || kind === 4) {
      assert.equal(data.length, 2);
      base = data.readUInt16BE() * (kind === 2 ? 16 : 65536);
    } else if (kind !== 3 && kind !== 5) {
      throw new Error(`Unsupported HEX record ${kind}`);
    }
  }
  assert(ended, 'Missing HEX EOF');
  return new Uint16Array(flash.buffer);
}

function createMega(file, onByte) {
  const frequency = 16_000_000;
  // CPU adds 0x100 register bytes internally; Mega SRAM starts at 0x200.
  const cpu = new avr.CPU(readHex(file), 0x2100);
  for (const name of 'ABCDEFGHJKL') {
    const config = avr[`port${name}Config`];
    const port = new avr.AVRIOPort(cpu, { ...config, pinChange: undefined, externalInterrupts: [] });
    if (name === 'D') { port.setPin(0, true); port.setPin(1, true); } // I2C pull-ups.
  }
  new avr.AVRTimer(cpu, {
    ...avr.timer0Config, compAInterrupt: 42, compBInterrupt: 44, ovfInterrupt: 46,
    compPortA: 0x25, compPinA: 7, compPortB: 0x34, compPinB: 5,
    externalClockPort: 0x2b, externalClockPin: 7,
  });
  new avr.AVRTimer(cpu, {
    ...avr.timer1Config, captureInterrupt: 32, compAInterrupt: 34, compBInterrupt: 36,
    compCInterrupt: 38, ovfInterrupt: 40, OCRC: 0x8c, OCFC: 8, OCIEC: 8,
    compPortA: 0x25, compPinA: 5, compPortB: 0x25, compPinB: 6,
    compPortC: 0x25, compPinC: 7, externalClockPort: 0x2b, externalClockPin: 6,
  });
  new avr.AVRTimer(cpu, {
    ...avr.timer2Config, compAInterrupt: 26, compBInterrupt: 28, ovfInterrupt: 30,
    compPortA: 0x25, compPinA: 4, compPortB: 0x102, compPinB: 6,
  });
  const serial = createUSART(cpu, {
    ...avr.usart0Config, rxCompleteInterrupt: 50, dataRegisterEmptyInterrupt: 52,
    txCompleteInterrupt: 54,
  }, frequency);
  new avr.AVREEPROM(cpu, new avr.EEPROMMemoryBackend(4096), {
    ...avr.eepromConfig, eepromReadyInterrupt: 60,
  });
  new avr.AVRTWI(cpu, { ...avr.twiConfig, twiInterrupt: 78 }, frequency);
  const channels = {};
  for (let n = 0; n < 16; n++) {
    channels[n < 8 ? n : n + 24] = { type: avr.ADCMuxInputType.SingleEnded, channel: n };
  }
  channels[30] = { type: avr.ADCMuxInputType.Constant, voltage: 1.1 };
  channels[31] = { type: avr.ADCMuxInputType.Constant, voltage: 0 };
  const adc = new avr.AVRADC(cpu, {
    ...avr.adcConfig, adcInterrupt: 58, numChannels: 16, muxInputMask: 63,
    muxChannels: channels,
    adcReferences: [avr.ADCReference.AREF, avr.ADCReference.AVCC,
      avr.ADCReference.Internal1V1, avr.ADCReference.Internal2V56],
  });
  adc.channelValues.fill(0); // No track load or decoder ACK stimulus.

  serial.onByteTransmit = onByte;
  return {
    serial,
    get cycles() { return cpu.cycles; },
    runFor(seconds) {
      const until = cpu.cycles + seconds * frequency;
      while (cpu.cycles < until) {
        avr.avrInstruction(cpu);
        cpu.tick();
      }
    },
  };
}
module.exports = { createMega, readHex };
