from saleae.analyzers import HighLevelAnalyzer, AnalyzerFrame

# PAN211x 7-bit I2C address
PAN211X_ADDR = 0x71

# Register address → name mapping
REGISTER_NAMES = {
    0x01: 'TRX_FIFO',
    0x02: 'STATE_CFG',
    0x07: 'WMODE_CFG0',
    0x08: 'WMODE_CFG1',
    0x09: 'RXPLLEN_CFG',
    0x0A: 'TXPLLEN_CFG',
    0x0F: 'PIPE0_RXADDR0',
    0x10: 'PIPE0_RXADDR1',
    0x11: 'PIPE0_RXADDR2',
    0x12: 'PIPE0_RXADDR3',
    0x14: 'TXADDR0',
    0x15: 'TXADDR1',
    0x16: 'TXADDR2',
    0x17: 'TXADDR3',
    0x1A: 'WHITEN_CFG',
    0x39: 'RF_CHANNEL',
    0x73: 'RFIRQFLG',
    0x77: 'STATUS3',
}

# STATE_CFG human-readable values
STATE_NAMES = {
    0x01: 'SLEEP',
    0x74: 'STB3',
    0x75: 'TX',
    0x76: 'RX',
}

# RFIRQFLG bit positions → name
RFIRQFLG_BITS = {
    7: 'TX_DONE',
    0: 'RX_DONE',
}

# WMODE_CFG0 field decoders (bit ranges)
WMODE_CFG0_CRC = {0b00: 'CRC_OFF', 0b01: 'CRC_8', 0b10: 'CRC_16', 0b11: 'CRC_24'}
WMODE_CFG0_WORK = {0b00: 'NORF', 0b01: 'SB', 0b10: 'ESB', 0b11: 'BLE'}

# WMODE_CFG1 address byte length (bits [1:0])
ADDR_BYTE_LEN = {0b00: '3B', 0b01: '4B', 0b10: '4B', 0b11: '5B'}


def _reg_name(reg):
    return REGISTER_NAMES.get(reg, f'REG_0x{reg:02X}')


def _decode_value(reg, data_bytes):
    """Return a human-readable string for a single register value or a byte buffer."""
    if len(data_bytes) == 0:
        return '(empty)'

    if len(data_bytes) > 1:
        # Multi-byte transfers (e.g. FIFO, addresses)
        return ' '.join(f'{b:02X}' for b in data_bytes)

    v = data_bytes[0]

    if reg == 0x02:  # STATE_CFG
        return STATE_NAMES.get(v, f'0x{v:02X}')

    elif reg == 0x07:  # WMODE_CFG0
        crc  = WMODE_CFG0_CRC.get((v >> 6) & 0x3, '?')
        work = WMODE_CFG0_WORK.get((v >> 4) & 0x3, '?')
        whiten   = 'WHITEN' if (v >> 3) & 1 else 'no_whiten'
        crc_skip = 'CRC_SKIP_ADDR' if (v >> 2) & 1 else ''
        noack    = 'TX_NOACK' if (v >> 1) & 1 else ''
        endian   = 'MSB_FIRST' if v & 1 else 'LSB_FIRST'
        parts = [crc, work, whiten] + [x for x in [crc_skip, noack, endian] if x]
        return f'0x{v:02X} ({" | ".join(parts)})'

    elif reg == 0x08:  # WMODE_CFG1
        rxgoon   = 'RX_GOON' if (v >> 7) & 1 else ''
        fifo128  = 'FIFO_128' if (v >> 5) & 1 else 'FIFO_64'
        enhance  = 'ENHANCE' if (v >> 4) & 1 else ''
        addr_len = ADDR_BYTE_LEN.get(v & 0x3, '?')
        parts = [x for x in [rxgoon, fifo128, enhance] if x] + [f'ADDR={addr_len}']
        return f'0x{v:02X} ({" | ".join(parts)})'

    elif reg == 0x39:  # RF_CHANNEL
        return f'0x{v:02X} ({2400 + v} MHz)'

    elif reg == 0x73:  # RFIRQFLG
        bits = [name for bit, name in sorted(RFIRQFLG_BITS.items(), reverse=True)
                if v & (1 << bit)]
        flag_str = '+'.join(bits) if bits else 'none'
        return f'0x{v:02X} ({flag_str})'

    elif reg == 0x1A:  # WHITEN_CFG
        skip_addr = bool(v & 0x80)
        seed = v & 0x7F
        return f'0x{v:02X} (skip_addr={int(skip_addr)}, seed=0x{seed:02X})'

    else:
        return f'0x{v:02X}'


# ---------------------------------------------------------------------------
# State machine states
# ---------------------------------------------------------------------------
# IDLE              – waiting for a START
# AWAIT_ADDR        – seen START, waiting for address frame (should be write to 0x71)
# AWAIT_REG_ACCESS  – seen address, waiting for first data byte (register access byte)
# WRITE_DATA        – accumulating write data bytes
# AWAIT_READ_RESTART– register access byte indicated read; waiting for repeated START
# AWAIT_READ_ADDR   – seen repeated START; waiting for address frame (read from 0x71)
# READ_DATA         – accumulating read data bytes

class Pan211xHla(HighLevelAnalyzer):

    result_types = {
        'write': {
            'format': 'W {{data.reg}}: [{{data.values}}]'
        },
        'read': {
            'format': 'R {{data.reg}}: [{{data.values}}]'
        },
        'error': {
            'format': 'PAN211x Error: {{data.msg}}'
        },
    }

    def __init__(self):
        self._reset()

    def _reset(self):
        self._state = 'IDLE'
        self._start_time = None
        self._end_time = None
        self._reg = None
        self._is_read = None
        self._data_bytes = []

    def _emit(self):
        if self._reg is None or self._start_time is None:
            return None
        frame_type = 'read' if self._is_read else 'write'
        values_str = _decode_value(self._reg, self._data_bytes)
        return AnalyzerFrame(frame_type, self._start_time, self._end_time, {
            'reg': _reg_name(self._reg),
            'values': values_str,
        })

    def _error(self, msg):
        f = None
        if self._start_time is not None and self._end_time is not None:
            f = AnalyzerFrame('error', self._start_time, self._end_time, {'msg': msg})
        self._reset()
        return f

    def decode(self, frame: AnalyzerFrame):

        # ---- START / repeated START ----------------------------------------
        if frame.type == 'start':
            if self._state == 'IDLE':
                self._start_time = frame.start_time
                self._end_time = frame.end_time
                self._state = 'AWAIT_ADDR'
            elif self._state == 'AWAIT_READ_RESTART':
                # Repeated START for the read phase – stay in I2C write state
                self._end_time = frame.end_time
                self._state = 'AWAIT_READ_ADDR'
            else:
                # Unexpected START: flush previous transaction (if any) and restart
                result = self._emit()
                self._reset()
                self._start_time = frame.start_time
                self._end_time = frame.end_time
                self._state = 'AWAIT_ADDR'
                return result
            return None

        # ---- ADDRESS frame -------------------------------------------------
        elif frame.type == 'address':
            addr = frame.data['address'][0]
            is_read = frame.data.get('read', False)
            self._end_time = frame.end_time

            if addr != PAN211X_ADDR:
                # Different device – ignore entirely
                self._reset()
                return None

            if self._state == 'AWAIT_ADDR':
                if not is_read:
                    self._state = 'AWAIT_REG_ACCESS'
                else:
                    return self._error('unexpected read address at start of transaction')

            elif self._state == 'AWAIT_READ_ADDR':
                if is_read:
                    self._state = 'READ_DATA'
                else:
                    return self._error('expected read address after repeated start')

            return None

        # ---- DATA frames ---------------------------------------------------
        elif frame.type == 'data':
            if 'data' not in frame.data:
                return None
            data_byte = frame.data['data'][0]
            self._end_time = frame.end_time

            if self._state == 'AWAIT_REG_ACCESS':
                # First data byte is the register access byte:
                #   bits[7:1] = 7-bit register address
                #   bit[0]    = 0 → write,  1 → read
                self._reg = data_byte >> 1
                self._is_read = bool(data_byte & 0x01)
                self._data_bytes = []
                if self._is_read:
                    self._state = 'AWAIT_READ_RESTART'
                else:
                    self._state = 'WRITE_DATA'

            elif self._state == 'WRITE_DATA':
                self._data_bytes.append(data_byte)

            elif self._state == 'READ_DATA':
                self._data_bytes.append(data_byte)

            return None

        # ---- STOP frame ----------------------------------------------------
        elif frame.type == 'stop':
            self._end_time = frame.end_time
            result = self._emit()
            self._reset()
            return result

        return None
