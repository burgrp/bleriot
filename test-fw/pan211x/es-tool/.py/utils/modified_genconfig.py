# Source Generated with Decompyle++
# File: modified_genconfig.pyc (Python 3.8)

import json
import sys
import argparse
from utils.genconfig_reg import *
import copy

class PAN211Attributes:
    PAN211_CRC_off = 0
    PAN211_CRC_1byte = 1
    PAN211_CRC_2byte = 2
    PAN211_CRC_3byte = 3
    PAN211_CRC_Max = 4
    USE_I2C = 1
    USE_SPI_3LINE = 2
    USE_SPI_4LINE = 3
    PAN211_PAGE0 = 0
    PAN211_PAGE1 = 1
    WMODE_CFG0 = 7
    PAN211_DR_1Mbps = 0
    PAN211_DR_2Mbps = 1
    PAN211_DR_250Kbps = 2
    PAN211_WIDTH_2BYTES = 2
    PAN211_WIDTH_3BYTES = 3
    PAN211_WIDTH_4BYTES = 4
    PAN211_WIDTH_5BYTES = 5
    PAN211_PIPE0 = 0
    PAN211_PIPE1 = 1
    PAN211_PIPE2 = 2
    PAN211_PIPE3 = 3
    PAN211_PIPE4 = 4
    PAN211_PIPE5 = 5
    PAN211_CHIPMODE_XN297 = 0
    PAN211_CHIPMODE_FS01 = 1
    PAN211_CHIPMODE_FS32 = 2
    PAN211_CHIPMODE_BLE = 3
    PAN211_WORKMODE_NORMAL = 0
    PAN211_WORKMODE_ENHANCE = 1
    PAN211_BLE_LEN_FILTER_DISABLE = 0
    PAN211_BLE_LEN_FILTER_EQUAL = 1
    PAN211_BLE_LEN_FILTER_EXCEED = 2
    PAN211_BLE_LEN_FILTER_BENEATH = 3
    PAN211_BLE_WhiteList_DISABLE = 0
    PAN211_BLE_WhiteList_1Byte = 1
    PAN211_BLE_WhiteList_2Byte = 2
    PAN211_BLE_WhiteList_3Byte = 3
    PAN211_BLE_WhiteList_4Byte = 4
    PAN211_BLE_WhiteList_5Byte = 5
    PAN211_BLE_WhiteList_6Byte = 6
    PAN211_BLE_WH_INIPHA_CH37 = 83
    PAN211_BLE_WH_INIPHA_CH38 = 51
    PAN211_BLE_WH_INIPHA_CH39 = 115
    PAN211_BLE_CHANNEL_CH37 = 2
    PAN211_BLE_CHANNEL_CH38 = 26
    PAN211_BLE_CHANNEL_CH39 = 80
    PAN211_TX_MODE_SINGLE = 0
    PAN211_TX_MODE_CONTINOUS = 1
    PAN211_RX_MODE_SINGLE = 0
    PAN211_RX_MODE_SINGLE_WITH_TIMEOUT = 1
    PAN211_RX_MODE_CONTINOUS = 2
    PAN211_ENHANCE_RX_MODE_CONTINOUS = 0
    PAN211_ENHANCE_RX_MODE_CONTINOUS_WITH_TIMEOUT = 1
    PAN211_ENDIAN_LITTLE = 0
    PAN211_ENDIAN_BIG = 1
    PAN211_PRIMODE_DIS = 0
    PAN211_PRIMODE_S2 = 1
    PAN211_PRIMODE_S8 = 2
    XN297_1Mbps_TxDev_250K = 0
    XN297_1Mbps_TxDev_300K = 1
    XN297_2Mbps_TxDev_500K = 2
    XN297_2Mbps_TxDev_600K = 3
    XN297_250Kbps_TxDev_170K = 4
    FS01_1Mbps_TxDev_160K = 5
    FS01_2Mbps_TxDev_320K = 6
    FS01_250Kbps_TxDev_160K = 7
    FS32_1Mbps_TxDev_170K = 8
    FS32_2Mbps_TxDev_330K = 9
    FS32_250Kbps_TxDev_170K = 10
    BLE_1Mbps_TxDev_250K = 11
    BLE_2Mbps_TxDev_500K = 12
    BLE_250Kbps_TxDev_170K = 13
    TxDemodTable = [
        [
            1,
            50,
            31,
            [
                30,
                31,
                16,
                24,
                31,
                16,
                16,
                31,
                16,
                16,
                31,
                16,
                24,
                31,
                16]],
        [
            1,
            51,
            63,
            [
                25,
                63,
                28,
                27,
                50,
                28,
                25,
                26,
                25,
                28,
                28,
                28,
                27,
                50,
                28]]]
    TxDemodIndex = [
        [
            PAN211_CHIPMODE_XN297,
            PAN211_DR_1Mbps,
            0],
        [
            PAN211_CHIPMODE_XN297,
            PAN211_DR_2Mbps,
            4],
        [
            PAN211_CHIPMODE_XN297,
            PAN211_DR_250Kbps,
            5],
        [
            PAN211_CHIPMODE_FS01,
            PAN211_DR_1Mbps,
            6],
        [
            PAN211_CHIPMODE_FS01,
            PAN211_DR_2Mbps,
            7],
        [
            PAN211_CHIPMODE_FS01,
            PAN211_DR_250Kbps,
            8],
        [
            PAN211_CHIPMODE_FS32,
            PAN211_DR_1Mbps,
            9],
        [
            PAN211_CHIPMODE_FS32,
            PAN211_DR_2Mbps,
            10],
        [
            PAN211_CHIPMODE_FS32,
            PAN211_DR_250Kbps,
            11],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_1Mbps,
            12],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_2Mbps,
            13],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_250Kbps,
            14]]
    RxDemodTable = [
        [
            0,
            56,
            [
                16,
                16,
                16,
                11,
                11,
                19],
            BITMASK_4_0],
        [
            0,
            56,
            [
                0,
                0,
                2,
                0,
                2,
                0],
            BIT6 | BIT5],
        [
            0,
            55,
            [
                96,
                96,
                96,
                107,
                107,
                90],
            BITMASK_6_0],
        [
            0,
            54,
            [
                0,
                0,
                1,
                0,
                1,
                0],
            BIT7],
        [
            0,
            54,
            [
                1,
                1,
                0,
                1,
                0,
                1],
            BIT6],
        [
            0,
            54,
            [
                5,
                5,
                0,
                4,
                0,
                6],
            BITMASK_3_0],
        [
            1,
            7,
            [
                7,
                7,
                7,
                5,
                7,
                8],
            BITMASK_7_4],
        [
            1,
            7,
            [
                5,
                5,
                5,
                4,
                5,
                6],
            BITMASK_3_0],
        [
            1,
            13,
            [
                9,
                9,
                9,
                6,
                9,
                11],
            BITMASK_5_0],
        [
            1,
            15,
            [
                15,
                15,
                15,
                10,
                15,
                18],
            BITMASK_4_0],
        [
            1,
            14,
            [
                0,
                1,
                1,
                0,
                0,
                0],
            BIT7],
        [
            1,
            14,
            [
                1,
                0,
                0,
                1,
                1,
                1],
            BIT6],
        [
            1,
            21,
            [
                1,
                0,
                0,
                1,
                1,
                1],
            BIT6],
        [
            1,
            92,
            [
                1,
                0,
                0,
                1,
                1,
                1],
            BIT7],
        [
            1,
            93,
            [
                0,
                1,
                1,
                0,
                0,
                0],
            BIT6],
        [
            1,
            10,
            [
                1,
                0,
                0,
                1,
                0,
                1],
            BIT7]]
    RxDemodIndex = [
        [
            PAN211_CHIPMODE_XN297,
            PAN211_DR_1Mbps,
            PAN211_PRIMODE_DIS,
            0],
        [
            PAN211_CHIPMODE_XN297,
            PAN211_DR_2Mbps,
            PAN211_PRIMODE_DIS,
            0],
        [
            PAN211_CHIPMODE_XN297,
            PAN211_DR_250Kbps,
            PAN211_PRIMODE_DIS,
            4],
        [
            PAN211_CHIPMODE_FS01,
            PAN211_DR_1Mbps,
            PAN211_PRIMODE_DIS,
            3],
        [
            PAN211_CHIPMODE_FS01,
            PAN211_DR_2Mbps,
            PAN211_PRIMODE_DIS,
            3],
        [
            PAN211_CHIPMODE_FS01,
            PAN211_DR_250Kbps,
            PAN211_PRIMODE_DIS,
            4],
        [
            PAN211_CHIPMODE_FS32,
            PAN211_DR_1Mbps,
            PAN211_PRIMODE_DIS,
            3],
        [
            PAN211_CHIPMODE_FS32,
            PAN211_DR_2Mbps,
            PAN211_PRIMODE_DIS,
            3],
        [
            PAN211_CHIPMODE_FS32,
            PAN211_DR_250Kbps,
            PAN211_PRIMODE_DIS,
            4],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_1Mbps,
            PAN211_PRIMODE_DIS,
            0],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_1Mbps,
            PAN211_PRIMODE_S2,
            1],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_1Mbps,
            PAN211_PRIMODE_S8,
            1],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_2Mbps,
            PAN211_PRIMODE_DIS,
            0],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_250Kbps,
            PAN211_PRIMODE_DIS,
            4],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_250Kbps,
            PAN211_PRIMODE_S2,
            2],
        [
            PAN211_CHIPMODE_BLE,
            PAN211_DR_250Kbps,
            PAN211_PRIMODE_S8,
            2]]
    PAN211_TXPWR_n45dBm = -45
    PAN211_TXPWR_n44dBm = -44
    PAN211_TXPWR_n41dBm = -41
    PAN211_TXPWR_n40dBm = -40
    PAN211_TXPWR_n37dBm = -37
    PAN211_TXPWR_n33dBm = -33
    PAN211_TXPWR_n29dBm = -29
    PAN211_TXPWR_n28dBm = -28
    PAN211_TXPWR_n25dBm = -25
    PAN211_TXPWR_n24dBm = -24
    PAN211_TXPWR_n23dBm = -23
    PAN211_TXPWR_n22dBm = -22
    PAN211_TXPWR_n21dBm = -21
    PAN211_TXPWR_n20dBm = -20
    PAN211_TXPWR_n19dBm = -19
    PAN211_TXPWR_n18dBm = -18
    PAN211_TXPWR_n17dBm = -17
    PAN211_TXPWR_n16dBm = -16
    PAN211_TXPWR_n15dBm = -15
    PAN211_TXPWR_n14dBm = -14
    PAN211_TXPWR_n13dBm = -13
    PAN211_TXPWR_n12dBm = -12
    PAN211_TXPWR_n11dBm = -11
    PAN211_TXPWR_n10dBm = -10
    PAN211_TXPWR_n9dBm = -9
    PAN211_TXPWR_n8dBm = -8
    PAN211_TXPWR_n7dBm = -7
    PAN211_TXPWR_n6dBm = -6
    PAN211_TXPWR_n5dBm = -5
    PAN211_TXPWR_n3dBm = -3
    PAN211_TXPWR_n2dBm = -2
    PAN211_TXPWR_n1dBm = -1
    PAN211_TXPWR_0dBm_LOWPWR = 99
    PAN211_TXPWR_0dBm = 0
    PAN211_TXPWR_1dBm = 1
    PAN211_TXPWR_2dBm = 2
    PAN211_TXPWR_3dBm = 3
    PAN211_TXPWR_4dBm = 4
    PAN211_TXPWR_5dBm = 5
    PAN211_TXPWR_6dBm = 6
    PAN211_TXPWR_7dBm = 7
    PAN211_TXPWR_8dBm = 8
    PAN211_TXPWR_9dBm = 9
    PAN211_TXPWR_10dBm = 10
    PAN211_TXPWR_11dBm = 11
    POWER_TABLE_SIZE = 6
    sop8_power_table_reg = [
        [
            PAN211_PAGE1,
            60,
            7],
        [
            PAN211_PAGE0,
            67,
            48],
        [
            PAN211_PAGE0,
            68,
            240],
        [
            PAN211_PAGE0,
            68,
            15],
        [
            PAN211_PAGE1,
            70,
            BIT0],
        [
            PAN211_PAGE1,
            70,
            12]]
    sop8_power_table = [
        [
            PAN211_TXPWR_11dBm,
            [
                7,
                3,
                15,
                12,
                0,
                0]],
        [
            PAN211_TXPWR_9dBm,
            [
                7,
                3,
                8,
                12,
                0,
                0]],
        [
            PAN211_TXPWR_8dBm,
            [
                7,
                3,
                8,
                6,
                0,
                0]],
        [
            PAN211_TXPWR_7dBm,
            [
                7,
                3,
                8,
                3,
                0,
                0]],
        [
            PAN211_TXPWR_6dBm,
            [
                7,
                3,
                8,
                4,
                0,
                1]],
        [
            PAN211_TXPWR_5dBm,
            [
                7,
                3,
                8,
                2,
                0,
                1]],
        [
            PAN211_TXPWR_4dBm,
            [
                7,
                3,
                8,
                0,
                0,
                1]],
        [
            PAN211_TXPWR_3dBm,
            [
                7,
                3,
                8,
                0,
                0,
                2]],
        [
            PAN211_TXPWR_2dBm,
            [
                3,
                3,
                8,
                2,
                0,
                3]],
        [
            PAN211_TXPWR_1dBm,
            [
                3,
                3,
                8,
                0,
                0,
                3]],
        [
            PAN211_TXPWR_0dBm,
            [
                3,
                3,
                8,
                4,
                1,
                3]],
        [
            PAN211_TXPWR_0dBm_LOWPWR,
            [
                7,
                1,
                8,
                15,
                0,
                0]],
        [
            PAN211_TXPWR_n1dBm,
            [
                4,
                3,
                8,
                0,
                1,
                3]],
        [
            PAN211_TXPWR_n2dBm,
            [
                7,
                1,
                15,
                15,
                0,
                1]],
        [
            PAN211_TXPWR_n5dBm,
            [
                7,
                1,
                15,
                15,
                1,
                3]],
        [
            PAN211_TXPWR_n7dBm,
            [
                3,
                1,
                8,
                8,
                1,
                3]],
        [
            PAN211_TXPWR_n8dBm,
            [
                3,
                1,
                8,
                4,
                1,
                1]],
        [
            PAN211_TXPWR_n10dBm,
            [
                3,
                1,
                8,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n11dBm,
            [
                3,
                1,
                6,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n12dBm,
            [
                3,
                1,
                5,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n14dBm,
            [
                3,
                1,
                4,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n16dBm,
            [
                3,
                1,
                3,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n19dBm,
            [
                3,
                1,
                2,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n23dBm,
            [
                3,
                1,
                1,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n25dBm,
            [
                2,
                1,
                1,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n28dBm,
            [
                1,
                1,
                1,
                8,
                1,
                0]],
        [
            PAN211_TXPWR_n33dBm,
            [
                3,
                1,
                0,
                0,
                1,
                0]],
        [
            PAN211_TXPWR_n37dBm,
            [
                0,
                1,
                0,
                0,
                0,
                0]],
        [
            PAN211_TXPWR_n40dBm,
            [
                0,
                1,
                0,
                0,
                1,
                0]]]
    XTAL_FREQ_32M = 32
    XTAL_FREQ_16M = 16
    RxLowGain = 0
    RxHighGain = 1


class PAN211MPCONFIG:
    
    def __init__(self = None):
        self.INTERFACE_MODE = PAN211Attributes.USE_SPI_3LINE
        self.Channel = 12
        self.TxPower = 0
        self.DataRate = PAN211Attributes.PAN211_DR_1Mbps
        self.ChipMode = PAN211Attributes.PAN211_CHIPMODE_XN297
        self.TxLen = 32
        self.RxLen = 32
        self.TxAddr = [
            204,
            204,
            204,
            204,
            204]
        self.RxAddr = [
            [
                True,
                [
                    192,
                    204,
                    204,
                    204,
                    204]],
            [
                True,
                [
                    193,
                    204,
                    204,
                    204,
                    204]],
            [
                True,
                [
                    194,
                    204,
                    204,
                    204,
                    204]],
            [
                True,
                [
                    195,
                    204,
                    204,
                    204,
                    204]],
            [
                True,
                [
                    196,
                    204,
                    204,
                    204,
                    204]],
            [
                True,
                [
                    197,
                    204,
                    204,
                    204,
                    204]]]
        self.EnTxNoAck = 1
        self.TxMode = PAN211Attributes.PAN211_TX_MODE_SINGLE
        self.RxMode = PAN211Attributes.PAN211_RX_MODE_CONTINOUS
        self.RxTimeoutUs = 2000
        self.Endian = 1
        self.crcSkipAddr = 0
        self.InterruptMask = 15
        self.IOMUX_EN = 0
        self.XTAL_FREQ = 32
        self.RxGain = 0
        self.TxDevSelect = 0
        self.EN_AGC = 0
        self.EnRxPlLenLimit = False
        self.EnManuPid = 0
        self.TRxDelayTimeUs = 0
        self.AutoDelayUs = 0
        self.AutoMaxCnt = 3
        self.EnDPL = 0
        self.EnWhite = 1
        self.Crc = PAN211Attributes.PAN211_CRC_2byte
        self.WorkMode = PAN211Attributes.PAN211_WORKMODE_NORMAL
        self.AddrWidth = PAN211Attributes.PAN211_WIDTH_5BYTES
        self.BLEHeadNum = 2
        self.BLEHead0 = 66
        self.BLEHead1 = 0
        self.S2S8Mode = PAN211Attributes.PAN211_PRIMODE_DIS
        self.WhiteInit = PAN211Attributes.PAN211_BLE_WH_INIPHA_CH37
        self.WhiteListMatchMode = PAN211Attributes.PAN211_BLE_WhiteList_DISABLE
        self.WhiteListOffset = 0
        self.WhiteList = [
            204,
            204,
            204,
            204,
            204,
            0]
        self.WhiteListLen = 5
        self.LengthFilterMode = PAN211Attributes.PAN211_BLE_LEN_FILTER_EQUAL



class PAN211xGENCONFIG:
    
    def __init__(self = None, DefaultPage0 = None, DefaultPage1 = None):
        self.PAN211Page0Table = None
        self.PAN211Page1Table = None
        self.DefaultRegPage0 = copy.deepcopy(DefaultPage0)
        self.DefaultRegPage1 = copy.deepcopy(DefaultPage1)
        self.RecommendRegPage0 = (lambda .0: [ reg[:] for reg in .0 ])(self.DefaultRegPage0)
        self.RecommendRegPage1 = (lambda .0: [ reg[:] for reg in .0 ])(self.DefaultRegPage1)

    
    def setPAN211Page0Table(self, table):
        self.PAN211Page0Table = table

    
    def setPAN211Page1Table(self, table):
        self.PAN211Page1Table = table

    
    def PAN211_InitRegConfig(self, page0table, page1table):
        for reg in page0table:
            for i in range(len(self.RecommendRegPage0)):
                if self.RecommendRegPage0[i][0] == reg[0]:
                    self.RecommendRegPage0[i][1] = reg[1]
                    continue
                    continue
                    continue
                    for reg in page1table:
                        for i in range(len(self.RecommendRegPage1)):
                            if self.RecommendRegPage1[i][0] == reg[0]:
                                self.RecommendRegPage1[i][1] = reg[1]
                                continue
                                continue
                                continue
                                return None

    
    def PAN211_ReadReg(self, Page, Addr):
        if Page == 0:
            for i in range(len(self.RecommendRegPage0)):
                if self.RecommendRegPage0[i][0] == Addr:
                    return self.RecommendRegPage0[i][1]
        if Page == 1:
            for i in range(len(self.RecommendRegPage1)):
                if self.RecommendRegPage1[i][0] == Addr:
                    return self.RecommendRegPage1[i][1]
                return None

    
    def PAN211_WriteReg(self, Page, Addr, Value):
        if Page == PAN211_PAGE0:
            for i in range(len(self.RecommendRegPage0)):
                if self.RecommendRegPage0[i][0] == Addr:
                    self.RecommendRegPage0[i][1] = Value
                
        if Page == PAN211_PAGE1:
            for i in range(len(self.RecommendRegPage1)):
                if self.RecommendRegPage1[i][0] == Addr:
                    self.RecommendRegPage1[i][1] = Value
                
                return None

    
    def PAN211_WriteRegs(self, Page, Addr, Value, Len):
        if Page == PAN211_PAGE0:
            for i in range(Len):
                self.PAN211_WriteReg(PAN211_PAGE0, Addr + i, Value[i])
        elif Page == PAN211_PAGE1:
            for i in range(Len):
                self.PAN211_WriteReg(PAN211_PAGE1, Addr + i, Value[i])

    
    def get_shift(self, value):
        shift = 0
        if value & 1 != 1:
            value = value >> 1
            shift += 1
            continue
        return shift

    
    def PAN211_WriteRegBits(self, Page, Addr, BitValue, BitMask):
        if Page == PAN211_PAGE0:
            for i in range(len(self.RecommendRegPage0)):
                if self.RecommendRegPage0[i][0] == Addr:
                    self.RecommendRegPage0[i][1] = self.RecommendRegPage0[i][1] & ~BitMask | BitValue << self.get_shift(BitMask) & BitMask
                
        if Page == PAN211_PAGE1:
            for i in range(len(self.RecommendRegPage1)):
                if self.RecommendRegPage1[i][0] == Addr:
                    self.RecommendRegPage1[i][1] = self.RecommendRegPage1[i][1] & ~BitMask | BitValue << self.get_shift(BitMask) & BitMask
                
                return None

    
    def PAN211_SetChannel(self, Channel):
        self.PAN211_WriteReg(PAN211_PAGE0, REG_P0_0X39, Channel)

    
    def PAN211_SetRxAddr(self, Pipe, RxAddr, AddrWidth):
        if isinstance(RxAddr, list) and all((lambda .0: for item in .0:
isinstance(item, int))(RxAddr)):
            RxAddr = (lambda .0: [ int(item) for item in .0 ])(RxAddr)
        elif all((lambda .0: for item in .0:
if isinstance(item, str):
passitem.startswith('0x'))(RxAddr)):
            RxAddr = (lambda .0: [ int(item, 16) for item in .0 ])(RxAddr)
        if Pipe == PAN211Attributes.PAN211_PIPE0:
            self.PAN211_WriteRegs(PAN211_PAGE0, PIPE0_RXADDR0_CFG, RxAddr, AddrWidth)
        elif Pipe == PAN211Attributes.PAN211_PIPE1:
            self.PAN211_WriteRegs(PAN211_PAGE0, PIPE1_RXADDR0_CFG, RxAddr, AddrWidth)
        elif Pipe == PAN211Attributes.PAN211_PIPE2:
            self.PAN211_WriteReg(PAN211_PAGE0, PIPE2_RXADDR0_CFG, RxAddr[0])
        elif Pipe == PAN211Attributes.PAN211_PIPE3:
            self.PAN211_WriteReg(PAN211_PAGE0, PIPE3_RXADDR0_CFG, RxAddr[0])
        elif Pipe == PAN211Attributes.PAN211_PIPE4:
            self.PAN211_WriteReg(PAN211_PAGE0, PIPE4_RXADDR0_CFG, RxAddr[0])
        elif Pipe == PAN211Attributes.PAN211_PIPE5:
            self.PAN211_WriteReg(PAN211_PAGE0, PIPE5_RXADDR0_CFG, RxAddr[0])

    
    def PAN211_SetTxAddr(self, Addr, len):
        if isinstance(Addr, list) and all((lambda .0: for item in .0:
isinstance(item, int))(Addr)):
            Addr = (lambda .0: [ int(item) for item in .0 ])(Addr)
        elif all((lambda .0: for item in .0:
if isinstance(item, str):
passitem.startswith('0x'))(Addr)):
            Addr = (lambda .0: [ int(item, 16) for item in .0 ])(Addr)
        self.PAN211_WriteRegs(PAN211_PAGE0, TXADDR0_CFG, Addr, len)

    
    def PAN211_SetAddrWidth(self, AddrWidth):
        self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, AddrWidth - 2, WMODE_CFG1_ADDR_BYTE_LENGTH)

    
    def WriteTxDemodConfig(self, ChipMode, DataRate):
        for i in range(len(PAN211Attributes.TxDemodIndex)):
            if ChipMode == PAN211Attributes.TxDemodIndex[i][0] and DataRate == PAN211Attributes.TxDemodIndex[i][1]:
                index = PAN211Attributes.TxDemodIndex[i][2]
                self.PAN211_WriteRegBits(PAN211Attributes.TxDemodTable[0][0], PAN211Attributes.TxDemodTable[0][1], PAN211Attributes.TxDemodTable[0][3][index], PAN211Attributes.TxDemodTable[0][2])
                self.PAN211_WriteRegBits(PAN211Attributes.TxDemodTable[1][0], PAN211Attributes.TxDemodTable[1][1], PAN211Attributes.TxDemodTable[1][3][index], PAN211Attributes.TxDemodTable[1][2])
                return None
            return None

    
    def WriteRxDemodConfig(self, ChipMode, DataRate, S2S8Mode):
        for i in range(len(PAN211Attributes.RxDemodIndex)):
            if ChipMode == PAN211Attributes.RxDemodIndex[i][0] and DataRate == PAN211Attributes.RxDemodIndex[i][1] and S2S8Mode == PAN211Attributes.RxDemodIndex[i][2]:
                index = PAN211Attributes.RxDemodIndex[i][3]
                for i in range(16):
                    self.PAN211_WriteRegBits(PAN211Attributes.RxDemodTable[i][0], PAN211Attributes.RxDemodTable[i][1], PAN211Attributes.RxDemodTable[i][2][index], PAN211Attributes.RxDemodTable[i][3])
                return None
            return None

    
    def PAN211_DRModConfig(self, ChipMode, DataRate):
        if DataRate == PAN211Attributes.PAN211_DR_1Mbps:
            if ChipMode == PAN211Attributes.PAN211_CHIPMODE_FS32 or ChipMode == PAN211Attributes.PAN211_CHIPMODE_FS01:
                self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 0, BIT7)
            else:
                self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 1, BIT7)
            self.PAN211_WriteRegBits(PAN211_PAGE0, 67, 0, BIT2)
            self.PAN211_WriteRegBits(PAN211_PAGE0, 67, 2, BIT1 | BIT0)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 58, 0, BIT7 | BIT6)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 4, BIT6 | BIT5 | BIT4)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 76, 0, BIT5)
        elif DataRate == PAN211Attributes.PAN211_DR_250Kbps:
            self.PAN211_WriteRegBits(PAN211_PAGE0, 67, 0, BIT2)
            self.PAN211_WriteRegBits(PAN211_PAGE0, 67, 3, BIT1 | BIT0)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 58, 1, BIT7 | BIT6)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 0, BIT7)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 4, BIT6 | BIT5 | BIT4)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 76, 0, BIT5)
        elif DataRate == PAN211Attributes.PAN211_DR_2Mbps:
            if ChipMode == PAN211Attributes.PAN211_CHIPMODE_FS32 or ChipMode == PAN211Attributes.PAN211_CHIPMODE_FS01:
                self.PAN211_WriteRegBits(PAN211_PAGE0, 67, 0, BIT2)
                self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 6, BIT6 | BIT5 | BIT4)
                self.PAN211_WriteRegBits(PAN211_PAGE1, 76, 0, BIT5)
            else:
                self.PAN211_WriteRegBits(PAN211_PAGE0, 67, 1, BIT2)
                self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 4, BIT6 | BIT5 | BIT4)
                self.PAN211_WriteRegBits(PAN211_PAGE1, 76, 1, BIT5)
            self.PAN211_WriteRegBits(PAN211_PAGE0, 67, 2, BIT1 | BIT0)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 58, 0, BIT7 | BIT6)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 73, 1, BIT7)

    
    def PAN211_EnableFifo128bytes(self, Enable):
        if Enable:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 1, WMODE_CFG1_FIFO_128_EN)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 0, WMODE_CFG1_FIFO_128_EN)

    
    def PAN211_SetTxMode(self, TxMode):
        if TxMode == PAN211Attributes.PAN211_TX_MODE_SINGLE:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, PAN211Attributes.PAN211_TX_MODE_SINGLE, TRXMODE_CFG_REG_TX_CFG_MODE)
        elif TxMode == PAN211Attributes.PAN211_TX_MODE_CONTINOUS:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, PAN211Attributes.PAN211_TX_MODE_CONTINOUS, TRXMODE_CFG_REG_TX_CFG_MODE)

    
    def PAN211_SetRxMode(self, RxMode):
        if RxMode == PAN211Attributes.PAN211_RX_MODE_SINGLE:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, PAN211Attributes.PAN211_RX_MODE_SINGLE, TRXMODE_CFG_REG_RX_CFG_MODE)
        elif RxMode == PAN211Attributes.PAN211_RX_MODE_SINGLE_WITH_TIMEOUT:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, PAN211Attributes.PAN211_RX_MODE_SINGLE_WITH_TIMEOUT, TRXMODE_CFG_REG_RX_CFG_MODE)
        elif RxMode == PAN211Attributes.PAN211_RX_MODE_CONTINOUS:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, PAN211Attributes.PAN211_RX_MODE_CONTINOUS, TRXMODE_CFG_REG_RX_CFG_MODE)

    
    def PAN211_EnableDynamicPL(self, EnDPL):
        if EnDPL:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 1, WMODE_CFG1_DPY_EN)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 0, WMODE_CFG1_DPY_EN)

    
    def PAN211_SetCrcScheme(self, Crc):
        self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, Crc, WMODE_CFG0_CRC_MODE_1_0)

    
    def PAN211_SetDataRate(self, ChipMode, DataRate, S2S8Mode):
        DataRateVal = 0
        if DataRate == PAN211Attributes.PAN211_DR_1Mbps:
            DataRateVal = 0
        elif DataRate == PAN211Attributes.PAN211_DR_2Mbps:
            DataRateVal = 1
        elif DataRate == PAN211Attributes.PAN211_DR_250Kbps:
            DataRateVal = 3
        self.PAN211_WriteRegBits(PAN211_PAGE0, REG_P0_0X36, DataRateVal, REG_P0_0X36_BW_MODE)
        self.PAN211_DRModConfig(ChipMode, DataRate)
        self.WriteTxDemodConfig(ChipMode, DataRate)
        self.WriteRxDemodConfig(ChipMode, DataRate, S2S8Mode)

    
    def PAN211_EnableRxPipe(self, Pipe):
        self.PAN211_WriteRegBits(PAN211_PAGE0, RXPIPE_CFG, 1, 1 << Pipe)

    
    def PAN211_DisableRxPipe(self, Pipe):
        self.PAN211_WriteRegBits(PAN211_PAGE0, RXPIPE_CFG, 0, 1 << Pipe)

    
    def PAN211_SetRxPayloadLen(self, PayloadLen):
        self.PAN211_WriteReg(PAN211_PAGE0, RXPLLEN_CFG, PayloadLen)

    
    def PAN211_SetTxPayloadLen(self, PayloadLen):
        self.PAN211_WriteReg(PAN211_PAGE0, TXPLLEN_CFG, PayloadLen)

    
    def PAN211_SetWorkMode(self, Mode):
        if Mode == PAN211Attributes.PAN211_WORKMODE_NORMAL:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 0, WMODE_CFG1_NORMAL_M1 | WMODE_CFG1_ENHANCE)
        elif Mode == PAN211Attributes.PAN211_WORKMODE_ENHANCE:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 1, WMODE_CFG1_ENHANCE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 0, WMODE_CFG1_NORMAL_M1)

    
    def PAN211_EnableTxNoAck(self, EnTxNoAck):
        modeCfg1 = self.PAN211_ReadReg(PAN211_PAGE0, WMODE_CFG1)
        if modeCfg1 & WMODE_CFG1_ENHANCE:
            if EnTxNoAck:
                self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_TX_NOACK_EN)
            else:
                self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 0, WMODE_CFG0_TX_NOACK_EN)
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 0, WMODE_CFG1_NORMAL_M1)
        elif EnTxNoAck:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 0, WMODE_CFG1_NORMAL_M1)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 1, WMODE_CFG1_NORMAL_M1)
        self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 0, WMODE_CFG0_TX_NOACK_EN)

    
    def PAN211_SetAckPipe(self, pipe):
        self.PAN211_WriteRegBits(PAN211_PAGE0, 111, pipe, 7)

    
    def PAN211_CrcSkipAddr(self, Skip):
        if Skip:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_ACCADDR_CRC_DIS)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 0, WMODE_CFG0_ACCADDR_CRC_DIS)

    
    def PAN211_WhiteSkipAddr(self, scrSkipAddr):
        if scrSkipAddr:
            self.PAN211_WriteRegBits(PAN211_PAGE0, SCR_CFG, 1, SCR_CFG_ACCADDR_SCR_DIS)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, SCR_CFG, 0, SCR_CFG_ACCADDR_SCR_DIS)

    
    def PAN211_SetWhiteInitVal(self, WhiteInit):
        self.PAN211_WriteRegBits(PAN211_PAGE0, SCR_CFG, WhiteInit, SCR_CFG_SCR_INI)

    
    def PAN211_SetEndian(self, Endian):
        if Endian == PAN211Attributes.PAN211_ENDIAN_LITTLE:
            self.PAN211_WriteRegBits(PAN211_PAGE0, 111, 1, BIT4)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, 111, 0, BIT4)
        self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, Endian, WMODE_CFG0_ENDIAN)

    
    def PAN211_EnableWhiten(self, EnWhite):
        if EnWhite:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_SCR_ENABLE)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 0, WMODE_CFG0_SCR_ENABLE)

    
    def PAN211_SetBleWhitelist(self, Start, FilterBuf, FilterLen):
        '''
        uint8_t start_reg = WLIST0_CFG + 5;

        if (FilterLen > 6)
        {
            return PAN211_ERR;
        }

        P_ASSERT(PAN211_WritePageRegBits(PAN211_PAGE0, BLEMATCHSTART_CFG, Start, BLEMATCHSTART_CFG_PLD_START_BYTE));
        start_reg -= (FilterLen-1);
        P_ASSERT(PAN211_WritePageRegs(PAN211_PAGE0, start_reg, FilterBuf, FilterLen));

        Args:
            BleWLMatchStart (_type_): _description_
            BleFilterWL (_type_): _description_
            filterLen (_type_): _description_
        '''
        start_reg = WLIST0_CFG + 5
        self.PAN211_WriteRegBits(PAN211_PAGE0, BLEMATCHSTART_CFG, Start, BLEMATCHSTART_CFG_PLD_START_BYTE)
        start_reg -= FilterLen - 1
        if isinstance(FilterBuf, list) and all((lambda .0: for item in .0:
isinstance(item, int))(FilterBuf)):
            FilterBuf = (lambda .0: [ int(item) for item in .0 ])(FilterBuf)
        elif all((lambda .0: for item in .0:
if isinstance(item, str):
passitem.startswith('0x'))(FilterBuf)):
            FilterBuf = (lambda .0: [ int(item, 16) for item in .0 ])(FilterBuf)
        self.PAN211_WriteRegs(PAN211_PAGE0, start_reg, FilterBuf, FilterLen)

    
    def PAN211_SetBleLenFilter(self, FilterType):
        self.PAN211_WriteRegBits(PAN211_PAGE0, BLEMATCH_CFG0, FilterType, BLEMATCH_CFG0_BLELEN_MATCH_MODE)

    
    def PAN211_SetBleWLMatchMode(self, MatchMode):
        self.PAN211_WriteRegBits(PAN211_PAGE0, BLEMATCH_CFG0, MatchMode, BLEMATCH_CFG0_WL_MATCH_MODE)

    
    def PAN211_EnableManualPid(self, EnManualPid):
        if EnManualPid:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, PID_CFG_PID_MANUAL_EN)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 0, PID_CFG_PID_MANUAL_EN)

    
    def PAN211_SetWaitAckTimeout(self, AckTimeoutUs):
        time = [
            AckTimeoutUs & 255,
            AckTimeoutUs >> 8 & 255]
        return self.PAN211_WriteRegs(PAN211_PAGE0, RXTIMEOUTL_CFG, time, 2)

    
    def PAN211_SetTRxTransTime(self, TransWaitTimeUs):
        time = [
            TransWaitTimeUs & 255,
            TransWaitTimeUs >> 8 & 255]
        return self.PAN211_WriteRegs(PAN211_PAGE0, TRXTWTL_CFG, time, 2)

    
    def PAN211_ConfigIT(self, RF_IT):
        self.PAN211_WriteReg(PAN211_PAGE0, RFIRQ_CFG, 255 - RF_IT)

    
    def PAN211_EnableInterfaceMuxIRQ(self, NewState, INTERFACE_MODE):
        if NewState:
            if INTERFACE_MODE == PAN211Attributes.USE_SPI_3LINE:
                self.PAN211_WriteRegBits(PAN211_PAGE0, SYS_CFG, 1, SYS_CFG_IRQ_MOSI_MUX_EN)
                self.PAN211_WriteRegBits(PAN211_PAGE0, LP_CFG, 0, LP_CFG_IRQ_I2C_MUX_EN)
            elif INTERFACE_MODE == PAN211Attributes.USE_I2C:
                self.PAN211_WriteRegBits(PAN211_PAGE0, SYS_CFG, 0, SYS_CFG_IRQ_MOSI_MUX_EN)
                self.PAN211_WriteRegBits(PAN211_PAGE0, LP_CFG, 1, LP_CFG_IRQ_I2C_MUX_EN)
            else:
                self.PAN211_WriteRegBits(PAN211_PAGE0, SYS_CFG, 0, SYS_CFG_IRQ_MOSI_MUX_EN)
                self.PAN211_WriteRegBits(PAN211_PAGE0, LP_CFG, 0, LP_CFG_IRQ_I2C_MUX_EN)

    
    def PAN211_SetAutoRetrans(self, DelayUs, MaxCnt):
        if DelayUs < 250:
            DelayUs = 250
        DelayUs /= 250
        DelayUs -= 1
        self.PAN211_WriteRegBits(PAN211_PAGE0, TXAUTO_CFG, int(DelayUs) & 255, TXAUTO_CFG_ARD)
        self.PAN211_WriteRegBits(PAN211_PAGE0, TXAUTO_CFG, MaxCnt & 255, TXAUTO_CFG_ARC_3_0)

    
    def PAN211_RxLengthLimit(self, EnRxPlLenLimit):
        if EnRxPlLenLimit:
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 1, PKT_EXT_CFG_W_RX_MAX_CTRL_EN)
        else:
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 0, PKT_EXT_CFG_W_RX_MAX_CTRL_EN)

    
    def PAN211_SetTxPower(self, ChipMode, DataRate, TxPower):
        for i in range(len(PAN211Attributes.sop8_power_table)):
            if PAN211Attributes.sop8_power_table[i][0] == TxPower:
                for j in range(len(PAN211Attributes.sop8_power_table_reg)):
                    self.PAN211_WriteRegBits(PAN211Attributes.sop8_power_table_reg[j][0], PAN211Attributes.sop8_power_table_reg[j][1], PAN211Attributes.sop8_power_table[i][1][j], PAN211Attributes.sop8_power_table_reg[j][2])
            
            CodeOffsetValue = self.GetCodeOffset(ChipMode, DataRate, TxPower)
            self.PAN211_WriteReg(PAN211_PAGE1, 39, CodeOffsetValue)
            if TxPower == PAN211Attributes.PAN211_TXPWR_0dBm_LOWPWR:
                self.PAN211_WriteRegBits(PAN211_PAGE1, 72, 12, BITMASK_3_0)
                self.PAN211_WriteRegBits(PAN211_PAGE1, 60, 0, BIT3)
            elif TxPower == PAN211Attributes.PAN211_TXPWR_11dBm:
                self.PAN211_WriteRegBits(PAN211_PAGE1, 72, 15, BITMASK_3_0)
                self.PAN211_WriteRegBits(PAN211_PAGE1, 60, 1, BIT3)
            else:
                self.PAN211_WriteRegBits(PAN211_PAGE1, 72, 8, BITMASK_3_0)
                self.PAN211_WriteRegBits(PAN211_PAGE1, 60, 0, BIT3)

    
    def GetCodeOffset(self, Chipmode, Datarate, TxPower):
        '''
        Get page1 0x27 register value based on chipmode, datarate and power level
        According to 1.md configuration table
        '''
        if TxPower >= PAN211Attributes.PAN211_TXPWR_9dBm:
            pass
        IsHighPower = TxPower != PAN211Attributes.PAN211_TXPWR_0dBm_LOWPWR
        if Chipmode == PAN211Attributes.PAN211_CHIPMODE_XN297:
            if Datarate == PAN211Attributes.PAN211_DR_1Mbps:
                return 170
            if None == PAN211Attributes.PAN211_DR_2Mbps:
                return 202
            if None == PAN211Attributes.PAN211_DR_250Kbps:
                return 10
        if Chipmode == PAN211Attributes.PAN211_CHIPMODE_BLE:
            if Datarate == PAN211Attributes.PAN211_DR_1Mbps:
                return 170
            if None == PAN211Attributes.PAN211_DR_2Mbps:
                return 202
            if None == PAN211Attributes.PAN211_DR_250Kbps:
                return 10
        if Chipmode == PAN211Attributes.PAN211_CHIPMODE_FS01:
            if Datarate == PAN211Attributes.PAN211_DR_1Mbps:
                if IsHighPower:
                    return 170
                return None
            if None == PAN211Attributes.PAN211_DR_2Mbps:
                if IsHighPower:
                    return 170
                return None
            if None == PAN211Attributes.PAN211_DR_250Kbps:
                return 10
        if Chipmode == PAN211Attributes.PAN211_CHIPMODE_FS32:
            if Datarate == PAN211Attributes.PAN211_DR_1Mbps:
                if IsHighPower:
                    return 170
                return None
            if None == PAN211Attributes.PAN211_DR_2Mbps:
                if IsHighPower:
                    return 170
                return None
            if None == PAN211Attributes.PAN211_DR_250Kbps:
                return 10
            return None

    
    def PAN211_CarrierWave(self):
        self.PAN211_WriteReg(PAN211_PAGE0, 2, 116)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 3, 1, BIT6)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 6, 1, BIT7)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 6, 1, BIT5)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 6, 1, BIT6)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 6, 1, BIT4)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 106, 1, BIT1)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 106, 1, BIT0)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 106, 1, BIT5)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 106, 1, BIT3)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 107, 1, BIT7)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 107, 1, BIT6)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 3, 1, BIT7)

    
    def PAN211_ExitCarrierWave(self):
        self.PAN211_WriteReg(PAN211_PAGE0, 106, 0)
        self.PAN211_WriteReg(PAN211_PAGE0, 107, 0)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 6, 0, BIT7 | BIT5 | BIT6 | BIT4)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 3, 0, BIT7)
        self.PAN211_WriteRegBits(PAN211_PAGE0, 3, 0, BIT6)

    
    def PAN211_SetNordicPktHeader(self, HeaderEn, HeaderLen):
        self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, HeaderEn, PKT_EXT_CFG_HDR_LEN_EXIST)
        self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, HeaderLen, PKT_EXT_CFG_HDR_LEN_NUMB)

    
    def PAN211_WriteNordicPktHeader(self, Header0, Header1, Length):
        self.PAN211_WriteReg(PAN211_PAGE0, TXHDR0_CFG, Header0)
        self.PAN211_WriteReg(PAN211_PAGE0, TXHDR1_CFG, Header1)
        self.PAN211_WriteReg(PAN211_PAGE0, TXPLLEN_CFG, Length)

    
    def PAN211_SetS2S8Mode(self, mode):
        if mode == PAN211Attributes.PAN211_PRIMODE_DIS:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, 1, TRXMODE_CFG_W_PRE_SYNC_EN)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 0, PKT_EXT_CFG_PRI_CI_MODE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 0, PKT_EXT_CFG_PRI_TX_FEC)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 0, PKT_EXT_CFG_PRI_RX_FEC)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 11, 0, BIT5)
        elif mode == PAN211Attributes.PAN211_PRIMODE_S2:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, 0, TRXMODE_CFG_W_PRE_SYNC_EN)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 1, PKT_EXT_CFG_PRI_CI_MODE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 1, PKT_EXT_CFG_PRI_TX_FEC)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 1, PKT_EXT_CFG_PRI_RX_FEC)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 11, 1, BIT5)
        elif mode == PAN211Attributes.PAN211_PRIMODE_S8:
            self.PAN211_WriteRegBits(PAN211_PAGE0, TRXMODE_CFG, 0, TRXMODE_CFG_W_PRE_SYNC_EN)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 0, PKT_EXT_CFG_PRI_CI_MODE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 1, PKT_EXT_CFG_PRI_TX_FEC)
            self.PAN211_WriteRegBits(PAN211_PAGE0, PKT_EXT_CFG, 1, PKT_EXT_CFG_PRI_RX_FEC)
            self.PAN211_WriteRegBits(PAN211_PAGE1, 11, 1, BIT5)

    
    def PAN211_SetChipMode(self, ChipMode, Endian, crcsKipAddr):
        if ChipMode == PAN211Attributes.PAN211_CHIPMODE_XN297:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 0, WMODE_CFG0_CHIP_MODE)
            self.PAN211_SetEndian(PAN211Attributes.PAN211_ENDIAN_BIG)
            self.PAN211_CrcSkipAddr(False)
            self.PAN211_WhiteSkipAddr(False)
            self.PAN211_SetWhiteInitVal(127)
        elif ChipMode == PAN211Attributes.PAN211_CHIPMODE_FS01:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_CHIP_MODE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 0, WMODE_CFG0_NORDIC_ENHANCE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, REG_P0_0X6F, 0, REG_P0_0X6F_I_NDC_PREAMBLE_SEL)
            self.PAN211_SetEndian(PAN211Attributes.PAN211_ENDIAN_BIG)
            self.PAN211_CrcSkipAddr(False)
            self.PAN211_WhiteSkipAddr(False)
            self.PAN211_SetWhiteInitVal(127)
        elif ChipMode == PAN211Attributes.PAN211_CHIPMODE_FS32:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_CHIP_MODE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_NORDIC_ENHANCE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, REG_P0_0X6F, 1, REG_P0_0X6F_I_NDC_PREAMBLE_SEL)
            self.PAN211_SetEndian(Endian)
            self.PAN211_CrcSkipAddr(crcsKipAddr)
            self.PAN211_WhiteSkipAddr(True)
            self.PAN211_SetWhiteInitVal(127)
        elif ChipMode == PAN211Attributes.PAN211_CHIPMODE_BLE:
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_CHIP_MODE)
            self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG0, 1, WMODE_CFG0_NORDIC_ENHANCE)
            self.PAN211_SetEndian(PAN211Attributes.PAN211_ENDIAN_LITTLE)
            self.PAN211_CrcSkipAddr(True)
            self.PAN211_WhiteSkipAddr(True)
        self.PAN211_WriteRegBits(PAN211_PAGE0, WMODE_CFG1, 1, WMODE_CFG1_RX_GOON)

    
    def PAN211_SetXTALFreq(self, freq, EN_AGC, DataRate):
        if freq == PAN211Attributes.XTAL_FREQ_16M:
            self.PAN211_WriteReg(PAN211_PAGE1, 65, 166)
            if EN_AGC == 0 and DataRate == PAN211Attributes.PAN211_DR_2Mbps:
                self.PAN211_WriteReg(PAN211_PAGE1, 63, 210)
                self.PAN211_WriteReg(PAN211_PAGE1, 64, 32)
            elif EN_AGC == 1:
                self.PAN211_WriteReg(PAN211_PAGE1, 63, 210)
                self.PAN211_WriteReg(PAN211_PAGE1, 64, 32)
            elif freq == PAN211Attributes.XTAL_FREQ_32M:
                self.PAN211_WriteReg(PAN211_PAGE1, 65, 162)

    
    def PAN211_SetRxGain(self, RxGain, EN_AGC):
        if RxGain == PAN211Attributes.RxLowGain:
            self.PAN211_WriteReg(PAN211_PAGE0, 97, 46)
            if EN_AGC == 1:
                self.PAN211_WriteReg(PAN211_PAGE0, 93, 220)
            else:
                self.PAN211_WriteRegBits(PAN211_PAGE0, 78, 46, 63)
        elif RxGain == PAN211Attributes.RxHighGain:
            self.PAN211_WriteReg(PAN211_PAGE0, 97, 62)
            if EN_AGC == 1:
                self.PAN211_WriteReg(PAN211_PAGE0, 93, 212)
            else:
                self.PAN211_WriteRegBits(PAN211_PAGE0, 78, 62, 63)
        else:
            raise ValueError('RxGain must be 0 or 1')

    
    def PAN211_SetPredefinedRegs(self, InterfaceMode, freq = (None, None)):
        self.modifyDefaultRegisters(PAN211_PAGE0, 2, 116)
        self.PAN211_WriteReg(PAN211_PAGE0, 2, 116)
        if freq == PAN211Attributes.XTAL_FREQ_16M:
            self.modifyDefaultRegisters(PAN211_PAGE0, 55, 224)
            self.PAN211_WriteReg(PAN211_PAGE0, 55, 224)
        if InterfaceMode == PAN211Attributes.USE_SPI_3LINE:
            self.modifyDefaultRegisters(PAN211_PAGE0, 4, 131)
            self.PAN211_WriteReg(PAN211_PAGE0, 4, 131)
            self.modifyDefaultRegisters(PAN211_PAGE0, 3, 2)
            self.PAN211_WriteReg(PAN211_PAGE0, 3, 2)
        elif InterfaceMode == PAN211Attributes.USE_SPI_4LINE:
            self.modifyDefaultRegisters(PAN211_PAGE0, 4, 3)
            self.PAN211_WriteReg(PAN211_PAGE0, 4, 3)
            self.modifyDefaultRegisters(PAN211_PAGE0, 3, 3)
            self.PAN211_WriteReg(PAN211_PAGE0, 3, 3)

    
    def PAN211_WriteRecommendedRegs(self, EN_AGC):
        self.PAN211_WriteReg(PAN211_PAGE1, 39, 202)
        if EN_AGC:
            self.PAN211_WriteReg(PAN211_PAGE1, 55, 21)
            self.PAN211_WriteReg(PAN211_PAGE1, 58, 20)
        self.PAN211_WriteReg(PAN211_PAGE1, 62, 241)
        self.PAN211_WriteReg(PAN211_PAGE0, 9, 3)
        self.PAN211_WriteReg(PAN211_PAGE0, 10, 3)
        self.PAN211_WriteReg(PAN211_PAGE0, 57, 85)
        if EN_AGC:
            self.PAN211_WriteReg(PAN211_PAGE0, 67, 58)
            self.PAN211_WriteReg(PAN211_PAGE0, 85, 221)
            self.PAN211_WriteReg(PAN211_PAGE0, 86, 201)
            self.PAN211_WriteReg(PAN211_PAGE0, 87, 183)
            self.PAN211_WriteReg(PAN211_PAGE0, 90, 16)
            self.PAN211_WriteReg(PAN211_PAGE0, 91, 253)
            self.PAN211_WriteReg(PAN211_PAGE0, 92, 233)
            self.PAN211_WriteReg(PAN211_PAGE0, 93, 220)
            self.PAN211_WriteReg(PAN211_PAGE0, 94, 2)
            self.PAN211_WriteReg(PAN211_PAGE0, 95, 6)
            self.PAN211_WriteReg(PAN211_PAGE0, 96, 14)
            self.PAN211_WriteReg(PAN211_PAGE0, 97, 46)
            self.PAN211_WriteReg(PAN211_PAGE0, 102, 52)
            self.PAN211_WriteReg(PAN211_PAGE0, 104, 13)
            self.PAN211_WriteReg(PAN211_PAGE0, 110, 32)
        else:
            self.PAN211_WriteReg(PAN211_PAGE0, 78, 126)
            self.PAN211_WriteReg(PAN211_PAGE0, 87, 221)
            self.PAN211_WriteReg(PAN211_PAGE0, 90, 205)
            self.PAN211_WriteReg(PAN211_PAGE0, 91, 205)
            self.PAN211_WriteReg(PAN211_PAGE0, 92, 205)
            self.PAN211_WriteReg(PAN211_PAGE0, 97, 46)

    
    def PAN211_SetTxDemodType(self, TxDevSelect):
        if TxDevSelect == PAN211Attributes.XN297_1Mbps_TxDev_250K:
            PAN211Attributes.TxDemodIndex[0][2] = 3
        elif TxDevSelect == PAN211Attributes.XN297_1Mbps_TxDev_300K:
            PAN211Attributes.TxDemodIndex[0][2] = 0

    
    def SetupConfig(self = None, config = None):
        self.PAN211_WriteRecommendedRegs(config.EN_AGC)
        self.PAN211_SetPredefinedRegs(config.INTERFACE_MODE, config.XTAL_FREQ)
        self.PAN211_WriteRegBits(PAN211_PAGE1, 76, 0, BIT4)
        self.PAN211_SetTxDemodType(config.TxDevSelect)
        self.PAN211_SetChipMode(config.ChipMode, config.Endian, config.crcSkipAddr)
        self.PAN211_SetChannel(config.Channel)
        self.PAN211_SetDataRate(config.ChipMode, config.DataRate, 0)
        self.PAN211_SetTxPayloadLen(config.TxLen)
        self.PAN211_SetRxPayloadLen(config.RxLen)
        self.PAN211_RxLengthLimit(config.EnRxPlLenLimit)
        self.PAN211_SetTxPower(config.ChipMode, config.DataRate, config.TxPower)
        self.PAN211_SetTxMode(config.TxMode)
        self.PAN211_SetRxMode(config.RxMode)
        if config.ChipMode != PAN211Attributes.PAN211_CHIPMODE_BLE:
            self.PAN211_EnableDynamicPL(config.EnDPL)
            self.PAN211_EnableWhiten(config.EnWhite)
            self.PAN211_SetCrcScheme(config.Crc)
            self.PAN211_SetWorkMode(config.WorkMode)
            self.PAN211_SetAddrWidth(config.AddrWidth)
            self.PAN211_SetTxAddr(config.TxAddr, config.AddrWidth)
            for i in range(6):
                if config.RxAddr[i][0]:
                    self.PAN211_EnableRxPipe(i)
                    self.PAN211_SetRxAddr(i, config.RxAddr[i][1], config.AddrWidth)
                else:
                    self.PAN211_DisableRxPipe(i)
        else:
            self.PAN211_EnableDynamicPL(True)
            self.PAN211_EnableWhiten(True)
            self.PAN211_SetCrcScheme(PAN211Attributes.PAN211_CRC_3byte)
            self.PAN211_SetWorkMode(PAN211Attributes.PAN211_WORKMODE_NORMAL)
            self.PAN211_SetAddrWidth(PAN211Attributes.PAN211_WIDTH_4BYTES)
            self.PAN211_SetTxAddr(config.TxAddr, 4)
            for i in range(6):
                if config.RxAddr[i][0]:
                    self.PAN211_EnableRxPipe(i)
                    self.PAN211_SetRxAddr(i, config.RxAddr[i][1], 4)
                else:
                    self.PAN211_DisableRxPipe(i)
            self.PAN211_SetS2S8Mode(config.S2S8Mode)
            self.PAN211_WriteNordicPktHeader(config.BLEHead0, config.BLEHead1, config.TxLen)
            if config.BLEHeadNum == 0:
                self.PAN211_SetNordicPktHeader(False, 0)
            else:
                self.PAN211_SetNordicPktHeader(True, config.BLEHeadNum)
            self.PAN211_SetBleWLMatchMode(config.WhiteListMatchMode)
            self.PAN211_SetBleWhitelist(config.WhiteListOffset, config.WhiteList, config.WhiteListLen)
            self.PAN211_SetBleLenFilter(config.LengthFilterMode)
            self.PAN211_SetWhiteInitVal(config.WhiteInit)
        self.PAN211_EnableTxNoAck(config.EnTxNoAck)
        if config.EnTxNoAck:
            self.PAN211_EnableFifo128bytes(True)
            self.PAN211_SetWaitAckTimeout(config.RxTimeoutUs)
            if config.RxTimeoutUs == 0:
                raise ValueError('RxTimeoutUs must not be 0 if EnTxNoAck is 1')
            None.PAN211_SetAutoRetrans(0, 0)
        else:
            self.PAN211_EnableFifo128bytes(False)
            self.PAN211_SetTRxTransTime(config.TRxDelayTimeUs)
            self.PAN211_SetWaitAckTimeout(config.RxTimeoutUs)
            self.PAN211_SetAutoRetrans(config.AutoDelayUs, config.AutoMaxCnt)
        self.PAN211_EnableInterfaceMuxIRQ(config.IOMUX_EN, config.INTERFACE_MODE)
        self.PAN211_ConfigIT(config.InterruptMask)
        self.PAN211_SetXTALFreq(config.XTAL_FREQ, config.EN_AGC, config.DataRate)
        self.PAN211_SetRxGain(config.RxGain, config.EN_AGC)
        return True

    
    def getModifiedRegisters(self):
        page0_table = []
        for i in range(len(self.RecommendRegPage0)):
            if self.RecommendRegPage0[i][1] != self.DefaultRegPage0[i][1]:
                page0_table.append([
                    self.RecommendRegPage0[i][0],
                    self.PAN211_ReadReg(PAN211_PAGE0, self.RecommendRegPage0[i][0])])
                continue
                page1_table = []
                for i in range(len(self.RecommendRegPage1)):
                    if self.RecommendRegPage1[i][1] != self.DefaultRegPage1[i][1]:
                        page1_table.append([
                            self.RecommendRegPage1[i][0],
                            self.PAN211_ReadReg(PAN211_PAGE1, self.RecommendRegPage1[i][0])])
                        continue
                        return (page0_table, page1_table)

    
    def modifyDefaultRegisters(self, page, addr, value):
        if page == PAN211_PAGE0:
            for i in range(len(self.DefaultRegPage0)):
                if self.DefaultRegPage0[i][0] == addr:
                    self.DefaultRegPage0[i][1] = value
                
        if page == PAN211_PAGE1:
            for i in range(len(self.DefaultRegPage1)):
                if self.DefaultRegPage1[i][0] == addr:
                    self.DefaultRegPage1[i][1] = value
                
                return None

    
    def getRegistersbyAddress(self, page0_addr_table, page1_addr_table):
        page0_table = []
        page1_table = []
        for i in range(len(self.RecommendRegPage0)):
            if self.RecommendRegPage0[i][0] in page0_addr_table:
                page0_table.append([
                    self.RecommendRegPage0[i][0],
                    self.PAN211_ReadReg(PAN211_PAGE0, self.RecommendRegPage0[i][0])])
                continue
                for i in range(len(self.RecommendRegPage1)):
                    if self.RecommendRegPage1[i][0] in page1_addr_table:
                        page1_table.append([
                            self.RecommendRegPage1[i][0],
                            self.PAN211_ReadReg(PAN211_PAGE1, self.RecommendRegPage1[i][0])])
                        continue
                        return (page0_table, page1_table)


if __name__ == '__main__':
    config = PAN211MPCONFIG()
    res = PAN211xGENCONFIG(DefaultRegPage0, DefaultRegPage1)
    res.SetupConfig(config)
    (page0_table, page1_table) = res.getModifiedRegisters()
    print('Modified Page 0 Registers:')
    for reg in page0_table:
        print(f'''Register: 0x{reg[0]:02X}, Value: 0x{reg[1]:02X}''')
    print('\nModified Page 1 Registers:')
    for reg in page1_table:
        print(f'''Register: 0x{reg[0]:02X}, Value: 0x{reg[1]:02X}''')
