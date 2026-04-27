# Source Generated with Decompyle++
# File: PanBus.pyc (Python 3.8)

from utils.TextUtils import replace_first
INTERRUPT_LIST = [
    'RF_IT_TX_IRQ',
    'RF_IT_MAX_RT_IRQ',
    'RF_IT_ADDR_ERR_IRQ',
    'RF_IT_CRC_ERR_IRQ',
    'RF_IT_LEN_ERR_IRQ',
    'RF_IT_PID_ERR_IRQ',
    'RF_IT_RX_TIMEOUT_IRQ',
    'RF_IT_RX_IRQ']

def parse_chip_mode(chip_mode):
    if isinstance(chip_mode, str):
        chip_mode = int(chip_mode)
    if chip_mode == 0:
        return 'XN297L'
    if None == 1:
        return 'FS01'
    if None == 2:
        return 'FS32'
    if None == 3:
        return 'Bluetooth-LE Beacon'


def parse_work_mode(work_mode):
    if isinstance(work_mode, str):
        work_mode = int(work_mode)
    if work_mode == 0:
        return 'Normal'
    if None == 1:
        return 'Enhance'


def parse_endian(endian):
    if isinstance(endian, str):
        endian = int(endian)
    if endian == 0:
        return 'Little'
    if None == 1:
        return 'Big'


def parse_trx_mode(trx_mode):
    if isinstance(trx_mode, str):
        trx_mode = int(trx_mode)
    if trx_mode == 0:
        return 'RX'
    if None == 1:
        return 'TX'


def parse_datarate(datarate):
    if isinstance(datarate, str):
        datarate = int(datarate)
    if datarate == 0:
        return '1Mbps'
    if None == 1:
        return '2Mbps'
    if None == 2:
        return '250Kbps'


def parse_crc(crc):
    if isinstance(crc, str):
        crc = int(crc)
    if crc == 0:
        return 'OFF'
    if None == 1:
        return '1byte'
    return None.format(crc)
    return ''


def parse_s2s8(s2s8):
    if isinstance(s2s8, str):
        s2s8 = int(s2s8)
    if s2s8 == 0:
        return 'S0'
    if None == 1:
        return 'S2'
    return None


def parse_on_off(on):
    if isinstance(on, str):
        on = int(on)
    if on == 0:
        return 'OFF'


def parse_spi_clk(clk):
    if isinstance(clk, str):
        clk = int(clk)
    clk_val = '{}MHz'
    if clk == 0:
        return clk_val.format('1')
    if None == 1:
        return clk_val.format('2')
    if None == 2:
        return clk_val.format('4')
    if None == 3:
        return clk_val.format('8')


def get_interface_defined(interface_index):
    interface_arr = [
        'USE_SPI_3LINE',
        'USE_I2C',
        'USE_SPI_4LINE']
    return interface_arr[interface_index]


def get_xtal_freq_defined(xtal):
    if xtal == 32:
        return 'XTAL_FREQ_32M'


def get_tx_mode_defined(mode):
    if mode:
        return 'PAN211_TX_MODE_CONTINOUS'


def get_rx_mode_defined(mode, work_mode, is_longrange = (False,)):
    if is_longrange:
        mode_list = [
            'PAN211_RX_MODE_SINGLE_WITH_TIMEOUT',
            'PAN211_RX_MODE_CONTINOUS']
    elif work_mode == 0:
        mode_list = [
            'PAN211_RX_MODE_SINGLE',
            'PAN211_RX_MODE_SINGLE_WITH_TIMEOUT',
            'PAN211_RX_MODE_CONTINOUS']
    else:
        mode_list = [
            'PAN211_ENHANCE_RX_MODE_CONTINOUS',
            'PAN211_ENHANCE_RX_MODE_CONTINOUS_WITH_TIMEOUT']
    return mode_list[mode % len(mode_list)]


def get_endian_defined(endian):
    endian_list = [
        'PAN211_ENDIAN_LITTLE',
        'PAN211_ENDIAN_BIG']
    return endian_list[endian]


def get_ble_len_filter_mode_defined(mode):
    mode_list = [
        'PAN211_BLE_LEN_FILTER_DISABLE',
        'PAN211_BLE_LEN_FILTER_EQUAL',
        'PAN211_BLE_LEN_FILTER_EXCEED',
        'PAN211_BLE_LEN_FILTER_BENEATH']
    return mode_list[mode]


def get_ble_white_list_match_mode_defined(mode):
    white_list = [
        'PAN211_BLE_WhiteList_DISABLE',
        'PAN211_BLE_WhiteList_1Byte',
        'PAN211_BLE_WhiteList_2Byte',
        'PAN211_BLE_WhiteList_3Byte',
        'PAN211_BLE_WhiteList_4Byte',
        'PAN211_BLE_WhiteList_5Byte',
        'PAN211_BLE_WhiteList_6Byte']
    return white_list[mode]


def get_ble_white_init_defined(ch_index):
    val_list = [
        'PAN211_BLE_WH_INIPHA_CH37',
        'PAN211_BLE_WH_INIPHA_CH38',
        'PAN211_BLE_WH_INIPHA_CH39']
    return val_list[ch_index]


def get_s2s8_mode_defined(mode):
    if mode == 0:
        return 'PAN211_PRIMODE_DIS'
    if None == 1:
        return 'PAN211_PRIMODE_S2'
    if None == 2:
        return 'PAN211_PRIMODE_S8'


def get_interrupt_arr(interrupt_val):
    interrupt_arr = []
    for i in range(8):
        mask = 1 << i
        if interrupt_val & mask > 0:
            interrupt_arr.append(INTERRUPT_LIST[i])
            continue
            return interrupt_arr


def get_addr_width_defined(addr_width):
    if addr_width:
        return 'PAN211_WIDTH_{}BYTES'.format(addr_width)


def get_work_mode_defined(work_mode):
    if work_mode:
        return 'PAN211_WORKMODE_ENHANCE'


def get_tx_mode_defined(tx_mode):
    mode_list = [
        'PAN211_TX_MODE_SINGLE',
        'PAN211_TX_MODE_CONTINOUS']
    return mode_list[tx_mode]


def get_tx_power_defined(curr_tx_power):
    if curr_tx_power == 99:
        return 'PAN211_TXPWR_0dBm_LOWPWR'
    if None == 9.5:
        return 'PAN211_TXPWR_9_5dBm'
    if None < 0:
        return 'PAN211_TXPWR_n{}dBm'.format(abs(curr_tx_power))
    return None.format(abs(curr_tx_power))


def get_datarate_defined(dr):
    if dr == 0:
        return 'PAN211_DR_1Mbps'
    if None == 1:
        return 'PAN211_DR_2Mbps'
    if None == 2:
        return 'PAN211_DR_250Kbps'


def get_chip_mode_defined(chip_mode):
    chip_mode_defined_list = [
        'PAN211_CHIPMODE_XN297',
        'PAN211_CHIPMODE_FS01',
        'PAN211_CHIPMODE_FS32',
        'PAN211_CHIPMODE_BLE']
    return chip_mode_defined_list[chip_mode]


def get_crc_defined(crc):
    crc_list = [
        'PAN211_CRC_off',
        'PAN211_CRC_1byte',
        'PAN211_CRC_2byte',
        'PAN211_CRC_3byte']
    return crc_list[crc]


def align_defined_content(content):
    rows = content.split('\n')
    act_content = ''
    for row in rows:
        defined_name = row.split('     ')[0]
        new_row = row + '\n'
        if defined_name.startswith('#define EASY_RF'):
            defined_name_len = len(defined_name)
            need_space = 40 - defined_name_len
            new_row = replace_first(row, '     ', ' ' * need_space) + '\r'
            print(':{}'.format(new_row))
        act_content += new_row
    return act_content

