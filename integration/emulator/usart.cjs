const { AVRUSART } = require('avr8js');

// avr8js 0.21.1 clears RXC whenever UCSRB is written with RXEN already set.
// Arduino changes TX interrupt enable while sending diagnostics, so this can
// discard a pending received character. Real hardware keeps RXC until UDR is
// read (or the receiver is disabled). Preserve that flag across unrelated writes.
function createUSART(cpu, config, frequency) {
  const serial = new AVRUSART(cpu, config, frequency);
  const writeControl = cpu.writeHooks[config.UCSRB];
  cpu.writeHooks[config.UCSRB] = (value, oldValue) => {
    const preserveReceive = (value & oldValue & 0x10) && (cpu.data[config.UCSRA] & 0x80);
    const handled = writeControl(value, oldValue);
    if (preserveReceive) cpu.setInterruptFlag(serial.RXC);
    return handled;
  };
  return serial;
}

module.exports = { createUSART };
