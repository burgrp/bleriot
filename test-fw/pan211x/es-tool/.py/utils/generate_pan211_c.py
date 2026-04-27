# Source Generated with Decompyle++
# File: generate_pan211_c.pyc (Python 3.8)

'''
PAN211 C代码生成器

该脚本用于根据配置文件生成PAN211芯片的C代码文件。主要功能包括：
1. 读取JSON配置文件
2. 验证配置参数的有效性和完整性
3. 生成包含寄存器配置的C代码
4. 生成功率设置函数和载波发射函数
5. 处理不同芯片模式和工作模式的条件参数

作者: PAN211开发团队
版本: 2.0
日期: 2025-07-04
'''
import os
import json
import re
import sys
import importlib.util as importlib
import subprocess
import shutil
import glob
import argparse
from pathlib import Path
from utils.modified_genconfig import *

class Pan211CGenerator:
    '''
    PAN211 C代码生成器类

    该类负责根据JSON配置文件生成PAN211芯片的C驱动代码。
    支持多种芯片模式（XN297、FS32、BLE）和工作模式（普通型、增强型）。
    '''
    RF_IT_MAP = {
        'RF_IT_TX_IRQ': 128,
        'RF_IT_MAX_RT_IRQ': 64,
        'RF_IT_ADDR_ERR_IRQ': 32,
        'RF_IT_CRC_ERR_IRQ': 16,
        'RF_IT_LEN_ERR_IRQ': 8,
        'RF_IT_PID_ERR_IRQ': 4,
        'RF_IT_RX_TIMEOUT_IRQ': 2,
        'RF_IT_RX_IRQ': 1,
        'RF_IT_ALL_IRQ': 255 }
    
    def __init__(self, template_dir = (None,)):
        '''
        初始化生成器

        Args:
            template_dir (str, optional): 模板文件目录路径。如果为None，使用默认路径。
        '''
        self.rf_config = { }
        self.config_type = 'RF'
        self.script_dir = os.path.dirname(os.path.abspath(__file__))
        if template_dir:
            self.template_dir = template_dir
        else:
            self.template_dir = os.path.join(self.script_dir, '../..', '01_SDK', 'radio')
        self.sdk_dir = os.path.join(self.script_dir, '..', '01_SDK')
        self.output_dir = None

    
    def load_config(self = None, config_file = None):
        '''
        从JSON文件加载配置

        Args:
            config_file (str): 配置文件的完整路径

        Raises:
            FileNotFoundError: 配置文件不存在
            json.JSONDecodeError: JSON格式错误
        '''
        pass
    # WARNING: Decompyle incomplete

    
    def print_all_parameters(self):
        '''
        打印所有配置参数用于调试

        输出格式化的配置参数列表，便于开发者查看和调试配置内容。
        '''
        print('\n=============== 配置参数列表 ===============')
        for key, value in self.rf_config.items():
            print(f'''  {key} = {value}''')
        print('=============================================\n')

    
    def find_example_configs(self):
        '''
        查找所有示例目录中的config.json文件

        遍历SDK示例目录，收集所有可用的配置文件路径。

        Returns:
            list: 包含所有找到的config.json文件路径的列表
        '''
        example_dir = os.path.join(self.sdk_dir, 'example')
        config_files = []
        for root, dirs, files in os.walk(example_dir):
            if 'config.json' in files:
                config_files.append(os.path.join(root, 'config.json'))
                continue
                return config_files

    
    def evaluate_condition(self = None, condition = None):
        '''
        评估预处理器条件表达式

        用于处理C代码模板中的条件编译指令，根据配置参数决定是否包含特定代码块。

        Args:
            condition (str): 条件表达式字符串

        Returns:
            bool: 条件评估结果
        '''
        defined_match = re.match('defined\\s*\\(\\s*(\\w+)\\s*\\)', condition)
        if defined_match:
            return defined_match.group(1) in self.rf_config
        if None in condition:
            
            try:
                (macro, value) = (lambda .0: [ x.strip() for x in .0 ])(condition.split('==', 1))
                if macro in self.rf_config:
                    pass
            finally:
                return None
            except ValueError:
                return False
            elif condition in self.rf_config:
                return bool(self.rf_config[condition])

        return False

    
    def process_macro_content(self = None, content = None):
        '''
        处理C代码模板中的宏内容

        根据配置参数处理C代码模板中的条件编译指令（#if、#ifdef、#ifndef等），
        移除不符合条件的代码块，保留符合条件的代码。

        Args:
            content (str): 包含预处理器指令的C代码内容

        Returns:
            str: 处理后的C代码内容
        '''
        lines = content.split('\n')
        processed_lines = []
        macro_stack = []
        current_block_keep = True
        i = 0
        if i < len(lines):
            line = lines[i]
            stripped_line = line.strip()
            if line.startswith('#include'):
                processed_lines.append(line)
                i += 1
                continue
            if stripped_line.startswith('#if') and stripped_line.startswith('#ifdef') or stripped_line.startswith('#ifndef'):
                if stripped_line.startswith('#if '):
                    condition = stripped_line[4:].strip()
                    keep_block = self.evaluate_condition(condition)
                elif stripped_line.startswith('#ifdef '):
                    macro = stripped_line[7:].strip()
                    keep_block = macro in self.rf_config
                elif stripped_line.startswith('#ifndef '):
                    macro = stripped_line[8:].strip()
                    keep_block = macro not in self.rf_config
                prev_block_keep = current_block_keep
                macro_stack.append((stripped_line, keep_block, prev_block_keep))
                if keep_block:
                    pass
                current_block_keep = prev_block_keep
                i += 1
                continue
            elif stripped_line.startswith('#elif '):
                if macro_stack:
                    (orig_condition, prev_keep, parent_block_keep) = macro_stack[-1]
                    if not parent_block_keep and prev_keep:
                        condition = stripped_line[6:].strip()
                        keep_block = self.evaluate_condition(condition)
                    else:
                        keep_block = False
                    if not prev_keep:
                        pass
                    macro_stack[-1] = (orig_condition, keep_block, parent_block_keep)
                    if keep_block:
                        pass
                    current_block_keep = parent_block_keep
                i += 1
                continue
            elif stripped_line == '#else':
                if macro_stack:
                    (orig_condition, prev_keep, parent_block_keep) = macro_stack[-1]
                    if not parent_block_keep and prev_keep:
                        keep_block = True
                    else:
                        keep_block = False
                    macro_stack[-1] = (orig_condition, prev_keep, parent_block_keep)
                    if keep_block:
                        pass
                    current_block_keep = parent_block_keep
                i += 1
                continue
            elif stripped_line == '#endif':
                if macro_stack:
                    (_, _, parent_block_keep) = macro_stack.pop()
                    if not macro_stack:
                        pass
                    elif macro_stack[-1][1]:
                        pass
                    current_block_keep = parent_block_keep
                i += 1
                continue
            elif stripped_line.startswith('#define'):
                if current_block_keep:
                    processed_lines.append(line)
                i += 1
                continue
            if current_block_keep:
                processed_lines.append(line)
            i += 1
            continue
        return '\n'.join(processed_lines)

    
    def apply_rf_config_to_genconfig(self):
        '''
        将RF配置应用到modified_genconfig模块

        这是核心配置处理函数，负责：
        1. 验证必需参数的完整性
        2. 根据芯片模式和工作模式检查条件参数
        3. 过滤不适用的参数
        4. 将配置应用到PAN211MPCONFIG对象

        Returns:
            tuple: (配置对象, 配置类型) 或 None（如果没有RF配置）

        Raises:
            ValueError: 当缺少必需参数时抛出异常
        '''
        if not self.rf_config:
            return None
        config = None()
        required_params = [
            'Channel',
            'TxPower',
            'DataRate',
            'ChipMode',
            'TxLen',
            'RxLen',
            'TxAddr',
            'RxAddr',
            'EnTxNoAck',
            'TxMode',
            'RxMode',
            'InterruptMask',
            'IOMUX_EN',
            'XTAL_FREQ',
            'RxGain',
            'EN_AGC']
        missing_required = []
        for param in required_params:
            if param not in self.rf_config:
                missing_required.append(param)
                continue
                if missing_required:
                    raise ValueError(f'''缺少必需的参数: {', '.join(missing_required)}''')
                None('\n=============== 应用RF配置参数 ===============')
                chip_mode = None
                work_mode = None
                tx_no_ack = None
                dpl_enabled = None
                rx_mode = None
                data_rate = None
                for key in ('ChipMode', 'WorkMode', 'EnTxNoAck', 'EnDPL', 'RxMode', 'DataRate'):
                    if key in self.rf_config or key == 'ChipMode':
                        chip_mode_value = self.rf_config[key]
                        if isinstance(chip_mode_value, str) and hasattr(PAN211Attributes, chip_mode_value):
                            chip_mode = getattr(PAN211Attributes, chip_mode_value)
                        else:
                            chip_mode = chip_mode_value
                        continue
                    if key == 'WorkMode':
                        work_mode_value = self.rf_config[key]
                        if isinstance(work_mode_value, str) and hasattr(PAN211Attributes, work_mode_value):
                            work_mode = getattr(PAN211Attributes, work_mode_value)
                        else:
                            work_mode = work_mode_value
                        continue
                    if key == 'EnTxNoAck':
                        tx_no_ack = self.rf_config[key]
                        continue
                    if key == 'EnDPL':
                        dpl_enabled = self.rf_config[key]
                        continue
                    if key == 'RxMode':
                        rx_mode_value = self.rf_config[key]
                        if isinstance(rx_mode_value, str) and hasattr(PAN211Attributes, rx_mode_value):
                            rx_mode = getattr(PAN211Attributes, rx_mode_value)
                        else:
                            rx_mode = rx_mode_value
                        continue
                    if key == 'DataRate':
                        data_rate_value = self.rf_config[key]
                        if isinstance(data_rate_value, str) and hasattr(PAN211Attributes, data_rate_value):
                            data_rate = getattr(PAN211Attributes, data_rate_value)
                            continue
                    data_rate = data_rate_value
                if chip_mode is not None:
                    if chip_mode == PAN211Attributes.PAN211_CHIPMODE_FS32:
                        fs32_required = [
                            'Endian',
                            'crcSkipAddr']
                        missing_fs32 = []
                        for param in fs32_required:
                            if param not in self.rf_config:
                                missing_fs32.append(param)
                                continue
                                if missing_fs32:
                                    raise ValueError(f'''芯片模式为FS32时，缺少必需参数: {', '.join(missing_fs32)}''')
                                if chip_mode == PAN211Attributes.PAN211_CHIPMODE_BLE:
                                    ble_required = [
                                        'BLEHeadNum',
                                        'BLEHead0',
                                        'BLEHead1',
                                        'S2S8Mode',
                                        'WhiteInit',
                                        'WhiteListMatchMode',
                                        'WhiteListOffset',
                                        'WhiteList',
                                        'WhiteListLen',
                                        'LengthFilterMode']
                                    missing_ble = []
                                    for param in ble_required:
                                        if param not in self.rf_config:
                                            missing_ble.append(param)
                                            continue
                                            if missing_ble:
                                                raise ValueError(f'''芯片模式为BLE时，缺少必需参数: {', '.join(missing_ble)}''')
                                            non_ble_required = [
                                                'EnDPL',
                                                'EnWhite',
                                                'Crc',
                                                'WorkMode',
                                                'AddrWidth']
                                            missing_non_ble = []
                                            for param in non_ble_required:
                                                if param not in self.rf_config:
                                                    missing_non_ble.append(param)
                                                    continue
                                                    if missing_non_ble:
                                                        raise ValueError(f'''芯片模式不是BLE时，缺少必需参数: {', '.join(missing_non_ble)}''')
                                                    if None == PAN211Attributes.PAN211_WORKMODE_ENHANCE and dpl_enabled:
                                                        enhance_dpl_required = [
                                                            'EnRxPlLenLimit']
                                                        missing_enhance_dpl = []
                                                        for param in enhance_dpl_required:
                                                            if param not in self.rf_config:
                                                                missing_enhance_dpl.append(param)
                                                                continue
                                                                if missing_enhance_dpl:
                                                                    raise ValueError(f'''工作模式为增强型且启用DPL时，缺少必需参数: {', '.join(missing_enhance_dpl)}''')
                                                                if None == PAN211Attributes.PAN211_WORKMODE_ENHANCE:
                                                                    enhance_required = [
                                                                        'EnManuPid']
                                                                    missing_enhance = []
                                                                    for param in enhance_required:
                                                                        if param not in self.rf_config:
                                                                            missing_enhance.append(param)
                                                                            continue
                                                                            if missing_enhance:
                                                                                raise ValueError(f'''工作模式为增强型时，缺少必需参数: {', '.join(missing_enhance)}''')
                                                                            if None == 0:
                                                                                tx_ack_required = [
                                                                                    'TRxDelayTimeUs',
                                                                                    'AutoDelayUs',
                                                                                    'AutoMaxCnt']
                                                                                missing_tx_ack = []
                                                                                for param in tx_ack_required:
                                                                                    if param not in self.rf_config:
                                                                                        missing_tx_ack.append(param)
                                                                                        continue
                                                                                        if missing_tx_ack:
                                                                                            raise ValueError(f'''EnTxNoAck为false时，缺少必需参数: {', '.join(missing_tx_ack)}''')
                                                                                        if None == 0 or rx_mode in (PAN211Attributes.PAN211_RX_MODE_SINGLE_WITH_TIMEOUT, PAN211Attributes.PAN211_ENHANCE_RX_MODE_CONTINOUS_WITH_TIMEOUT):
                                                                                            rx_timeout_required = [
                                                                                                'RxTimeoutUs']
                                                                                            missing_rx_timeout = []
                                                                                            for param in rx_timeout_required:
                                                                                                if param not in self.rf_config:
                                                                                                    missing_rx_timeout.append(param)
                                                                                                    continue
                                                                                                    if missing_rx_timeout:
                                                                                                        raise ValueError(f'''EnTxNoack为false或使用普通型单次超时模式或增强型连续超时模式时，缺少必需参数: {', '.join(missing_rx_timeout)}''')
                                                                                                    if None == PAN211Attributes.PAN211_DR_1Mbps and chip_mode == PAN211Attributes.PAN211_CHIPMODE_XN297:
                                                                                                        txdevselect_required = [
                                                                                                            'TxDevSelect']
                                                                                                        missing_txdevselect = []
                                                                                                        for param in txdevselect_required:
                                                                                                            if param not in self.rf_config:
                                                                                                                missing_txdevselect.append(param)
                                                                                                                continue
                                                                                                                if missing_txdevselect:
                                                                                                                    raise ValueError(f'''数据速率为1M且芯片模式为XN297时，缺少必需参数: {', '.join(missing_txdevselect)}''')
                                                                                                                keys_to_skip = None
                                                                                                                for key, value in self.rf_config.items():
                                                                                                                    should_apply = True
                                                                                                                    if chip_mode is not None and chip_mode != PAN211Attributes.PAN211_CHIPMODE_FS32 and key in ('Endian', 'crcSkipAddr'):
                                                                                                                        print(f'''  NOTICE: 跳过参数 \'{key}\'，因为芯片模式不是FS32''')
                                                                                                                        should_apply = False
                                                                                                                    if chip_mode is not None and chip_mode != PAN211Attributes.PAN211_CHIPMODE_BLE and key in ('BLEHeadNum', 'BLEHead0', 'BLEHead1', 'S2S8Mode', 'WhiteInit', 'WhiteListMatchMode', 'WhiteListOffset', 'WhiteList', 'WhiteListLen', 'LengthFilterMode'):
                                                                                                                        print(f'''  NOTICE: 跳过参数 \'{key}\'，因为芯片模式不是BLE''')
                                                                                                                        should_apply = False
                                                                                                                    if (work_mode != PAN211Attributes.PAN211_WORKMODE_ENHANCE or dpl_enabled) and key in ('EnRxPlLenLimit',):
                                                                                                                        print(f'''  NOTICE: 跳过参数 \'{key}\'，因为工作模式不是增强型或DPL未启用''')
                                                                                                                        should_apply = False
                                                                                                                    if work_mode != PAN211Attributes.PAN211_WORKMODE_ENHANCE and key in ('EnManuPid',):
                                                                                                                        print(f'''  NOTICE: 跳过参数 \'{key}\'，因为工作模式不是增强型''')
                                                                                                                        should_apply = False
                                                                                                                    if tx_no_ack == 1 and key in ('TRxDelayTimeUs', 'AutoDelayUs', 'AutoMaxCnt'):
                                                                                                                        print(f'''  NOTICE: 跳过参数 \'{key}\'，因为EnTxNoAck为true''')
                                                                                                                        should_apply = False
                                                                                                                    if should_apply == False:
                                                                                                                        print(f'''  NOTICE: 跳过参数 \'{key}\'，因为不适用于当前配置''')
                                                                                                                        keys_to_skip.append(key)
                                                                                                                        continue
                                                                                                                        for key in keys_to_skip:
                                                                                                                            del self.rf_config[key]
                                                                                                                        for key, value in self.rf_config.items():
                                                                                                                            if hasattr(config, key):
                                                                                                                                attr_name = key
                                                                                                                            elif hasattr(config, key.lower()):
                                                                                                                                attr_name = key.lower()
                                                                                                                            else:
                                                                                                                                print(f'''  WARNING: RF配置对象没有属性 \'{key}\' 或 \'{key.lower()}\'''')
                                                                                                                            if key == 'TxAddr' and isinstance(value, list):
                                                                                                                                hex_values = []
                                                                                                                                for addr in value:
                                                                                                                                    if isinstance(addr, str) and addr.startswith('0x'):
                                                                                                                                        hex_values.append(int(addr, 16))
                                                                                                                                    else:
                                                                                                                                        hex_values.append(addr)
                                                                                                                                original_value = value
                                                                                                                                value = hex_values
                                                                                                                                print(f'''  {key} -> {attr_name} = {original_value} (解析为十六进制: {value})''')
                                                                                                                            elif key == 'RxAddr' and isinstance(value, list):
                                                                                                                                for i, addr_obj in enumerate(value):
                                                                                                                                    new_addr_obj = [
                                                                                                                                        addr_obj[0],
                                                                                                                                        []]
                                                                                                                                    for addr in addr_obj[1]:
                                                                                                                                        if isinstance(addr, str) and addr.startswith('0x'):
                                                                                                                                            new_addr_obj[1].append(int(addr, 16))
                                                                                                                                        else:
                                                                                                                                            new_addr_obj[1].append(addr)
                                                                                                                                    value[i] = new_addr_obj
                                                                                                                                original_value = '复杂RxAddr结构'
                                                                                                                                print(f'''  {key} -> {attr_name} = {original_value} (地址解析为十六进制{value})''')
                                                                                                                            elif key == 'InterruptMask':
                                                                                                                                print(f'''  处理中断掩码 \'{value}\'''')
                                                                                                                                original_value = value
                                                                                                                                if isinstance(value, list):
                                                                                                                                    mask_value = 0
                                                                                                                                    for interrupt in value:
                                                                                                                                        if interrupt in self.RF_IT_MAP:
                                                                                                                                            mask_value |= self.RF_IT_MAP[interrupt]
                                                                                                                                        else:
                                                                                                                                            print(f'''  WARNING: 未知中断 \'{interrupt}\'，跳过''')
                                                                                                                                    print(f'''  {key} -> {attr_name} = {original_value} (解析为: {mask_value})''')
                                                                                                                                    value = mask_value
                                                                                                                                else:
                                                                                                                                    original_value = value
                                                                                                                                    if isinstance(value, str) and hasattr(PAN211Attributes, value):
                                                                                                                                        value = getattr(PAN211Attributes, value)
                                                                                                                                        print(f'''  {key} -> {attr_name} = {original_value} (解析为: {value})''')
                                                                                                                                    else:
                                                                                                                                        print(f'''  {key} -> {attr_name} = {value}''')
                                                                                                                            setattr(config, attr_name, value)
                                                                                                                        print('===============================================\n')
                                                                                                                        return (config, 'RF')

    
    def apply_config(self, generator):
        '''
        应用配置到寄存器生成器

        将RF配置应用到PAN211xGENCONFIG生成器，获取修改后的寄存器配置。

        Args:
            generator: PAN211xGENCONFIG实例，用于生成寄存器配置

        Returns:
            tuple: (page0寄存器表, page1寄存器表)
        '''
        sys.path.append(self.script_dir)
        config_result = self.apply_rf_config_to_genconfig()
        config = config_result[0]
        print('使用SetupConfig()应用RF配置')
        generator.SetupConfig(config)
        (page0_table, page1_table) = generator.getModifiedRegisters()
        print('\n=============== 寄存器配置摘要 ===============')
        print(f'''Page 0寄存器更改: {len(page0_table)} 个寄存器被修改''')
        print(f'''Page 1寄存器更改: {len(page1_table)} 个寄存器被修改''')
        print('==========================================\n')
        return (page0_table, page1_table)

    
    def generate_tx_power_function(self = None, generator = None):
        '''
        生成PAN211_SetTxPower函数，采用switch-case格式

        根据配置中的PowerTable生成发射功率设置函数，为每个功率等级
        生成相应的寄存器设置代码。

        Args:
            generator: PAN211xGENCONFIG实例，用于获取功率相关的寄存器值

        Returns:
            str: 生成的C函数代码

        Raises:
            ValueError: 如果配置中没有PowerTable字段
        '''
        sys.path.append(self.script_dir)
        builtin_power_table_level_maskedvalues = PAN211Attributes.sop8_power_table
        builtin_power_table_page_reg_mask = PAN211Attributes.sop8_power_table_reg
        power_levels_to_generate = []
        if self.rf_config and 'PowerTable' in self.rf_config:
            power_table_names = self.rf_config['PowerTable']
            print(f'''使用自定义功率表: {power_table_names}''')
            for name_in_power_table in power_table_names:
                if hasattr(PAN211Attributes, name_in_power_table):
                    power_level_value = getattr(PAN211Attributes, name_in_power_table)
                    power_levels_to_generate.append([
                        name_in_power_table,
                        power_level_value])
                    print(f'''添加功率级别 {name_in_power_table} = {power_level_value}''')
                    continue
                print(f'''警告: 功率级别名称 \'{name_in_power_table}\' 在PAN211PowerLevel中未找到''')
        else:
            raise ValueError("配置中未找到自定义功率表。请在配置文件中指定'PowerTable'字段。")
        function_code = None
        txpower_page0_reg_list = [
            67,
            68]
        txpower_page1_reg_list = [
            39,
            60,
            70,
            72]
        chip_mode = self.rf_config.get('ChipMode', 0)
        data_rate = self.rf_config.get('DataRate', 0)
        if isinstance(chip_mode, str) and hasattr(PAN211Attributes, chip_mode):
            chip_mode = getattr(PAN211Attributes, chip_mode)
        if isinstance(data_rate, str) and hasattr(PAN211Attributes, data_rate):
            data_rate = getattr(PAN211Attributes, data_rate)
        for item in power_levels_to_generate:
            power_level_name = item[0]
            power_level_to_generate = item[1]
            function_code += f'''    case {power_level_to_generate}: /* {power_level_name} */\n'''
            generator.PAN211_SetTxPower(chip_mode, data_rate, power_level_to_generate)
            (page0_table, page1_table) = generator.getRegistersbyAddress(txpower_page0_reg_list, txpower_page1_reg_list)
            function_code += '        PAN211_WriteReg(0x00, 0x01);\n'
            for reg in page1_table:
                function_code += f'''        PAN211_WriteReg(0x{reg[0]:02x}, 0x{reg[1]:02x});\n'''
            function_code += '        PAN211_WriteReg(0x00, 0x00);\n'
            for reg in page0_table:
                function_code += f'''        PAN211_WriteReg(0x{reg[0]:02x}, 0x{reg[1]:02x});\n'''
            function_code += '        break;\n\n'
        function_code += '    }\n}\n'
        return function_code

    
    def generate_carrier_wave_functions(self = None, generator = None):
        '''
        生成载波发射相关函数

        生成PAN211_StartCarrierWave和PAN211_ExitCarrierWave函数，
        用于进入和退出载波发射模式。

        Args:
            generator: PAN211xGENCONFIG实例，用于获取当前寄存器值

        Returns:
            str: 生成的载波发射函数C代码
        '''
        reg_values = { }
        for reg in generator.RecommendRegPage0:
            reg_values[reg[0]] = reg[1]
        reg_03_carrier = reg_values.get(3, 0)
        reg_06_carrier = reg_values.get(6, 0)
        reg_6A_carrier = reg_values.get(106, 0)
        reg_6B_carrier = reg_values.get(107, 0)
        carrier_wave_code = f'''/**\n * @brief 进入载波发射模式（由工具自动生成）\n * @note 进入载波模式前可设置频点和发射功率\n *          - PAN211_SetChannel(12)\n *          - PAN211_SetTxPower(PAN211_TXPWR_9dBm)\n */\nvoid PAN211_StartCarrierWave(void)\n{{\n    PAN211_WriteReg(0x02, 0x74);\n    PAN211_WriteReg(0x03, 0x{reg_03_carrier | 64:02x});\n    PAN211_WriteReg(0x06, 0x{reg_06_carrier | 128:02x});\n    PAN211_WriteReg(0x06, 0x{reg_06_carrier | 160:02x});\n    PAN211_WriteReg(0x06, 0x{reg_06_carrier | 224:02x});\n    PAN211_WriteReg(0x06, 0x{reg_06_carrier | 240:02x});\n    PAN211_WriteReg(0x6A, 0x{reg_6A_carrier | 2:02x});\n    PAN211_WriteReg(0x6A, 0x{reg_6A_carrier | 3:02x});\n    PAN211_WriteReg(0x6A, 0x{reg_6A_carrier | 35:02x});\n    PAN211_WriteReg(0x6A, 0x{reg_6A_carrier | 43:02x});\n    PAN211_WriteReg(0x6B, 0x{reg_6B_carrier | 128:02x});\n    PAN211_WriteReg(0x6B, 0x{reg_6B_carrier | 192:02x});\n    PAN211_WriteReg(0x03, 0x{reg_03_carrier | 192:02x});\n}}\n\n/**\n * @brief 退出载波发射模式（由工具自动生成）\n */\nvoid PAN211_ExitCarrierWave(void)\n{{\n    PAN211_WriteReg(0x6A, 0x00);\n    PAN211_WriteReg(0x6B, 0x00);\n    PAN211_WriteReg(0x06, 0x{reg_06_carrier:02x});\n    PAN211_WriteReg(0x03, 0x{reg_03_carrier | 64:02x});\n    PAN211_WriteReg(0x03, 0x{reg_03_carrier:02x});\n}}\n'''
        return carrier_wave_code

    
    def generate_easy_carrier_wave_functions(self = None, generator = None):
        '''
        生成载波发射相关函数

        生成PAN211_StartCarrierWave和PAN211_ExitCarrierWave函数，
        用于进入和退出载波发射模式。

        Args:
            generator: PAN211xGENCONFIG实例，用于获取当前寄存器值

        Returns:
            str: 生成的载波发射函数C代码
        '''
        deviation_table = {
            (0, 0): 300,
            (0, 1): 500,
            (0, 2): 170,
            (1, 0): 160,
            (1, 1): 320,
            (1, 2): 160,
            (2, 0): 170,
            (2, 1): 333,
            (2, 2): 170,
            (3, 0): 250,
            (3, 1): 500,
            (3, 2): 170 }
        chip_mode = self.rf_config.get('ChipMode', 0)
        data_rate = self.rf_config.get('DataRate', 0)
        tx_dev_select = self.rf_config.get('TxDevSelect', 0)
        if isinstance(chip_mode, str) and hasattr(PAN211Attributes, chip_mode):
            chip_mode = getattr(PAN211Attributes, chip_mode)
        if isinstance(data_rate, str) and hasattr(PAN211Attributes, data_rate):
            data_rate = getattr(PAN211Attributes, data_rate)
        if chip_mode == 0 and data_rate == 0:
            if isinstance(tx_dev_select, str):
                if '300K' in tx_dev_select:
                    deviation = 300
                elif '250K' in tx_dev_select:
                    deviation = 250
                
            tx_dev_select = int(tx_dev_select)
            if tx_dev_select == 0:
                deviation = 300
            elif tx_dev_select == 1:
                deviation = 250
            else:
                key = (chip_mode, data_rate)
                deviation = deviation_table.get(key, 300)
        correction = int(deviation * 1000 / 15)
        if correction > 32767:
            correction = 32767
        correction_tx_low = correction & 255
        correction_tx_high = correction >> 8 & 255
        print(f'''载波发射参数: ChipMode={chip_mode}, DataRate={data_rate}, TxDevSelect={tx_dev_select}''')
        print(f'''Deviation={deviation}, Correction={correction} (0x{correction:04X})''')
        print(f'''Correction Low=0x{correction_tx_low:02X}, High=0x{correction_tx_high:02X}''')
        reg_values = { }
        for reg in generator.RecommendRegPage0:
            reg_values[reg[0]] = reg[1]
        reg_2A_carrier = reg_values.get(42, 65)
        carrier_wave_code = f'''/**\n * @brief 进入载波发射模式（由工具自动生成）\n * @note 进入载波模式前可设置频点和发射功率\n *          - PAN211_SetChannel(12)\n *          - PAN211_SetTxPower(PAN211_TXPWR_9dBm)\n */\nvoid PAN211_StartCarrierWave(void)\n{{\n    PAN211_WriteReg(0x41, 0x{correction_tx_low:02x});\n    PAN211_WriteReg(0x42, 0x{correction_tx_high:02x});\n    PAN211_WriteReg(0x2A, 0x81); /* 设置为连续发送模式 */\n    PAN211_WriteReg(0x02, 0x75); /* 进入发送状态 */\n}}\n\n/**\n * @brief 退出载波发射模式（由工具自动生成）\n */\nvoid PAN211_ExitCarrierWave(void)\n{{\n    PAN211_WriteReg(0x41, 0x00);\n    PAN211_WriteReg(0x42, 0x00);\n    PAN211_WriteReg(0x02, 0x74);\n    PAN211_WriteReg(0x2A, 0x{reg_2A_carrier:02x});\n}}\n'''
        return carrier_wave_code

    
    def generate_sleep_wakeup_functions(self = None, generator = None):
        interface = self.rf_config.get('INTERFACE_MODE', 'SPI')
        if interface != 'USE_I2C':
            sleep_code = '/**\n * @brief 从待机状态进入睡眠状态\n */\nvoid PAN211_EnterSleep(void)\n{\n    PAN211_WriteReg(0x02, 0x74); /* 进入待机状态 */\n    PAN211_WriteReg(0x02, 0x21); /* 进入睡眠状态 */\n}\n\n/**\n * @brief 退出睡眠状态并进入待机状态\n */\nvoid PAN211_ExitSleep(void)\n{\n    PAN211_WriteReg(0x02, 0x22); /* 退出睡眠状态 */\n    PAN211_WriteReg(0x02, 0x74); /* 进入待机状态 */\n    DelayMs(1);                /* 等待晶振稳定 */\n}\n'
            return sleep_code
        sleep_code = None
        return sleep_code

    
    def format_config_table(self):
        '''
        将所有配置参数格式化为注释表格

        生成包含所有配置参数的格式化注释表格，用于插入到生成的C文件中。
        表格包含参数名称、值和描述信息。

        Returns:
            str: 格式化的注释表格字符串
        '''
        all_config = { }
        for key, value in self.rf_config.items():
            all_config[key] = value
        lines = [
            '/*-----------------------------------------------------------------------------------------------',
            ' *                                   Configuration Parameters',
            ' *-----------------------------------------------------------------------------------------------',
            ' *   Name                 | Value                                   | Description',
            ' * -----------------------|-----------------------------------------|----------------------------']
        param_name_description = [
            [
                'XTAL_FREQ',
                '晶振频率'],
            [
                'EN_AGC',
                '是否使能AGC'],
            [
                'INTERFACE_MODE',
                '接口模式'],
            [
                'ChipMode',
                '芯片工作模式'],
            [
                'WorkMode',
                '工作模式'],
            [
                'TxMode',
                '发送模式'],
            [
                'RxMode',
                '接收模式'],
            [
                'TxPower',
                '发射功率'],
            [
                'Channel',
                '工作频道'],
            [
                'DataRate',
                '数据传输速率'],
            [
                'EnWhite',
                '是否使能白化'],
            [
                'Endian',
                '大小端模式'],
            [
                'Crc',
                'CRC校验方式'],
            [
                'crcSkipAddr',
                'CRC是否跳过地址'],
            [
                'TxLen',
                '发送数据长度'],
            [
                'RxLen',
                '接收数据长度'],
            [
                'RxTimeoutUs',
                '接收超时时间(微秒)'],
            [
                'TRxDelayTimeUs',
                '发送接收延迟时间(微秒)，仅在增强型模式下有效'],
            [
                'AutoDelayUs',
                '自动重发延迟时间(微秒)，仅在增强型模式下有效'],
            [
                'AutoMaxCnt',
                '最大自动重发次数，仅在增强型模式下有效'],
            [
                'EnDPL',
                '是否使能动态有效负载长度，仅在增强型模式下有效'],
            [
                'EnManuPid',
                '是否使用手动包ID，仅在增强型模式下有效'],
            [
                'EnRxPlLenLimit',
                '是否限制接收有效负载长度，仅在增强型模式下有效'],
            [
                'EnTxNoAck',
                '发送是否不需要应答，仅在增强型模式下有效'],
            [
                'AddrWidth',
                '发送地址宽度'],
            [
                'TxAddr',
                '发送地址'],
            [
                'RxAddr',
                '各接收通道使能状态及地址'],
            [
                'IOMUX_EN',
                '复用中断引脚功能使能'],
            [
                'InterruptMask',
                '复用中断引脚关联的中断事件'],
            [
                'PowerTable',
                '可配置的功率档位列表'],
            [
                'RxGain',
                '接收增益'],
            [
                'TxDevSelect',
                '发射频偏选择'],
            [
                'BLEHead0',
                'BLE帧头标识符0'],
            [
                'BLEHead1',
                'BLE帧头标识符1'],
            [
                'BLEHeadNum',
                'BLE帧头字节数'],
            [
                'LengthFilterMode',
                'BLE长度过滤模式'],
            [
                'S2S8Mode',
                'S2S8工作模式'],
            [
                'WhiteInit',
                '白化初始化值'],
            [
                'WhiteList',
                '白名单地址'],
            [
                'WhiteListOffset',
                '白名单偏移'],
            [
                'WhiteListLen',
                '白名单长度'],
            [
                'BLEWhiteListMatchMode',
                '白名单匹配模式']]
        
        def add_line(name_text = None, value_text = None, description_text = None):
            '''添加格式化的表格行'''
            name_text = name_text.ljust(20)
            value_text = value_text.ljust(39)
            description_text = description_text.ljust(30)
            lines.append(f''' *   {name_text} | {value_text} | {description_text}''')

        
        def list_to_str(lst):
            '''将列表转换为格式化字符串'''
            return '[' + ', '.join((lambda .0: for a in .0:
passcontinuef'''0x{a:02x}'''[str(a)])(lst)) + ']'

        processed_params = set()
        for key, description in param_name_description:
            if key not in all_config:
                continue
            processed_params.add(key)
            value = all_config[key]
            if isinstance(value, list):
                if key == 'RxAddr':
                    add_line(key, '', description)
                    for i, item in enumerate(value):
                        enabled = 'True' if item[0] else 'False'
                        addr = item[1]
                        add_line('', enabled + ', ' + list_to_str(addr), f'''Pipe{i}接收通道使能状态及地址''')
                elif key == 'PowerTable' or key == 'InterruptMask':
                    add_line(key, '', description)
                    for item in value:
                        add_line('', item, '')
                else:
                    add_line(key, list_to_str(value), description)
            elif isinstance(value, bool):
                value_formatted = 'true' if value else 'false'
                add_line(key, value_formatted, description)
            elif key in ('BLEHead0', 'BLEHead1'):
                add_line(key, f'''0x{value:02x}''', description)
            else:
                value_formatted = str(value)
                add_line(key, value_formatted, description)
        lines.append(' *-----------------------------------------------------------------------------------------------*/')
        return '\n'.join(lines)

    
    def generate_c_file(self = None, output_file = None):
        """
        根据配置生成pan211.c文件

        这是核心的C代码生成函数，负责：
        1. 读取并处理C代码模板
        2. 移除不需要的内容块和包含文件
        3. 生成寄存器配置代码
        4. 插入配置参数表格
        5. 生成功率设置和载波发射函数
        6. 生成并拷贝头文件

        Args:
            output_file (str): 输出文件名，默认为'pan211.c'

        Returns:
            None

        Raises:
            FileNotFoundError: 模板文件不存在
            IOError: 文件读写错误
        """
        template_file = os.path.join(self.template_dir, 'pan211.c')
    # WARNING: Decompyle incomplete

    
    def _generate_header_file(self = None, c_file_path = None):
        '''
        生成对应的头文件

        根据生成的C文件路径，处理并生成相应的头文件。
        主要处理：
        1. 修改函数签名（移除参数）
        2. 清理不必要的包含文件和宏定义
        3. 拷贝到输出目录

        Args:
            c_file_path (str): 生成的C文件的完整路径
        '''
        h_file_path = os.path.join(self.template_dir, 'pan211.h')
    # WARNING: Decompyle incomplete



def process_example(generator, config_file, output_file = ('pan211.c',)):
    '''Process a single example configuration file'''
    if not generator:
        generator = Pan211CGenerator()
    generator.load_config(config_file)
    return generator.generate_c_file(output_file)


def gen_pan211_c(win, gpus, config_file, dist_folder_path, output_file = ('pan211.c',)):
    generator = Pan211CGenerator()
    generator.win = win
    generator.gpus = gpus
    generator.script_dir = Path(__file__).resolve().parent
    generator.template_dir = Path(dist_folder_path)
    generator.sdk_dir = dist_folder_path
    return process_example(generator, config_file, output_file, **('output_file',))

if __name__ == '__main__':
    pass
