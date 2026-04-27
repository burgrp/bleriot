# Source Generated with Decompyle++
# File: convert_pan211_to_pms.pyc (Python 3.8)

from __future__ import annotations
import re
import sys
import os
from pathlib import Path
CONFIG_BLOCK_END_RE = re.compile('\\*/')
FUNC_INIT_SIG_RE = re.compile('\\bunsigned\\s+char\\s+PAN211_Init\\s*\\(\\s*void\\s*\\)')
RE_WRITEREG = re.compile('PAN211_WriteReg\\s*\\(\\s*(0x[0-9a-fA-F]+)\\s*,\\s*(0x[0-9a-fA-F]+)\\s*\\)\\s*;')
RE_DELAY_MS = re.compile('延时\\s*(\\d+)\\s*ms')
RE_DELAY_US = re.compile('延时\\s*(\\d+)\\s*us*')
RE_READREG = re.compile('PAN211_ReadReg\\s*\\(')
RE_WHILE_MS = re.compile('while\\s*\\([^)]*\\)\\s*;?\\s*/\\*[^*]*?(\\d+)\\s*ms', re.IGNORECASE)
STEP_COMMENT_RE = re.compile('^\\s*/\\*\\s*\\d+.')
MS_TOKEN_MAP = {
    1: '0xF0',
    2: '0xF1',
    55: '0xF2' }
US_TOKEN = '0xF0'
TIME_MULTIPLIER = 1
TIME_MULTIPLIER_MAP = {
    '2M': 1,
    '4M': 2,
    '8M': 4 }

def extract_function_body(text = None, sig_re = None):
    '''
    提取指定函数的函数体。

    参数:
        text: 源文件内容
        sig_re: 用于匹配函数签名的正则表达式

    返回:
        元组 (左花括号位置, 右花括号位置, 函数体内容)

    异常:
        如果未找到函数或函数体不完整，则抛出RuntimeError
    '''
    m = sig_re.search(text)
    if not m:
        raise RuntimeError('PAN211_Init signature not found')
    brace_start = None.find('{', m.end())
    if brace_start < 0:
        raise RuntimeError('Opening brace for PAN211_Init not found')
    depth = None
    i = brace_start
    if i < len(text):
        ch = text[i]
        if ch == '{':
            depth += 1
        elif ch == '}':
            depth -= 1
            if depth == 0:
                return (brace_start, i, text[brace_start + 1:i])
            None += 1
            continue
            raise RuntimeError('Failed to match closing brace for PAN211_Init')
            return None


def extract_remaining_functions(text = None, init_end_pos = None):
    '''
    提取PAN211_Init函数之后的所有函数，并进行类型替换和结构转换。

    参数:
        text: 源文件内容
        init_end_pos: PAN211_Init函数结束位置

    返回:
        处理后的PAN211_Init函数之后的所有函数的文本
    '''
    functions_text = text[init_end_pos:].strip()
    function_pattern = re.compile('(/\\*\\*.*?\\*/\\s*)?void\\s+(\\w+)\\s*\\([^)]*\\)\\s*{|(/\\*\\*.*?\\*/\\s*)?unsigned\\s+char\\s+(\\w+)\\s*\\([^)]*\\)\\s*{|(/\\*\\*.*?\\*/\\s*)?byte\\s+(\\w+)\\s*\\([^)]*\\)\\s*{', re.DOTALL)
    matches = list(function_pattern.finditer(functions_text))
    functions_to_remove = [
        'PAN211_SetRxAddr',
        'PAN211_GetRxPipeNum',
        'PAN211_SetAckPipeNum',
        'PAN211_SetTxAddr',
        'PAN211_SetAddrWidth',
        'PAN211_GetRecvLen',
        'PAN211_ExitTxMode',
        'PAN211_ExitRxMode',
        'PAN211_SetWhiteInitVal']
    processed_functions = []
    for i in range(len(matches)):
        start = matches[i].start()
        end = len(functions_text) if i == len(matches) - 1 else matches[i + 1].start()
        func_block = functions_text[start:end]
        if not matches[i].group(2) and matches[i].group(4):
            pass
        func_name = matches[i].group(6)
        print(f'''[DEBUG] Found function: {func_name}, void_type={matches[i].group(2) is not None}, char_type={matches[i].group(4) is not None}, byte_type={matches[i].group(6) is not None}''')
        if func_name in functions_to_remove:
            print(f'''[INFO] Removing function: {func_name}''')
            continue
        func_block = replace_types(func_block)
        if 'switch' in func_block:
            func_block = replace_switch_case(func_block)
        func_block = process_delay_functions(func_block)
        processed_functions.append(func_block)
    return ''.join(processed_functions)


def replace_types(text = None):
    '''
    将unsigned char, int8_t等类型替换为byte，并将所有函数返回类型改为void。

    参数:
        text: 原始代码文本

    返回:
        替换类型后的代码文本
    '''
    text = re.sub('\\bunsigned\\s+char\\b', 'byte', text)
    text = re.sub('\\bint8_t\\b', 'byte', text)
    text = re.sub('\\buint8_t\\b', 'byte', text)
    text = re.sub('\\bunsigned\\s+short\\b', 'int', text)
    text = re.sub('\\buint16_t\\b', 'int', text)
    text = re.sub('\\bint16_t\\b', 'int', text)
    text = re.sub('(\\s*(?:/\\*\\*.*?\\*/\\s*)?)\\bbyte\\s+(\\w+\\s*\\([^)]*\\)\\s*{)', '\\1void \\2', text, re.DOTALL, **('flags',))
    return text


def process_delay_functions(text = None):
    '''
    处理代码中的延时函数和return语句，确保它们正确转换。

    参数:
        text: 原始代码文本

    返回:
        处理延时函数和return语句后的代码文本
    '''
    text = re.sub('DelayMs\\((\\d+)\\);', (lambda m: f'''.delay {int(m.group(1)) * 2000 * TIME_MULTIPLIER};'''), text)
    text = re.sub('DelayUs\\((\\d+)\\);', (lambda m: f'''.delay {int(m.group(1)) * TIME_MULTIPLIER};'''), text)
    text = re.sub('return\\s+(PAN211_\\w+\\([^)]*\\));', '\\1;', text)
    return text


def replace_switch_case(text = None):
    '''
    将switch-case结构替换为if-else结构。

    参数:
        text: 原始代码文本

    返回:
        替换结构后的代码文本
    '''
    switch_pattern = re.compile('switch\\s*\\(\\s*(.*?)\\s*\\)\\s*{(.*?)}', re.DOTALL)
    
    def replace_switch(match):
        switch_var = match.group(1).strip()
        switch_body = match.group(2)
        case_pattern = re.compile('case\\s+(.*?):(.*?)(?=\\s*case|\\s*default:|\\s*}|$)', re.DOTALL)
        cases = case_pattern.findall(switch_body)
        default_pattern = re.compile('default:(.*?)(?=\\s*}|$)', re.DOTALL)
        default_match = default_pattern.search(switch_body)
        default_body = default_match.group(1).strip() if default_match else None
        if_else_structure = []
        for case_value, case_body in enumerate(cases):
            case_body = case_body.strip()
            case_body = re.sub('\\s*break\\s*;', '', case_body).strip()
            comment_pattern = re.compile('/\\*\\s*(.*?)\\s*\\*/', re.DOTALL)
            comment_match = comment_pattern.search(case_body)
            comment = f''' /* {comment_match.group(1)} */''' if comment_match else ''
            if_else_structure.append('    {')
            if comment:
                if_else_structure.append(f'''       {comment}''')
            for line in case_body.splitlines():
                if '/*' in line and '*/' in line and comment and line.strip().startswith('/*'):
                    continue
                if line.strip():
                    if_else_structure.append(f'''        {line.strip()}''')
                    continue
                    if_else_structure.append('    }')
                    continue
                    if default_body:
                        default_body = re.sub('\\s*break\\s*;', '', default_body).strip()
                        if_else_structure.append('    else')
                        if_else_structure.append('    {')
                        for line in default_body.splitlines():
                            if line.strip():
                                if_else_structure.append(f'''        {line.strip()}''')
                                continue
                                if_else_structure.append('    }')
                                return '\n'.join(if_else_structure)

    function_pattern = re.compile('(void|byte|unsigned\\s+char)\\s+(\\w+)\\s*\\([^)]*\\)\\s*{(.*?)}', re.DOTALL)
    
    def process_function(match = None):
        func_type = match.group(1)
        func_name = match.group(2)
        func_body = match.group(3)
        if 'switch' in func_body:
            for _ in range(3):
                switch_match = switch_pattern.search(func_body)
                if not switch_match:
                    pass
                else:
                    switch_var = switch_match.group(1).strip()
                    switch_body = switch_match.group(2)
                    replaced_switch = replace_switch(switch_match)
                    func_body = func_body.replace(switch_match.group(0), replaced_switch)
                return f'''{func_type} {func_name}({func_body})'''

    result = text
    for _ in range(5):
        switch_match = switch_pattern.search(result)
        if not switch_match:
            pass
        else:
            new_result = switch_pattern.sub(replace_switch, result)
            if new_result == result:
                pass
            else:
                result = new_result
    return result


def classify_dynamic_lines(lines = None):
    """
    检测连续的工厂校准代码块（包含ReadReg但不包含while循环）。
    如果存在，则在下一个步骤注释（如'/* 4.'）处结束该块。

    参数:
        lines: 代码行列表

    返回:
        元组 (动态区块开始行索引, 动态区块结束行索引)，如果未找到则返回 (None, None)
    """
    start = None
    for i, l in enumerate(lines):
        if 'PAN211_ReadReg' in l and 'while' not in l:
            start = i
        
        if start is None:
            return (None, None)
        end = None
        for j in range(start + 1, len(lines)):
            if STEP_COMMENT_RE.search(lines[j]):
                pass
            else:
                end = j
            return (start, end)


def delay_to_token(line = None):
    '''
    将延时函数调用转换为对应的延时标记。

    参数:
        line: 包含延时函数调用的代码行

    返回:
        延时标记字符串，如果不是延时函数调用则返回None
    '''
    print('Line for delay_to_token:', line)
    m = RE_DELAY_MS.search(line)
# WARNING: Decompyle incomplete


def while_to_token(line = None):
    '''
    将while循环转换为对应的延时标记。

    参数:
        line: 包含while循环的代码行

    返回:
        延时标记字符串
    '''
    m = RE_WHILE_MS.search(line)
    if m:
        
        try:
            ms = int(m.group(1))
        finally:
            pass
        ms = 1
        return MS_TOKEN_MAP.get(ms, '0xF0')
        return '0xF0'



def strip_comments_keep_lines(s = None):
    '''
    移除代码中的注释同时保持行数结构不变。

    参数:
        s: 包含注释的代码文本

    返回:
        移除注释后的代码文本，保持原有行结构
    '''
    result = []
    lines = s.splitlines()
    for line in lines:
        cleaned_line = re.sub('/\\*.*?\\*/', '', line)
        cleaned_line = re.sub('//.*', '', cleaned_line)
        result.append(cleaned_line)
    return '\n'.join(result)


def extract_config_block(text = None):
    '''
    从源代码中提取配置参数块注释和头部块。

    参数:
        text: 源代码文本

    返回:
        元组 (配置参数块文本, 头部块文本)
        如果未找到则返回空字符串
    '''
    header_block = ''
    include_pos = text.find('#include "pan211.h"')
    if include_pos > 0:
        header_block = text[:include_pos].strip()
    config_block = ''
    for i, line in enumerate(text.splitlines()):
        if not '配置参数' in line:
            if 'Configuration' in line:
                start_idx = text.find('/*', 0, text.find(line) + len(line))
                if start_idx >= 0:
                    end_match = CONFIG_BLOCK_END_RE.search(text, start_idx)
                    if end_match:
                        config_block = text[start_idx:end_match.end()]
                    
                    return (config_block, header_block)


def extract_unsigned_char_decls(text = None):
    """
    提取文本中声明为'unsigned char x, y;'的变量名。

    参数:
        text: 包含变量声明的代码文本

    返回:
        提取的变量名列表
    """
    names = []
    for m in re.finditer('\\bunsigned\\s+char\\s+([^;]+);', text):
        part = m.group(1)
        for seg in part.split(','):
            nm = seg.strip()
            if not nm:
                continue
            nm = nm.split('=')[0].strip()
            nm = nm.replace('*', '').strip()
            if nm and nm not in names:
                names.append(nm)
                continue
                continue
                return names


def convert_init_to_ret(lines = None, raw_lines = None):
    '''
    将初始化函数体转换为ret指令序列。

    参数:
        lines: 清理后的代码行列表（已去除注释）
        dyn_range: 动态校准区块的范围，格式为(开始行索引, 结束行索引)
        raw_lines: 原始代码行列表（包含注释）

    返回:
        转换后的ret指令列表
    '''
    ret = []
    i = 0
    inserted_reset_after_pg0 = False
    last_comment = None
    in_calibration_section = False
    calibration_start_comment = '/* 3. 读取工厂校准值 */'
    calibration_end_comment = '/* 4. 写入预配置page1寄存器 */'
    comment_writereg_re = re.compile('PAN211_WriteReg\\s*\\(\\s*(0x[0-9a-fA-F]+)\\s*,\\s*(0x[0-9a-fA-F]+)\\s*\\)')
    if i < len(lines):
        raw = raw_lines[i].strip()
        l = lines[i].strip()
        print(f'''[DEBUG] Processing line {i}: {raw}''')
        if raw.startswith('/*'):
            print(f'''[INFO] Processing comment line: {raw}''')
            comment_text = raw
            if calibration_start_comment in raw:
                in_calibration_section = True
                ret.append(f'''        /* {comment_text.strip('/* ').strip()} */''')
                ret.append('        ret 0x00; ret 0x01; // page1')
                ret.append('        ret 0x05; ret 0x00;')
                ret.append('        ret 0x04; ret 0x04;')
                ret.append('        ret 0xFB;')
                i += 1
                if i < len(lines) and calibration_end_comment not in raw_lines[i]:
                    i += 1
                    continue
                    continue
                elif calibration_end_comment in raw:
                    in_calibration_section = False
                    ret.append(f'''        /* {comment_text.strip('/* ').strip()} */''')
                    i += 1
                    continue
            if not in_calibration_section:
                write_reg_matches = comment_writereg_re.findall(comment_text)
                if write_reg_matches:
                    for addr, val in write_reg_matches:
                        comment_content = ''
                        if '/*' in comment_text and '*/' in comment_text:
                            parts = comment_text.split('/*', 1)
                            if len(parts) > 1:
                                comment_content = parts[1].split('*/', 1)[0].strip()
                        ret.append(f'''        ret {addr}; ret {val}; /* {comment_content} */''')
                else:
                    ret.append(f'''        /* {comment_text.strip('/* ').strip()} */''')
            last_comment = comment_text
            i += 1
            continue
        if in_calibration_section:
            i += 1
            continue
        pattern = re.compile('延时\\s*\\d+\\s*(ms|us)')
        if pattern.search(raw):
            print(f'''found 延时 in line: {raw}''')
            parts = raw.split('/*', 1)
            print(f'''split parts: {parts}''')
            p_content = raw
            for p in parts:
                print(f'''[INFO] Processing part: {p}''')
                if '延时' in p or '*/' in p:
                    p_content = p.split('*/', 1)[0].strip()
                else:
                    p_content = p.strip()
                tok = delay_to_token(p_content)
            continue
            if tok:
                ret.append(f'''        ret {tok}; /* {p_content} */''')
                i += 1
                continue
        if raw.replace(' ', '').startswith('//'):
            print(f'''[INFO] Processing //// commented line: {raw}''')
            write_reg_matches = comment_writereg_re.findall(raw)
            if write_reg_matches:
                for addr, val in write_reg_matches:
                    comment_prefix = '// '
                    comment_suffix = ''
                    if '/*' in raw and '*/' in raw:
                        parts = raw.split('/*', 1)
                        if len(parts) > 1:
                            comment_suffix = f''' /* {parts[1].split('*/', 1)[0].strip()} */'''
                    ret.append(f'''        {comment_prefix}ret {addr}; ret {val}; {comment_suffix}''')
            else:
                ret.append(f'''        /* {raw.strip('/ ')} */''')
            i += 1
            continue
        m = RE_WRITEREG.search(l)
        if m:
            addr = m.group(1)
            val = m.group(2)
            comment = ''
            if '/*' in raw:
                comment_parts = raw.split('/*', 1)
                if len(comment_parts) > 1 and '*/' in comment_parts[1]:
                    comment = f''' /* {comment_parts[1].split('*/', 1)[0].strip()} */'''
            if addr == '0x09' or addr == '0x0a':
                val = '0x08'
            if addr == '0x06':
                ret.append(f'''        // ret {addr}; ret {val};{comment}''')
            else:
                ret.append(f'''        ret {addr}; ret {val};{comment}''')
            if inserted_reset_after_pg0 and addr.lower() == '0x00' and val.lower() == '0x00':
                ret.append('        ret 0xFA; /* IIC复位 */')
                inserted_reset_after_pg0 = True
            i += 1
            continue
        i += 1
        continue
    ret.append('        ret 0xFF; /* 结束标记 */')
    return ret


def build_factory_func_body(dynamic_code = None, decls = None):
    '''
    构建工厂校准函数的函数体。

    参数:
        dynamic_code: 动态校准代码
        decls: 需要声明的变量名列表

    返回:
        工厂校准函数的函数体文本
    '''
    body = dynamic_code
    body = re.sub('\\bunsigned\\s+char\\b', 'byte', body)
    body = body.replace('return 0;', '// return ignored in factory calibration')
    decl_lines = (lambda .0: [ '    byte ' + n + ';' for n in .0 ])(decls) if decls else []
    lines = decl_lines + (lambda .0: [ '    ' + l.rstrip() for l in .0 if l.strip() ])(body.splitlines())
    return '\n'.join(lines)


def extra_functions():
    '''
    返回任何额外的辅助函数定义。

    返回:
        辅助函数定义字符串
    '''
    return '\nvoid PAN211_WriteReg(byte addr,byte value)\n{\n\tAddress = addr;\n    reg_value = value;\n    write_reg();\n}\n\nvoid PAN211_FactoryCalibration(void)\n{\n    Address = 0x04; read_reg();\n    iic_data[0] = Read_data;\n    PAN211_WriteReg(0x04, 0x08);\n    Address = 0x04; read_reg();\n    iic_data[1] = Read_data;\n    PAN211_WriteReg(0x05, 0x01);\n    PAN211_WriteReg(0x47, 0x83 | ((iic_data[0] >> 1) & 0x70));\n    PAN211_WriteReg(0x43, 0x10 | (0x01 - ((iic_data[1] >> 4) & 0x01)));\n}\n\n    '


def gen_pms_pan211(in_path, time_multiplier_str):
    '''
    主函数，处理命令行参数并执行转换。
    '''
    global TIME_MULTIPLIER
    in_path = Path(in_path)
    if time_multiplier_str not in TIME_MULTIPLIER_MAP:
        print(f'''Invalid time multiplier: {time_multiplier_str}''')
        print('Valid options: 2M, 4M, 8M')
        sys.exit(1)
    TIME_MULTIPLIER = TIME_MULTIPLIER_MAP[time_multiplier_str]
    print(f'''Using time multiplier: {time_multiplier_str} (factor: {TIME_MULTIPLIER})''')
    src = in_path.read_text('utf-8', 'ignore', **('encoding', 'errors'))
    (braceStart, braceEnd, init_body) = extract_function_body(src, FUNC_INIT_SIG_RE)
    init_lines_raw = init_body.splitlines()
    init_body_clean = strip_comments_keep_lines(init_body)
    init_lines = init_body_clean.splitlines()
    ret_lines = convert_init_to_ret(init_lines, init_lines_raw)
    factory_func = ''
    factory_func += 'void PAN211_FactoryCalibration(void)\n'
    factory_func += '{\n'
    factory_func += '    Address = 0x04; read_reg();\n'
    factory_func += '    iic_data[0] = Read_data;\n'
    factory_func += '    PAN211_WriteReg(0x04, 0x08);\n'
    factory_func += '    Address = 0x04; read_reg();\n'
    factory_func += '    iic_data[1] = Read_data;\n'
    factory_func += '    PAN211_WriteReg(0x05, 0x01);\n'
    factory_func += '    PAN211_WriteReg(0x47, 0x83 | ((iic_data[0] >> 1) & 0x70));\n'
    factory_func += '    PAN211_WriteReg(0x43, 0x10 | (0x01 - ((iic_data[1] >> 4) & 0x01)));\n'
    factory_func += '    PAN211_WriteReg(0, 0);\n'
    factory_func += '    PAN211_WriteReg(0x05, (iic_data[1] >> 4)|0xC0);\n'
    factory_func += '    PAN211_WriteReg(0, 1);\n'
    factory_func += '}\n'
    remaining_functions = extract_remaining_functions(src, braceEnd + 1)
    print('\n转换后的内容:')
    print('void __get_rf_init_reg()')
    print('{')
    print('    _pcadd{   /*  0xF0--1ms   0xF1--2ms  0xF2--55ms      0xFA--iic_reset  0xFB--calibration       0xFF--end*/')
    for line in ret_lines:
        print(line)
    print('    }')
    print('}')
    print('\n工厂校准函数:')
    print(factory_func)
    print('\n剩余函数:')
    print(remaining_functions)
    out_path = in_path.parent / 'pms_pan211.c'
    if out_path.exists():
        os.remove(out_path)
    (config_block, header_block) = extract_config_block(src)
    ret_func = 'void __get_rf_init_reg()\n{\n    _pcadd{   /*  0xF0--1ms   0xF1--2ms  0xF2--55ms      0xFA--iic_reset  0xFB--calibration       0xFF--end*/\n'
    ret_func += '\n'.join(ret_lines)
    ret_func += '\n    }\n}\n'
    out_content = f'''\n{header_block}\n\n#include\t"RF.h"\n#include \t"soft_iic.h"\nextern byte\tAddress;//reg addr\nextern byte reg_value;//Write\nextern byte\tRead_Data;//read\nextern byte iic_data[PAYLOAD_LEN]; \nextern byte iic_len;\nbyte index = 1;\n\n{ret_func}\nvoid PAN211_WriteReg(byte addr,byte value)\n{{\n    Address = addr;reg_value = value;write_reg();\n}}\n\nbyte PAN211_ReadReg(byte addr)\n{{\n    Address = addr; read_reg(); return Read_data;\n}}\n\n{factory_func if '0xFB' in str(ret_lines) else '// No dynamic calibration detected; factory function not generated.'}\n\nvoid pan211_ez_init(void)\n{{\n    index = 1;\n    while(1)\n    {{\n        A = index;\n        __get_rf_init_reg();\n        Address = A;\n        if(Address==0xF0)\n        {{\n            .delay {2000 * TIME_MULTIPLIER};\n        }}\n        else if(Address==0xF1)\n        {{\n            .delay {4000 * TIME_MULTIPLIER};\n        }}\n        else if(Address==0xF2)\n        {{\n            .delay {110000 * TIME_MULTIPLIER};\n        }}\n        else if(Address==0xFA)\n        {{\n            iic_reset();\n        }}\n        else if(Address==0xFB)\n        {{\n            PAN211_FactoryCalibration();\n        }}\n        else if(Address==0xFF)\n        {{\n            break;\n        }}\n        else \n        {{\n            index++;\n            A = index;\n            __get_rf_init_reg();\n            reg_value = A;\n            write_reg();\n        }}\n        index++;\n    }}\n}}\n\nvoid PAN211_Send(void)\n{{\n\tiic_len = PAYLOAD_LEN;\n\n\tAddress = 0x01;write_buff();\n\n\tAddress = 0x02;reg_value = 0x74;write_reg();\n\tAddress = 0x02;reg_value = 0x75;write_reg();\n\n    // 使用while循环等待irq标志\n\t//while(PB.7){{}}  // 等待irq\n\n    while(1)\n    {{\n        PAN211_GetIRQFlags();\n        if(Read_data&0x80)\n        {{\n            break;\n        }}\n    }}\n\n\tPAN211_ClearIRQFlags(0xff);\n}}\n\nvoid PAN211_Read(void)\n{{\n\tiic_len = PAYLOAD_LEN;\n\tAddress = TRX_FIFO;\n\tread_buff();\n}}\n\n/**\n * @brief 设置BLE白化初始值\n * @param Value 白化初始值\n */\nvoid PAN211_SetWhiteInitVal(byte Value)\n{{\n    PAN211_ReadReg(0x1A);\n    reg_value = (Read_data & 0x80) | (Value & 0x7F);\n    PAN211_WriteReg(0x1A, reg_value);\n}}\n\n/**\n * @brief 设置PAN211的接收地址\n * @param addr_len 地址长度\n */\nvoid PAN211_SetRxAddr(byte addr_len)\n{{\n    Address = 0X0F;\n    iic_len = addr_len;\n    write_buff();\n}}\n\n/**\n * @brief 设置PAN211的发送地址\n * @param addr_len 地址长度\n */\nvoid PAN211_SetTxAddr(byte addr_len)\n{{\n    Address = 0X14;\n    iic_len = addr_len;\n    write_buff();\n}}\n\n{remaining_functions}'''
    out_path.write_text(out_content, 'utf-8', **('encoding',))
    return out_path

if __name__ == '__main__':
    pass
