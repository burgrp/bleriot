# Source Generated with Decompyle++
# File: main.pyc (Python 3.8)

import copy
import json
import re
import sys
import threading
import time
import webbrowser
from datetime import datetime
from functools import partial
from pathlib import Path
import serial
from PyQt5 import QtCore
from PyQt5.QtCore import Qt, QRegExp, pyqtSignal, QEvent, QRegularExpression
from PyQt5.QtGui import QIcon, QRegExpValidator, QPixmap, QFont, QStandardItemModel, QStandardItem, QDesktopServices, QRegularExpressionValidator
from PyQt5.QtWidgets import QApplication, QMainWindow, QAbstractItemView, QHeaderView, QFileDialog, QMessageBox, QLabel, QDialog, QActionGroup, QAction
from infi.devicemanager import DeviceManager
import gol
from PanchipDfuTool.dfu.dfu_program import DfuProgram
from PanchipDfuTool.dfu.dfu_usb_com import DfuUsbCom
from PanchipDfuTool.usb_hid.usb_hid_dev_control import HidDeviceControl
from bus.PanBus import *
from config.home_table import *
from config.style_sheet import scroll_vertical_style
from confirmDialog.ConfirmDialog import ConfirmDialog
from dlg.PasswordDialog import PasswordDialog
from dlg.WhatsNewDialog import WhatsNewDialog
from dlg.WhatsNewDialog216 import WhatsNewDialog216
from dlg.code_preview_dialog import CodeViewDialog
from dlg.rf_test_dialog import RfTestDialog
from fileUtils import unzip_file, delete_folder, rename_folder, get_file_from_datas, copy_file, replace_file_content, update_txbuf_array, replace_file_next_line_content, replace_file_content_arr, copy_file_from_stream, update_ble_txbuf_array
from protol.pan_atcmd import PanATClass
from ui.ui_main import Ui_MainWindow
from update.sw_updater import SW_Updater
from utils.FilesUtil import FilesUtil
from utils.LogUtils import LogUtils, DEBUG_FLAG
from utils.TextUtils import split_by_two, ascii_to_hex, check_arry_str_change, hex_to_int
from utils.convert_pan211_to_pms import gen_pms_pan211
from utils.generate_pan211_c import gen_pan211_c
from utils.keil_arm import get_all_keil_path, build_keil_project
from utils.layout_utils import *
from version.version import Version
from widgets.ClickableQLabel import ClickableQLabel
from widgets.HexValidator import HexValidator
from widgets.SaveableLabel import SaveableLabel
from widgets.WheelClickDetector import WheelClickDetector
from widgets.frame_struct import FrameStructureView
TIME_SEQ_MAX_HEIGHT = 180
MSG_SUCCESS = 0
MSG_FAIL = 1
MSG_NORMAL = 2

class MyWindow(Ui_MainWindow, QMainWindow):
    message_info_signal = pyqtSignal(str, str)
    update_dlg_view = pyqtSignal(str, str)
    version_updated = pyqtSignal(str)
    time_seq_signal = pyqtSignal()
    ui_init_signal = pyqtSignal()
    doubleMiddleClick = pyqtSignal()
    confirm_dialog_signal = pyqtSignal(int)
    dl_program_btn_signal = pyqtSignal(int)
    action_rf_test_signal = pyqtSignal(bool)
    dfu_dev_status_signal = pyqtSignal(bool)
    dfu_progress_signal = pyqtSignal(int)
    dfu_progress_msg_signal = pyqtSignal(str, int)
    hid_device_action_signal = pyqtSignal()
    hid_device_name_signal = pyqtSignal()
    rf_test_dialog_signal = pyqtSignal(bytes, str, str)
    config_list_disabled_signal = pyqtSignal()
    check_rx_addr_signal = pyqtSignal()
    ble_channel_signal = pyqtSignal(int)
    
    def __init__(self = None, parent = None):
        super(MyWindow, self).__init__(parent)
        self.setAttribute(Qt.WA_StaticContents)
        self.setupUi(self)
        self.init_args()
        self.init_signal()
        print('chip_mode_list:{}'.format(json.dumps(chip_mode_list)))
        self.start_ui_thread()
        self.detector = WheelClickDetector()
        self.installEventFilter(self)
        self.doubleMiddleClick.connect(self.on_double_middle_click)

    
    def center_window(self):
        '''将窗口居中到主屏幕'''
        screen = QApplication.primaryScreen().availableGeometry()
        size = self.size()
        self.move(screen.center().x() - size.width() // 2, screen.center().y() - size.height() // 2)

    
    def eventFilter(self = None, obj = None, event = None):
        if event.type() == event.MouseButtonDblClick and event.button() == Qt.MiddleButton:
            self.doubleMiddleClick.emit()
        return super().eventFilter(obj, event)

    
    def resizeEvent(self, a0):
        self.windows_width = a0.size().width()
        self.time_seq_signal.emit()

    
    def changeEvent(self, event):
        if event.type() == QtCore.QEvent.ActivationChange:
            if self.isActiveWindow():
                self.is_suspend = False
            else:
                self.is_suspend = True

    
    def closeEvent(self, a0):
        self.is_windows_closed = True

    
    def on_double_middle_click(self):
        print('滚轮双击触发！')
        dialog = PasswordDialog()
        dialog.set_correct_passsword('123456')
        if dialog.exec_() == QDialog.Accepted:
            print('弹窗已确认')
            self.password_checked = True
            self.showOtherLayoutParams(True)
            self.showHideTxDeviation()
            if not self.curr_config['custome'] and self.is_pms:
                self.btn_start_dl_program.show()
            self.show_dev_dfu_progress(True)

    
    def showOtherLayoutParams(self, is_show):
        other_layout_params = [
            self.optmize_0dbm_label,
            self.optmize_0dbm_cb,
            self.tx_deviation_label,
            self.tx_deviation_cb,
            self.en_agc_label,
            self.en_agc_cb,
            self.en_11dbm_label,
            self.en_11dbm_cb]
        for item in other_layout_params:
            item.show() if is_show else item.hide()

    
    def start_ui_thread(self):
        t = threading.Thread(self.send_ui_init_sig, True, **('target', 'daemon'))
        t.start()

    
    def preload_ui(self):
        self.codeViewDialog = CodeViewDialog(None, self.on_base_freq_changed)
        UpdateDlg = UpdateDlg
        import update.dfu_progress_dlg
        self.update_dlg = UpdateDlg()
        self.update_dlg_view.connect(self.setUpdateDlgView)
        self.whats_new_dialog = WhatsNewDialog()
        self.whats_new_dialog_216 = WhatsNewDialog216()
        KeilSelectDialog = KeilSelectDialog
        import dlg.KeilSelectDialog
        self.keil_select_dialog = KeilSelectDialog()
        self.confirmDialog = None

    
    def send_ui_init_sig(self):
        self.ui_init_signal.emit()

    
    def init_ui(self):
        self.initTimeSeqTableWidget()
        self.init_widgets()
        self.init_config_list()
        self.reload_tx_mode_cb()
        self.reload_rx_mode_cb()
        self.preload_ui()
        self.updater = SW_Updater(self, True)
        self.start_df_check_thread()
        self.center_window()

    
    def start_df_check_thread(self):
        t = threading.Thread(self.dfu_check, True, **('target', 'daemon'))
        t.start()

    
    def dfu_check(self):
        is_first = True
        if self.is_windows_closed:
            pass
    # WARNING: Decompyle incomplete

    
    def init_config_list(self):
        for index, config in enumerate(self.client_list):
            text = ClickableQLabel()
            name = config['name']
            if not STATE_211_MODE:
                name = name.replace(' XN297L', '')
            text.setText('{}'.format(name))
            text.setStyleSheet('QLabel{font-size:18px}')
            text.setMargin(0)
            text.setContentsMargins(10, 3, 0, 3)
            text.setScaledContents(True)
            text.clicked.connect(partial(self.on_selection_changed, index))
            self.config_list_vl.addWidget(text)
        self.config_list_vl.setSpacing(0)
        self.scrollArea.setStyleSheet(scroll_vertical_style)
        self.on_selection_changed(0)

    
    def disable_configs_by_pms(self):
        child_cont = self.config_list_vl.count()
        for i in range(child_cont):
            w = self.config_list_vl.itemAt(i)
            label = w.widget()
            if isinstance(label, QLabel) or self.is_pms:
                if not 'Normal' in label.text():
                    pass
                is_enable = 'Carrier' in label.text()
                label.setDisabled(not is_enable)
                continue
            label.setDisabled(False)

    
    def on_base_freq_changed(self, base_freq):
        self.base_freq = base_freq
        self.base_freq_cb.setCurrentText(base_freq)
        (content, req_content) = self.parse_easy_rf_code()
        self.codeViewDialog.set_content('c', content, self.is_pms, self.base_freq)

    
    def get_time_img(self):
        if self.enhance_cb.currentIndex() == 0:
            if self.trx_mode:
                return 'img_tx.png'
            return None
        if None.trx_mode:
            return 'img_tx2rx.png'
        return None

    
    def show_time_seq_img(self):
        if not self.windows_width:
            return None
        group_height = None.time_seq_label_group_layout.height() * 0.8
        clear_Layout(self.time_seq_label_layout)
        self.time_seq_label_layout.addSpacing(0)
        img_name = self.get_time_img()
        ix = QPixmap(':/images/{}'.format(img_name))
        time_seq_label = SaveableLabel()
        time_seq_label.setPixmap(ix)
        wh_ratio = ix.width() / ix.height()
        hw_ratio = ix.height() / ix.width()
        max_height = group_height
        label_height = max_height
        label_width = int(label_height * wh_ratio)
        widget_width = self.windows_width * 0.8
        if label_width > widget_width:
            label_width = widget_width
            label_height = label_width * hw_ratio
        if label_height >= max_height:
            label_height = max_height
            label_width = label_height * wh_ratio
        if not isinstance(label_width, int):
            label_width = int(label_width)
        if not isinstance(label_height, int):
            label_height = int(label_height)
        time_seq_label.setMaximumWidth(label_width)
        time_seq_label.setMinimumWidth(label_width)
        time_seq_label.setMinimumHeight(label_height)
        time_seq_label.setMaximumHeight(label_height)
        time_seq_label.setMargin(0)
        time_seq_label.setScaledContents(True)
        self.time_seq_label_layout.addWidget(time_seq_label)
        self.time_seq_label_layout.addSpacing(0)

    
    def setUpdateDlgView(self, title, meg):
        self.update_dlg.show(title, meg)

    
    def set_config_list_disabled(self):
        for cnt in range(self.config_list_vl.count()):
            child = self.config_list_vl.itemAt(cnt).widget()
            child.setDisabled(self.is_dfu_downloading)

    
    def init_args(self):
        self.hid_select_uuid = None
        self.hid_dev_list = None
        self.sel_dev_path = None
        self.is_progress_running = False
        self.rf_dialog = None
        self.frame_view = None
        self.time_seq_list = None
        self.windows_width = 0
        self.trx_mode = 0
        self.password_checked = False
        self.curr_config = None
        self.is_serial_searching = False
        self.is_dfu_downloading = False
        self.is_rebooting = False
        self.is_longrange = False
        self.is_loading_params = False
        self.rf_dialog_dict = { }
        self.is_suspend = False
        self.is_windows_closed = False
        self.is_pms = False
        self.base_freq = None
        self.is_oled = None
        self.client_list = list(filter((lambda x: x['chipMode'] in (0, 3)), table_config_list_simple)) if STATE_211_MODE else table_config_list_simple
    # WARNING: Decompyle incomplete

    
    def init_tx_power_cb(self):
        self.power_spin.clear()
        self.power_table_widget.clear()
        for i in range(-40, 12):
            if i in (10, -3, -4, -6, -9, -13, -15, -17, -18, -20, -21, -22, -24, -26, -27, -29, -30, -31, -32, -34, -35, -36, -38, -39):
                continue
            if i == 11 and self.en_11dbm_cb.currentIndex() == 0:
                continue
            self.power_spin.addItem('{}dBm'.format(i))
            self.power_table_widget.addItem('{}dBm'.format(i))
            total_count = self.power_table_widget.count()
            self.power_table_widget.setCheckedItems([
                17,
                26])
        if self.curr_config:
            self.power_table_widget.setCheckedItems(self.curr_config.get('power_table', [
                17,
                26]))
            self.power_spin.setCurrentIndex(self.curr_config.get('txPowerIndex', 17))
        else:
            self.power_table_widget.setCheckedItems([
                17,
                26])
            self.power_spin.setCurrentIndex(17)

    
    def init_auto_delay_sp(self):
        regex = QRegExp('^([25][0]?|[5][0]{2}|[7-9][0-9]{2}|[1-9][0-9]{3}|...)0$')
        validator = QRegExpValidator(regex)
        self.auto_delay_sp.lineEdit().setValidator(validator)

    
    def init_sdk_ver(self):
        if not STATE_211_MODE:
            pass

    
    def init_interface(self):
        if not STATE_211_MODE:
            self.cb_interface.addItem('4-SPI')

    
    def on_cb_io_change(self):
        pass

    
    def init_pack_interrupt(self):
        self.cb_io_enable.setCurrentIndex(1)
        self.interrupt_btns = [
            self.mcb_interrupt1,
            self.mcb_interrupt2,
            self.mcb_interrupt3,
            self.mcb_interrupt4,
            self.mcb_interrupt5,
            self.mcb_interrupt6,
            self.mcb_interrupt7,
            self.mcb_interrupt8]
        for interrupt_btn in self.interrupt_btns:
            interrupt_btn.set_active(True)
            interrupt_btn.click_connect(self.click_pack_interrupt)

    
    def get_selected_interrupt_val(self):
        interrupt_val = 0
        rev_interrupt_btns = self.interrupt_btns.copy()
        for index, interrupt_btn in enumerate(rev_interrupt_btns):
            if interrupt_btn.is_active:
                interrupt_val |= 1 << index
                continue
                return get_interrupt_arr(interrupt_val)

    
    def click_pack_interrupt(self, w):
        w.switch_active()

    
    def getHexEditValid(self, l):
        rxp_str = '^[a-fA-F0-9]{0,10}$'
        if l == 12:
            rxp_str = '^[a-fA-F0-9]{0,12}$'
        elif l == 10:
            rxp_str = '^[a-fA-F0-9]{0,10}$'
        elif l == 8:
            rxp_str = '^[a-fA-F0-9]{0,8}$'
        elif l == 6:
            rxp_str = '[^[a-fA-F0-9]{0,6}$'
        elif l == 4:
            rxp_str = '^[a-fA-F0-9]{0,4}$'
        return QRegularExpressionValidator(QRegularExpression(rxp_str), self)

    
    def on_focus_out_event(self, key_name):
        if 'rxAddress' == key_name:
            l = self.tx_addr_width_sp.value() * 2
            if l > len(self.rx_addr_et.text()):
                t = self.rx_addr_et.text() + 'C' * (l - len(self.rx_addr_et.text()))
            else:
                t = self.rx_addr_et.text()[0:l]
            self.rx_addr_et.setText(t.upper())
            rxp_str = '[a-fA-F0-9]{{{}}}'.format(l)
            self.rx_addr_et.setValidator(QRegExpValidator(QRegExp(rxp_str), self))
        elif 'txAddress' == key_name:
            l = self.tx_addr_width_sp.value() * 2
            if l > len(self.tx_addr_et.text()):
                t = self.tx_addr_et.text() + 'C' * (l - len(self.tx_addr_et.text()))
            else:
                t = self.tx_addr_et.text()[0:l]
            self.tx_addr_et.setText(t.upper())
            rxp_str = '[a-fA-F0-9]{{{}}}'.format(l)
            self.tx_addr_et.setValidator(QRegExpValidator(QRegExp(rxp_str), self))
        elif 'whiteList' == key_name:
            l = self.white_list_match_mode_cb.currentIndex() * 2
            if l > len(self.adva_white_list_et.text()):
                t = self.adva_white_list_et.text() + 'C' * (l - len(self.adva_white_list_et.text()))
            else:
                t = self.adva_white_list_et.text()[0:l]
            self.adva_white_list_et.setText(t)
            rxp_str = '[a-fA-F0-9]{{{}}}'.format(l)
            self.adva_white_list_et.setValidator(QRegExpValidator(QRegExp(rxp_str), self))
        elif 'ble_adva' == key_name:
            l = 12
            if l > len(self.adva_et.text()):
                t = self.adva_et.text() + 'C' * (l - len(self.adva_et.text()))
            else:
                t = self.adva_et.text()[0:l]
            self.adva_et.setText(t)

    
    def init_trx_addr_form_view(self):
        addr_width = self.tx_addr_width_sp.value()
        hexEditValid = self.getHexEditValid(addr_width * 2)
        self.rx_addr_et.setAlignment(Qt.AlignLeft)
        self.tx_addr_et.setAlignment(Qt.AlignLeft)
        self.on_focus_out_event('txAddress')
        self.on_focus_out_event('rxAddress')

    
    def init_ble_form_view(self):
        self.adva_white_list_et.setText('CCCCCCCCCCCC')
        self.adva_white_list_et.setValidator(QRegExpValidator(QRegExp('[a-fA-F0-9]{12}'), self))
        self.adva_white_list_et.setAlignment(Qt.AlignLeft)
        self.ble_advd_groupbox.hide()
        self.adva_et.setAlignment(Qt.AlignLeft)
        self.adva_et.set_key('ble_adva', self.on_focus_out_event, **('on_focus_out_event',))
        self.adva_et.setValidator(QRegExpValidator(QRegExp('[a-fA-F0-9]{12}'), self))
        self.btn_expand_ble_payload.setIcon(QIcon(':/images/ic_expand.png'))

    
    def showHideAdvdGroupView(self):
        if self.ble_advd_groupbox.isHidden():
            self.btn_expand_ble_payload.setIcon(QIcon(':/images/ic_shrink.png'))
            self.ble_advd_groupbox.show()
        else:
            self.btn_expand_ble_payload.setIcon(QIcon(':/images/ic_expand.png'))
            self.ble_advd_groupbox.hide()
        self.reset_windows_size()

    
    def resetAdvaGroupView(self, need_adjust_window = (False,)):
        self.btn_expand_ble_payload.setIcon(QIcon(':/images/ic_expand.png'))
        self.ble_advd_groupbox.hide()
        if need_adjust_window:
            self.reset_windows_size()

    
    def reset_windows_size(self):
        ble_add_shown = not self.ble_advd_groupbox.isHidden()
        self.setMinimumSize(1000, 680) if not ble_add_shown else self.setMinimumSize(1000, 740)
        self.setMaximumSize(1000, 680) if not ble_add_shown else self.setMaximumSize(1000, 740)
        self.adjustSize()

    
    def hide_widget(self, w):
        layout = w.layout()
        layout.removeWidget(w)

    
    def init_tab_widget(self):
        self.tabWidget.removeTab(3)

    
    def init_widgets(self):
        self.init_sdk_ver()
        self.init_tx_power_cb()
        self.init_auto_delay_sp()
        self.init_interface()
        self.init_pack_interrupt()
        self.showOtherLayoutParams(False)
        self.show_warning_tips()
        self.init_enhance_form()
        self.init_ble_form_view()
        self.init_tab_widget()
        self.init_trx_addr_form_view()
        self.interrupt_label1.setPixmap(QPixmap(':/images/tag_unactive.png'))
        self.interrupt_label2.setPixmap(QPixmap(':/images/tag_active.png'))
        self.interrupt_label1.setScaledContents(True)
        self.interrupt_label2.setScaledContents(True)
        self.interrupt_label1.setMaximumSize(65, 25)
        self.interrupt_label2.setMaximumSize(65, 25)
        self.ble_name_et.setValidator(QRegExpValidator(QRegExp('[A-Za-z0-9]+'), self))
        self.config_desc_tw.setOpenLinks(False)
        self.config_desc_tw.anchorClicked.connect(self.open_external_browser)
        self.clk_label.hide()
        self.spi_clk_cb.hide()
        self.xtal_freq_cb.setCurrentIndex(1)
        self.actionDownload_Oled_Firmware.deleteLater()
        if not STATE_211_MODE:
            self.export_mode_cb.hide()
            self.export_mode_label.hide()
        self.base_freq_cb.hide()
        self.base_freq_label.hide()
        self.btn_start_dl_oled_program.show() if DEBUG_FLAG else self.btn_start_dl_oled_program.hide()
        self.rx_gain_cb.setCurrentIndex(0)
        self.en_agc_cb.setCurrentIndex(1)
        self.config_desc_tw.setStyleSheet('QTextBrowser{background-color:#dddddd}')
        group_box_style = 'QGroupBox { font-weight: bold;font-size:12.5px;color:#343434 }'
        all_group_boxes = [
            self.groupBox_8,
            self.groupBox_12,
            self.groupBox_2,
            self.groupBox,
            self.frame_format_groupbox,
            self.ble_advd_groupbox,
            self.groupBox_4,
            self.groupBox_7,
            self.time_seq_label_group_layout]
        self.xtal_freq_cb.setCurrentIndex(1)
        self.tx_deviation_cb.setCurrentIndex(1)
        for g in all_group_boxes:
            g.setStyleSheet(group_box_style)
        self.tabWidget.setStyleSheet('\n            #tabWidget QTabBar::tab {\n                font-weight: bold;\n                min-width: 165px;\n                height: 25px;\n                font-size: 12px;\n            }\n            #tabWidget QTabBar::tab:selected {\n                background-color: #b4c8e9;\n                color: #343434;\n            }\n            #tabWidget QTabBar::tab:hover {\n                background-color: lightgray;\n            }\n        ')
        self.pipex_addr_ck_list = [
            self.multi_pipe_ck1,
            self.multi_pipe_ck2,
            self.multi_pipe_ck3,
            self.multi_pipe_ck4,
            self.multi_pipe_ck5]
        self.pipex_addr0_et_list = [
            self.pipe1_addr0,
            self.pipe2_addr0,
            self.pipe3_addr0,
            self.pipe4_addr0,
            self.pipe5_addr0]
        self.pipex_addr0_lb_list = [
            self.pipe1_addr0_lb,
            self.pipe2_addr0_lb,
            self.pipe3_addr0_lb,
            self.pipe4_addr0_lb,
            self.pipe5_addr0_lb]
        multi_pipe_addr_widgets = [
            self.pipex_addr4,
            self.pipex_addr3,
            self.pipex_addr2,
            self.pipex_addr1,
            self.pipe1_addr0,
            self.pipe2_addr0,
            self.pipe3_addr0,
            self.pipe4_addr0,
            self.pipe5_addr0]
        for p in self.pipex_addr0_et_list:
            p.setValidator(QRegExpValidator(QRegExp('[a-fA-F0-9]{2}'), self))
        for p in multi_pipe_addr_widgets:
            p.setValidator(QRegExpValidator(QRegExp('[a-fA-F0-9]{2}'), self))
        self.chip_mode_cb.clear()
        self.chip_mode_cb.addItems(chip_mode_list_simple[0:-1]) if STATE_211_MODE else self.chip_mode_cb.addItems(chip_mode_list[0:-1])
        self.dfu_progress_msg_signal.emit('', MSG_SUCCESS)
        self.dfu_progress_signal.emit(0)
        self.btn_start_dl_program.hide()
        self.show_dev_dfu_progress(False)
        self.menuFirmware.setVisible(False)

    
    def show_dev_dfu_progress(self, is_show):
        self.dfu_dev_status_pb.show() if is_show or self.password_checked else self.dfu_dev_status_pb.hide()
        self.line_4.show() if is_show else self.line_4.hide()

    
    def initTimeSeqTableWidget(self):
        self.time_seq_table.horizontalHeader().setSectionResizeMode(QHeaderView.ResizeMode.Stretch)
        self.time_seq_table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.Fixed)
        self.time_seq_table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.Fixed)
        self.time_seq_table.horizontalHeader().setSectionResizeMode(3, QHeaderView.ResizeMode.Fixed)
        self.time_seq_table.setColumnWidth(0, 100)
        self.time_seq_table.setColumnWidth(1, 200)
        self.time_seq_table.setColumnWidth(3, 200)
        self.time_seq_table.setEditTriggers(QAbstractItemView.NoEditTriggers)
        self.time_seq_table.setSelectionBehavior(QAbstractItemView.SelectColumns)
        self.time_seq_table.setSelectionMode(QAbstractItemView.NoSelection)
        self.time_seq_table.setWordWrap(True)
        self.time_seq_table.setShowGrid(True)
        self.time_seq_table.resizeRowsToContents()
        self.setTimeSeqTableDataList()

    
    def check_if_ble_mode(self):
        if self.get_current_chip_mode() == 3:
            pass
        return not (self.is_longrange)

    
    def get_air_trans_us(self):
        work_mode = self.enhance_cb.currentIndex()
        preamble_length = self.get_preamble_length()
        address_length = self.tx_addr_width_sp.value()
        tx_len = self.tx_len_sp.value()
        crc_len = self.crc_cb.currentIndex()
        daterate = self.get_datarate_hz()
        s2s8_mode = self.s2s8_cb.currentIndex()
        tips = [
            '普通型<html>T<sub>OA</sub></html>计算公式： (preamble length(byte)+ address length(byte) + payload length(byte) + CRC length(byte)) * 8(bit/byte) / datarate(bit/us); ',
            '增强型<html>T<sub>OA</sub></html>计算公式： (preamble length(byte)+ address length(byte) + packet control field length(bit) + payload length(byte) + CRC length(byte)) * 8(bit/byte) / datarate(bit/us); ',
            'S0模式下<html>T<sub>OA</sub></html>计算公式：(preamble length(byte)+  address length(byte) + BLE head num(byte) +  payload length(byte) +  CRC length(byte)) * 8(bit/byte) / datarate(bit/us) ；',
            'S2模式下<html>T<sub>OA</sub></html>计算公式：((preamble length(byte)+ 8 * address length(byte) + CI(2bytes)+ TERM1(3bytes) + 2 * BLE head num(byte) + 2 * payload length(byte) + 2 * CRC length(byte)) * 8(bit/byte) + 2* term2(3bit))/ datarate(bit/us) ；',
            'S8模式下<html>T<sub>OA</sub></html>计算公式：((preamble length(byte)+ 8 * address length(byte) + CI(2bytes)+ TERM1(3bytes) + 8 * BLE head num(byte) + 8 * payload length(byte) + 8 * CRC length(byte)) * 8(bit/byte) + 8* term2(3bit)) / datarate(bit/us) ']
        (air_trans_us, air_trans_tips) = (0, '')
        preamble = 1
        signal = 10
        tx_addr = self.tx_addr_width_sp.value()
        tx_len = self.tx_len_sp.value()
        crc = self.crc_cb.currentIndex()
        enhance = self.enhance_mode_enable()
        datarate = self.daterate_cb.currentIndex()
        s2s8_mode = self.s2s8_cb.currentIndex()
        chip_mode = self.chip_mode_cb.currentText()
        if not chip_mode is not BLE_CHIP_MODE_NAME and self.is_longrange:
            if chip_mode == PN297_CHIP_MODE_NAME:
                preamble = 3
                signal = 10
            elif chip_mode == FS01_CHIP_MODE_NAME:
                preamble = 1
                signal = 9
            elif chip_mode == FS32_CHIP_MODE_NAME:
                preamble = 1
                signal = 11
            if not enhance:
                t_250K = (preamble + tx_addr + tx_len + crc) * 8 * 4
                if datarate == 0:
                    t1_time = t_250K / 4
                elif datarate == 1:
                    t1_time = t_250K / 8
                elif datarate == 2:
                    t1_time = t_250K
                else:
                    t_250K = (preamble + tx_addr + tx_len + crc) * 8 * 4 + signal * 4
                    if self.daterate_cb.currentIndex() == 0:
                        t1_time = t_250K / 4
                    elif self.daterate_cb.currentIndex() == 1:
                        t1_time = t_250K / 8
                    elif datarate == 2:
                        t1_time = t_250K
                    else:
                        preamble = 1
                        header = 2
                        if s2s8_mode == 0:
                            t_250K = (preamble + tx_addr + header + tx_len + crc) * 8 * 4
                            if datarate == 0:
                                t1_time = t_250K / 4
                            elif datarate == 1:
                                t1_time = (preamble * 2 + tx_addr + header + tx_len + crc) * 4
                            elif datarate == 2:
                                t1_time = t_250K
                            elif s2s8_mode == 1:
                                t_250K = 4 * (preamble * 80 + tx_addr * 8 * 8 + 16 + 24 + (header + tx_len + crc) * 8 * 2 + 6)
                                if datarate == 0:
                                    t1_time = t_250K / 4
                                elif datarate == 2:
                                    t1_time = t_250K
                                elif s2s8_mode == 2:
                                    t_250K = 4 * (preamble * 80 + tx_addr * 8 * 8 + 16 + 24 + (header + tx_len + crc) * 8 * 8 + 24)
                                    if datarate == 0:
                                        t1_time = t_250K / 4
                                    elif datarate == 2:
                                        t1_time = t_250K
        t1_time = int(t1_time)
        return (t1_time, air_trans_tips)

    
    def get_time_seq_list(self):
        spiClkArr = [
            1,
            2,
            4,
            8,
            10]
        spiClk = spiClkArr[self.spi_clk_cb.currentIndex()] * 1000 * 1000
        if self.cb_interface.currentIndex() == 0:
            t1_time = int((((self.tx_len_sp.value() + 1) * 8 + 132) / spiClk) * 1 * 1000 * 1000 * 1.1)
        else:
            t1_time = int((((self.tx_len_sp.value() + 2) * 9 + 132) / spiClk) * 1 * 1000 * 1000 * 1.2)
        work_mode = self.enhance_cb.currentIndex()
        (air_trans_us, air_trans_tips) = self.get_air_trans_us()
        if work_mode:
            if self.trx_mode:
                self.time_seq_list = [
                    [
                        '<html>T<sub>UL</sub></html>',
                        'UPLOAD TIME',
                        'Time of writing data into the FIFO via SPI or I2C interface.',
                        '{}us'.format(t1_time),
                        '<html>T<sub>UL</sub></html>= 8 (bit) * payload length / SPI or I2C data rate(bit/s) '],
                    [
                        '<html>T<sub>TS</sub></html>',
                        'TX SETTLING TIME',
                        'Time to enable TX circuit.',
                        '73us'],
                    [
                        '<html>T<sub>OA</sub></html>',
                        'TIME ON AIR',
                        'Tx air transmission time.',
                        '{}us'.format(air_trans_us),
                        air_trans_tips],
                    [
                        '<html>T<sub>TE</sub></html>',
                        'TX EXIT TIME',
                        'Time from completion of TX transmission to generation of a TX interrupt.',
                        '{}us'.format(23)],
                    [
                        '<html>T<sub>TW</sub></html>',
                        'TRANS WAIT TIME',
                        'TRX switching delay time.',
                        '{}us'.format(self.trx_delay_time_sp.value()),
                        '<html>T<sub>TW</sub></html>= TRX_TRANS_WAIT_TIME'],
                    [
                        '<html>T<sub>RS</sub></html>',
                        'RX SETTLING TIME',
                        'Time to enable RX circuit',
                        '62us'],
                    [
                        '<html>T<sub>RE</sub></html>',
                        'RX EXIT TIME',
                        'Time to disable RX circuit',
                        '5us']]
            else:
                self.time_seq_list = [
                    [
                        '<html>T<sub>RS</sub></html>',
                        'RX SETTLING TIME',
                        'Time to enable RX circuit.',
                        '{}us'.format(64)],
                    [
                        '<html>T<sub>RE</sub></html>',
                        'RX EXIT TIME',
                        'time to Disable RX circuit.',
                        '2us'],
                    [
                        '<html>T<sub>TW</sub></html>',
                        'TRX TRANS WAIT TIME',
                        'TRX switching delay time.',
                        '{}us'.format(self.trx_delay_time_sp.value()),
                        '<html>T<sub>TW</sub></html>= TRX_TRANS_WAIT_TIME.'],
                    [
                        '<html>T<sub>TS</sub></html>',
                        'TX SETTLING TIME',
                        'Time to enable TX circuit.',
                        '{}us'.format(70)],
                    [
                        '<html>T<sub>UL</sub></html>',
                        'UPLOAD TIME',
                        '<html>Time of writing data into the FIFO via SPI or I2C interface.',
                        '{}us'.format(t1_time),
                        '增强型接收模式，如果需要收到数据之后再写FIFO，那么T<sub>RE</sub> + T<sub>TW</sub> + T<sub>TS</sub> 应当大于T<sub>UL</sub>，否则来不及写FIFO。增强型接收模式，为了保证能及时收到数据，发送端的T<sub>IRQ</sub> + T<sub>TW</sub> + T<sub>RS</sub> 应当大于接收端的T<sub>RE</sub> + T<sub>TW</sub> + T<sub>TS</sub>。<br />T<sub>UL</sub>= 8 (bit) * payload length / SPI or I2C data rate(bit/s) </html>'],
                    [
                        '<html>T<sub>OA</sub></html>',
                        'TIME ON AIR',
                        'Tx air transmission time.',
                        '{}us'.format(air_trans_us),
                        air_trans_tips],
                    [
                        '<html>T<sub>IRQ</sub></html>',
                        'IRQ TIME',
                        'Time from completion of TX transmission to generation of a TX interrupt.',
                        '23us'],
                    [
                        '<html>T<sub>TE</sub></html>',
                        'TX EXIT TIME',
                        'Time from completion of TX transmission to fully exiting the TX state.',
                        '26us']]
        elif self.trx_mode:
            self.time_seq_list = [
                [
                    '<html>T<sub>UL</sub></html>',
                    'UPLOAD TIME',
                    'Time of writing data into the FIFO via SPI or I2C interface.',
                    '{}us'.format(t1_time),
                    '<html>T<sub>UL</sub></html>= 8 (bit) * payload length / SPI or I2C data rate(bit/s) '],
                [
                    '<html>T<sub>TS</sub></html>',
                    'TX SETTLING TIME',
                    'Time to enable Tx circuit.',
                    '73us'],
                [
                    '<html>T<sub>OA</sub></html>',
                    'TIME ON AIR',
                    'Tx air transmission time.',
                    '{}us'.format(air_trans_us),
                    air_trans_tips],
                [
                    '<html>T<sub>IRQ</sub></html>',
                    'IRQ TIME',
                    'Time from completion of TX transmission to generation of a TX interrupt.',
                    '{}us'.format(23)],
                [
                    '<html>T<sub>TE</sub></html>',
                    'TX EXIT TIME',
                    'Time from completion of TX transmission to fully exiting the TX state.',
                    '26us']]
        else:
            self.time_seq_list = [
                [
                    '<html>T<sub>RS</sub></html>',
                    'RX SETTLING TIME',
                    'Time to enable RX circuit.',
                    '{}us'.format(64)],
                [
                    '<html>T<sub>RE</sub></html>',
                    'RX EXIT TIME',
                    'Time to disable RX circuit.',
                    '5us']]

    
    def setTimeSeqTableDataList(self):
        if self.time_seq_table.rowCount() > 0:
            self.time_seq_table.removeRow(self.time_seq_table.rowCount() - 1)
            continue
        self.get_time_seq_list()
        if not self.time_seq_list:
            return None
        for data in None.time_seq_list:
            row = self.time_seq_table.rowCount()
            self.time_seq_table.setRowCount(row + 1)
            for i in range(len(data)):
                data_value = data[i]
                qLa = QLabel()
                qLa.setText('     ' + data_value + '     ')
                qLa.setAlignment(Qt.AlignCenter)
                qLa.setWordWrap(True)
                qLa.setIndent(5)
                if i == 3:
                    qLa.setFont(QFont('黑体', 11, QFont.Bold))
                    qLa.setStyleSheet('color: #006400;')
                if i == 2 and len(data) >= 5:
                    w = QWidget()
                    ll = QHBoxLayout()
                    ll.addSpacing(0)
                    qLa.setAlignment(Qt.AlignCenter)
                    ll.addWidget(qLa)
                    help_btn = QLabel()
                    help_btn.setPixmap(QPixmap(':/images/ic_help.png'))
                    help_btn.setToolTip(data[4])
                    help_btn.setMaximumSize(15, 15)
                    help_btn.setMinimumSize(15, 15)
                    help_btn.setScaledContents(True)
                    ll.addWidget(help_btn, Qt.AlignVCenter, **('alignment',))
                    ll.addSpacing(0)
                    w.setLayout(ll)
                    self.time_seq_table.setCellWidget(row, i, w)
                    continue
                self.time_seq_table.setCellWidget(row, i, qLa)
        self.set_row_height(self.time_seq_table, 40)

    
    def open_external_browser(self, url):
        QDesktopServices.openUrl(url)

    
    def set_addr_irq_disabled(self):
        if not self.check_if_ble_mode():
            pass
        is_disabled = self.is_longrange
        self.mcb_interrupt3.set_active(not is_disabled)
        self.mcb_interrupt3.setDisabled(is_disabled)

    
    def on_selection_changed(self, select_row_index):
        self.is_loading_params = True
        self.curr_config = self.client_list[select_row_index]
        for i in range(self.config_list_vl.count()):
            if not self.config_list_vl.itemAt(i).widget():
                continue
            if i == select_row_index:
                self.config_list_vl.itemAt(i).widget().setStyleSheet('\n                    background-color:#b4c8e9; \n                ')
                continue
            self.config_list_vl.itemAt(i).widget().setStyleSheet('\n                                    background-color:white; \n                                ')
        description = self.curr_config['description']
        if not STATE_211_MODE:
            description = re.sub('such as:[\\s\\S]*', '', description).strip()
        self.config_desc_tw.setHtml(description)
        self.btn_export_project.hide() if self.curr_config.get('custome', False) else self.btn_export_project.show()
        self.btn_start_dl_program.hide() if self.curr_config.get('custome', False) and self.password_checked or self.is_pms else self.btn_start_dl_program.show()
        self.load_params()
        self.show_form_view_by_section()
        self.set_addr_irq_disabled()
        if self.is_pms:
            self.rx_gain_cb.setCurrentIndex(1)
        elif self.is_longrange:
            pass
        
        1(0)
        self.check_rx_len_is_show()
        self.show_frame_struct()
        self.set_trx_len_max()
        self.set_white_offset_max_val()
        self.set_tx_noack_cb()
        self.set_trx_delay_time()
        self.init_trx_addr_form_view()
        self.reload_tx_mode_cb()
        self.reload_rx_mode_cb()
        self.reload_data_rate_cb()
        self.get_white_list()
        self.changeFormViewByChipMode()
        self.showHideTxDeviation()
        self.showHideRxTimeOut()
        self.set_en_dpl_cb()
        self.set_txnoack_cb()
        self.reload_s2s8_mode_cb()
        self.time_seq_signal.emit()
        self.initTimeSeqTableWidget()
        time.sleep(0.2)
        self.is_loading_params = False
        if self.curr_config:
            self.curr_config['s2S8Mode'] = self.s2s8_cb.currentIndex()
        config_name = self.curr_config['name']
        if not 'Normal' in config_name:
            pass
        is_enable = 'Carrier' in config_name
        if STATE_211_MODE:
            pass

    
    def show_form_view_by_section(self):
        multi_pipe_show = False
        ble_chip_mode_show = False
        self.is_longrange = 'LongRange' in self.curr_config.get('name')
        self.adva_et.setText('CCCCCCCCCCCC')
        hide_views = [
            self.tx_noack_label,
            self.tx_noack_cb]
        if self.trx_mode == 2:
            show_views = [
                self.channel_label,
                self.channel_sp,
                self.channel_unit_label]
            hide_views += [
                self.chip_mode_label,
                self.chip_mode_cb,
                self.enhance_label,
                self.enhance_cb,
                self.crc_label,
                self.crc_cb,
                self.rx_len_label,
                self.rx_len_sp,
                self.rx_mode_label,
                self.rx_mode_cb,
                self.tx_mode_label,
                self.tx_mode_cb,
                self.rx_addr_label,
                self.rx_addr_et,
                self.en_white_label,
                self.en_white_cb,
                self.rx_timeout_label,
                self.rx_timeout_sp,
                self.tx_len_label,
                self.tx_len_sp,
                self.tx_addr_width_label,
                self.tx_addr_width_sp,
                self.tx_addr_width_unit,
                self.tx_addr_width_unit,
                self.tx_addr_label,
                self.tx_addr_et,
                self.s2s8_label,
                self.s2s8_cb,
                self.ble_head_num_label,
                self.ble_head_num_cb,
                self.en_dpl_label,
                self.en_dpl_cb,
                self.daterate_cb,
                self.datarate_label,
                self.datarate_unit_label]
        elif not self.check_if_ble_mode():
            hide_views += [
                self.length_filter_mode_label,
                self.length_filter_mode_cb,
                self.white_list_offset_label,
                self.white_list_offset_sp,
                self.white_list_match_mode_label,
                self.white_list_match_mode_cb,
                self.adva_white_list_label,
                self.adva_white_list_et]
            if self.is_longrange:
                multi_pipe_show = False
                ble_chip_mode_show = True
                self.tx_mode_cb.setCurrentIndex(1)
                show_views = [
                    self.datarate_label,
                    self.daterate_cb,
                    self.rx_mode_label,
                    self.rx_mode_cb,
                    self.tx_mode_label,
                    self.tx_mode_cb,
                    self.tx_len_label,
                    self.tx_len_sp,
                    self.rx_len_label,
                    self.rx_len_sp,
                    self.channel_label,
                    self.channel_sp,
                    self.channel_unit_label,
                    self.s2s8_label,
                    self.s2s8_cb,
                    self.tx_addr_width_label,
                    self.tx_addr_width_sp,
                    self.tx_addr_width_unit,
                    self.length_filter_mode_label,
                    self.length_filter_mode_cb,
                    self.tx_addr_label,
                    self.tx_addr_et,
                    self.rx_addr_label,
                    self.rx_addr_et]
                hide_views += [
                    self.chip_mode_label,
                    self.chip_mode_cb,
                    self.ble_head_num_label,
                    self.ble_head_num_cb,
                    self.ble_channel_label,
                    self.ble_channel_cb,
                    self.en_dpl_label,
                    self.en_dpl_cb,
                    self.crc_label,
                    self.crc_cb,
                    self.en_white_label,
                    self.en_white_cb,
                    self.rx_timeout_label,
                    self.rx_timeout_sp,
                    self.adva_white_list_label,
                    self.adva_white_list_et,
                    self.white_list_match_mode_label,
                    self.white_list_match_mode_cb,
                    self.ble_channel_label,
                    self.ble_channel_cb,
                    self.white_list_offset_label,
                    self.white_list_offset_sp,
                    self.auto_delay_label,
                    self.auto_delay_sp,
                    self.trx_delay_time_sp,
                    self.trx_delay_time_label,
                    self.auto_max_cnt_label,
                    self.auto_max_cnt_sp]
            elif self.enhance_cb.currentIndex() == 0:
                show_views = [
                    self.datarate_label,
                    self.daterate_cb,
                    self.chip_mode_label,
                    self.chip_mode_cb,
                    self.channel_label,
                    self.channel_sp,
                    self.channel_unit_label]
                hide_views += [
                    self.s2s8_label,
                    self.s2s8_cb,
                    self.ble_head_num_label,
                    self.ble_head_num_cb,
                    self.en_dpl_label,
                    self.en_dpl_cb,
                    self.rx_timeout_label,
                    self.rx_timeout_sp,
                    self.ble_channel_label,
                    self.ble_channel_cb,
                    self.length_filter_mode_label,
                    self.length_filter_mode_cb,
                    self.trx_delay_time_label,
                    self.trx_delay_time_sp,
                    self.auto_delay_label,
                    self.auto_delay_sp,
                    self.auto_max_cnt_label,
                    self.auto_max_cnt_sp,
                    self.white_list_match_mode_label,
                    self.white_list_match_mode_cb,
                    self.rx_mode_label,
                    self.rx_mode_cb,
                    self.adva_white_list_label,
                    self.adva_white_list_et,
                    self.white_list_offset_label,
                    self.white_list_offset_sp]
                if self.curr_config.get('custome', False):
                    multi_pipe_show = True
                    show_views += [
                        self.rx_len_label,
                        self.rx_len_sp,
                        self.tx_addr_width_label,
                        self.tx_addr_width_sp,
                        self.tx_addr_width_unit,
                        self.rx_mode_label,
                        self.rx_mode_cb,
                        self.crc_label,
                        self.crc_cb,
                        self.tx_len_label,
                        self.tx_len_sp,
                        self.tx_mode_label,
                        self.en_white_label,
                        self.en_white_cb,
                        self.tx_mode_cb,
                        self.rx_addr_label,
                        self.rx_addr_et,
                        self.tx_addr_label,
                        self.tx_addr_et,
                        self.rx_timeout_label,
                        self.rx_timeout_sp]
                    hide_views += [
                        self.enhance_label,
                        self.enhance_cb,
                        self.s2s8_label,
                        self.s2s8_cb,
                        self.rx_len_label,
                        self.rx_len_sp]
                elif self.trx_mode == 1:
                    multi_pipe_show = True
                    show_views += [
                        self.tx_len_label,
                        self.tx_len_sp,
                        self.tx_addr_width_label,
                        self.tx_addr_width_sp,
                        self.tx_addr_width_unit,
                        self.tx_addr_label,
                        self.tx_addr_et]
                    hide_views += [
                        self.chip_mode_label,
                        self.chip_mode_cb,
                        self.enhance_label,
                        self.enhance_cb,
                        self.crc_label,
                        self.crc_cb,
                        self.rx_len_label,
                        self.rx_len_sp,
                        self.rx_mode_label,
                        self.rx_mode_cb,
                        self.tx_mode_label,
                        self.tx_mode_cb,
                        self.rx_addr_label,
                        self.rx_addr_et,
                        self.en_white_label,
                        self.en_white_cb]
                else:
                    multi_pipe_show = True
                    show_views += [
                        self.rx_len_label,
                        self.rx_len_sp,
                        self.tx_addr_width_label,
                        self.tx_addr_width_sp,
                        self.tx_addr_width_unit,
                        self.rx_addr_label,
                        self.rx_addr_et]
                    hide_views += [
                        self.enhance_label,
                        self.enhance_cb,
                        self.crc_label,
                        self.crc_cb,
                        self.tx_len_label,
                        self.tx_len_sp,
                        self.tx_mode_label,
                        self.tx_mode_cb,
                        self.rx_mode_label,
                        self.rx_mode_cb,
                        self.tx_addr_label,
                        self.tx_addr_et,
                        self.en_white_label,
                        self.en_white_cb,
                        self.rx_timeout_label,
                        self.rx_timeout_sp]
            else:
                show_views = [
                    self.datarate_label,
                    self.daterate_cb,
                    self.chip_mode_label,
                    self.chip_mode_cb,
                    self.channel_label,
                    self.channel_sp,
                    self.channel_unit_label,
                    self.tx_noack_label,
                    self.tx_noack_cb]
                hide_views = [
                    self.s2s8_label,
                    self.s2s8_cb,
                    self.ble_head_num_label,
                    self.ble_head_num_cb,
                    self.ble_channel_label,
                    self.ble_channel_cb]
                if self.curr_config.get('custome', False):
                    multi_pipe_show = True
                    show_views += [
                        self.rx_len_label,
                        self.rx_len_sp,
                        self.tx_addr_width_label,
                        self.tx_addr_width_sp,
                        self.tx_addr_width_unit,
                        self.rx_mode_label,
                        self.rx_mode_cb,
                        self.crc_label,
                        self.crc_cb,
                        self.tx_len_label,
                        self.tx_len_sp,
                        self.tx_mode_label,
                        self.en_white_label,
                        self.en_white_cb,
                        self.tx_mode_cb,
                        self.rx_addr_label,
                        self.rx_addr_et,
                        self.tx_addr_label,
                        self.tx_addr_et,
                        self.auto_delay_label,
                        self.auto_delay_sp,
                        self.trx_delay_time_label,
                        self.trx_delay_time_sp,
                        self.rx_timeout_label,
                        self.rx_timeout_sp,
                        self.auto_max_cnt_label,
                        self.auto_max_cnt_sp,
                        self.en_dpl_label,
                        self.en_dpl_cb]
                    hide_views += [
                        self.enhance_label,
                        self.enhance_cb]
                elif self.trx_mode == 1:
                    multi_pipe_show = True
                    show_views += [
                        self.tx_len_label,
                        self.tx_len_sp,
                        self.tx_addr_width_label,
                        self.tx_addr_width_sp,
                        self.tx_addr_width_unit,
                        self.tx_addr_label,
                        self.tx_addr_et,
                        self.rx_addr_label,
                        self.rx_addr_et,
                        self.rx_timeout_label,
                        self.rx_timeout_sp]
                    hide_views += [
                        self.enhance_label,
                        self.enhance_cb,
                        self.crc_label,
                        self.crc_cb,
                        self.rx_len_label,
                        self.rx_len_sp,
                        self.rx_mode_label,
                        self.rx_mode_cb,
                        self.tx_mode_label,
                        self.tx_mode_cb,
                        self.en_white_label,
                        self.en_white_cb,
                        self.auto_delay_label,
                        self.auto_delay_sp,
                        self.trx_delay_time_label,
                        self.trx_delay_time_sp,
                        self.auto_max_cnt_label,
                        self.auto_max_cnt_sp,
                        self.trx_delay_time_label,
                        self.trx_delay_time_sp,
                        self.en_dpl_label,
                        self.en_dpl_cb]
                else:
                    multi_pipe_show = True
                    show_views += [
                        self.tx_len_label,
                        self.tx_len_sp,
                        self.tx_addr_width_label,
                        self.tx_addr_width_sp,
                        self.tx_addr_width_unit,
                        self.rx_addr_label,
                        self.rx_addr_et,
                        self.tx_addr_label,
                        self.tx_addr_et,
                        self.trx_delay_time_label,
                        self.trx_delay_time_sp]
                    hide_views += [
                        self.enhance_label,
                        self.enhance_cb,
                        self.crc_label,
                        self.crc_cb,
                        self.rx_len_label,
                        self.rx_len_sp,
                        self.rx_mode_label,
                        self.rx_mode_cb,
                        self.tx_mode_label,
                        self.tx_mode_cb,
                        self.en_white_label,
                        self.en_white_cb,
                        self.auto_delay_label,
                        self.auto_delay_sp,
                        self.auto_max_cnt_label,
                        self.auto_max_cnt_sp,
                        self.rx_timeout_label,
                        self.rx_timeout_sp,
                        self.en_dpl_label,
                        self.en_dpl_cb]
        else:
            self.tx_addr_et.setText('8E89BED6')
            self.rx_addr_et.setText('8E89BED6')
            multi_pipe_show = False
            ble_chip_mode_show = True
            hide_views += [
                self.tx_addr_et,
                self.tx_addr_label,
                self.rx_addr_et,
                self.rx_addr_label,
                self.en_dpl_label,
                self.en_dpl_cb,
                self.en_white_label,
                self.en_white_cb,
                self.channel_label,
                self.channel_sp,
                self.channel_unit_label,
                self.rx_timeout_label,
                self.rx_timeout_sp,
                self.auto_delay_label,
                self.auto_delay_sp,
                self.trx_delay_time_label,
                self.trx_delay_time_sp,
                self.auto_max_cnt_label,
                self.auto_max_cnt_sp,
                self.tx_addr_width_label,
                self.tx_addr_width_sp,
                self.tx_addr_width_unit,
                self.crc_label,
                self.crc_cb]
            if self.curr_config.get('custome', False):
                show_views = [
                    self.datarate_label,
                    self.daterate_cb,
                    self.rx_len_label,
                    self.rx_len_sp,
                    self.rx_mode_label,
                    self.rx_mode_cb,
                    self.tx_mode_label,
                    self.tx_mode_cb,
                    self.length_filter_mode_label,
                    self.length_filter_mode_cb,
                    self.ble_channel_label,
                    self.ble_channel_cb,
                    self.white_list_match_mode_label,
                    self.white_list_match_mode_cb,
                    self.adva_white_list_label,
                    self.adva_white_list_et,
                    self.ble_head_num_label,
                    self.ble_head_num_cb,
                    self.white_list_offset_label,
                    self.white_list_offset_sp,
                    self.s2s8_label,
                    self.s2s8_cb,
                    self.rx_addr_label,
                    self.rx_addr_et,
                    self.tx_addr_label,
                    self.tx_addr_et]
                hide_views += [
                    self.chip_mode_label,
                    self.chip_mode_cb,
                    self.enhance_label,
                    self.enhance_cb,
                    self.tx_len_label,
                    self.tx_len_sp]
            elif self.trx_mode == 1:
                show_views = [
                    self.datarate_label,
                    self.daterate_cb,
                    self.ble_channel_label,
                    self.ble_channel_cb,
                    self.tx_addr_label,
                    self.tx_addr_et]
                hide_views += [
                    self.chip_mode_label,
                    self.chip_mode_cb,
                    self.enhance_label,
                    self.enhance_cb,
                    self.crc_label,
                    self.crc_cb,
                    self.tx_len_label,
                    self.tx_len_sp,
                    self.rx_mode_label,
                    self.rx_mode_cb,
                    self.tx_mode_label,
                    self.tx_mode_cb,
                    self.ble_head_num_label,
                    self.ble_head_num_cb,
                    self.rx_addr_label,
                    self.rx_addr_et,
                    self.rx_len_label,
                    self.rx_len_sp,
                    self.white_list_offset_label,
                    self.white_list_offset_sp,
                    self.s2s8_label,
                    self.s2s8_cb,
                    self.length_filter_mode_label,
                    self.length_filter_mode_cb,
                    self.white_list_match_mode_cb,
                    self.white_list_match_mode_label,
                    self.adva_white_list_label,
                    self.adva_white_list_et]
            else:
                show_views = [
                    self.datarate_label,
                    self.daterate_cb,
                    self.length_filter_mode_label,
                    self.length_filter_mode_cb,
                    self.ble_channel_label,
                    self.ble_channel_cb,
                    self.adva_white_list_label,
                    self.adva_white_list_et,
                    self.white_list_match_mode_label,
                    self.white_list_match_mode_cb,
                    self.rx_addr_label,
                    self.rx_addr_et]
                hide_views += [
                    self.chip_mode_label,
                    self.chip_mode_cb,
                    self.enhance_label,
                    self.enhance_cb,
                    self.crc_label,
                    self.crc_cb,
                    self.tx_len_label,
                    self.tx_len_sp,
                    self.rx_mode_label,
                    self.rx_mode_cb,
                    self.tx_mode_label,
                    self.tx_mode_cb,
                    self.ble_head_num_label,
                    self.ble_head_num_cb,
                    self.tx_addr_label,
                    self.tx_addr_et,
                    self.adva_white_list_label,
                    self.adva_white_list_et,
                    self.white_list_offset_label,
                    self.white_list_offset_sp,
                    self.s2s8_label,
                    self.s2s8_cb]
                if self.length_filter_mode_cb.currentIndex() > 0:
                    self.rx_len_label.show()
                    self.rx_len_sp.show()
        if 'Wave' not in self.curr_config['name']:
            self.tabWidget.setTabVisible(1, multi_pipe_show)
            for v in hide_views:
                v.hide()
            for v in show_views:
                v.show()
        else:
            self.tabWidget.setTabVisible(1, False)
            for v in hide_views:
                v.hide()
            for v in show_views:
                v.show()

    
    def load_params(self):
        self.trx_mode = self.curr_config.get('trxMode')
        self.channel_sp.setValue(self.curr_config.get('channel'))
        self.showHideChipModeBle(self.curr_config.get('chipMode') == 3)
        self.chip_mode_cb.setCurrentText(chip_mode_list[self.curr_config.get('chipMode')])
        self.enhance_cb.setCurrentIndex(self.curr_config.get('workMode'))
        self.daterate_cb.setCurrentIndex(self.curr_config.get('dataRate'))
        self.power_spin.setCurrentText('{}dBm'.format(self.curr_config.get('txPower')))
        self.crc_cb.setCurrentIndex(self.curr_config.get('crc'))
        self.tx_len_sp.setValue(self.curr_config.get('txLen'))
        self.rx_len_sp.setValue(self.curr_config.get('rxLen'))
        self.spi_clk_cb.setCurrentIndex(self.curr_config.get('spiClk'))
        self.trx_delay_time_sp.setValue(self.curr_config.get('trxDelayTimeUs', 0))
        self.auto_delay_sp.setValue(self.curr_config.get('autoDelayUs'))
        self.en_white_cb.setCurrentIndex(self.curr_config.get('enWhite'))
        self.en_dpl_cb.setCurrentIndex(self.curr_config.get('enDPL', 0))
        self.auto_max_cnt_sp.setValue(self.curr_config.get('autoMaxCnt'))
        self.en_multi_pipe_cb.setCurrentIndex(self.curr_config.get('enMultiPipe'))
        self.tx_addr_width_sp.setValue(self.curr_config.get('txAddrWidth'))
        self.s2s8_cb.setCurrentIndex(self.curr_config.get('s2S8Mode'))
        self.ble_channel_cb.setCurrentIndex(self.curr_config.get('bleChannel'))
        self.ble_head_num_cb.setCurrentText(str(self.curr_config.get('bleHeadNum')))
        self.rx_addr_et.setText(self.curr_config.get('rxAddress', ''))
        self.tx_addr_et.setText(self.curr_config.get('txAddress', ''))
        self.tx_mode_cb.setCurrentIndex(self.curr_config.get('txMode', 0))
        self.rx_mode_cb.setCurrentIndex(self.curr_config.get('rxMode', 0))
        self.white_list_match_mode_cb.setCurrentIndex(self.curr_config.get('whiteListMatchMode'))
        self.length_filter_mode_cb.setCurrentIndex(self.curr_config.get('lengthFilterMode'))
        self.adva_white_list_et.setText(self.curr_config.get('whiteList'))
        self.white_list_offset_sp.setValue(self.curr_config.get('whiteListOffset'))
        self.on_multi_pipe_change(self.en_multi_pipe_cb.currentIndex())
        for interrupt_btn in self.interrupt_btns:
            interrupt_btn.set_active(True)
        self.ble_head_num_cb.setCurrentIndex(self.curr_config.get('headNum', 1))
        self.show_addrs_or_ble_tab()

    
    def set_row_height(self, table_widget, height):
        table_widget.verticalHeader().setMinimumSectionSize(height)
        for row in range(table_widget.rowCount()):
            table_widget.setRowHeight(row, height)

    
    def show_message_info_box(self, title, msg):
        QMessageBox.information(self, title, msg)

    
    def set_tx_noack_cb(self):
        if not self.enhance_mode_enable():
            self.auto_max_cnt_sp.setMinimum(0)
            self.auto_max_cnt_sp.setValue(0)
        else:
            self.auto_max_cnt_sp.setMinimum(1)

    
    def init_signal(self):
        self.ui_init_signal.connect(self.init_ui)
        self.message_info_signal.connect(self.show_message_info_box)
        self.multi_pipe_ck1.click_connect(partial(self.multi_pipe_ck_click, 0))
        self.multi_pipe_ck2.click_connect(partial(self.multi_pipe_ck_click, 1))
        self.multi_pipe_ck3.click_connect(partial(self.multi_pipe_ck_click, 2))
        self.multi_pipe_ck4.click_connect(partial(self.multi_pipe_ck_click, 3))
        self.multi_pipe_ck5.click_connect(partial(self.multi_pipe_ck_click, 4))
        self.btn_expand_ble_payload.clicked.connect(self.showHideAdvdGroupView)
        self.power_table_widget.currentTextChanged.connect(self.on_power_table_change)
        self.action_doc.triggered.connect(self.open_wiki_web)
        self.action_ver_update.triggered.connect(self.sw_ver_update)
        self.action_export_file.triggered.connect(self.show_preview_dialog)
        self.actionExport_Source_File_pms.triggered.connect(self.show_preview_dialog)
        self.action_export_project.triggered.connect(self.gen_code_thread)
        self.action_whats_new.triggered.connect(self.show_whats_new)
        self.action_rf_test_signal.connect(self.show_disabled_when_download)
        self.actionDFU_Updater.triggered.connect(self.show_dfu_dialog)
        self.confirm_dialog_signal.connect(self.show_confirm_dialog)
        self.rf_test_dialog_signal.connect(self.show_rf_test_dialog)
        self.power_spin.currentIndexChanged.connect(self.on_power_change)
        self.tabWidget.currentChanged.connect(self.on_tab_change)
        self.cb_interface.currentIndexChanged.connect(self.on_interface_change)
        self.ble_name_et.setMaxLength(20)
        self.ble_name_et.textChanged.connect(self.on_ble_name_change)
        self.ble_advd_et.textChanged.connect(self.on_ble_advd_change)
        self.config_list_disabled_signal.connect(self.set_config_list_disabled)
        self.check_rx_addr_signal.connect(self.check_addrs)
        self.ble_channel_signal.connect(self.channel_sp.setValue)
        self.rx_addr_et.set_key('rxAddress', self.on_curr_config_change, self.on_focus_out_event, **('on_form_change', 'on_focus_out_event'))
        self.tx_addr_et.set_key('txAddress', self.on_curr_config_change, self.on_focus_out_event, **('on_form_change', 'on_focus_out_event'))
        self.adva_white_list_et.set_key('whiteList', self.on_curr_config_change, self.on_focus_out_event, **('on_form_change', 'on_focus_out_event'))
        self.ble_head_num_cb.set_key('bleHeadNum', self.on_curr_config_change, **('on_form_change',))
        self.time_seq_signal.connect(self.show_time_seq_img)
        self.tx_len_sp.set_key('txLen', self.on_curr_config_change)
        self.rx_len_sp.set_key('rxLen', self.on_curr_config_change)
        self.chip_mode_cb.set_key('chipMode', self.on_curr_config_change)
        self.enhance_cb.set_key('workMode', self.on_curr_config_change)
        self.xtal_freq_cb.set_key('xtalFreq', self.on_curr_config_change)
        self.rx_mode_cb.set_key('rxMode', self.on_curr_config_change)
        self.s2s8_cb.set_key('s2S8Mode', self.on_curr_config_change)
        self.channel_sp.set_key('channel', self.on_curr_config_change)
        self.daterate_cb.set_key('dataRate', self.on_curr_config_change)
        self.ble_channel_cb.set_key('bleChannel', self.on_curr_config_change)
        self.trx_delay_time_sp.set_key('tRxDelayTimeUs', self.on_curr_config_change)
        self.spi_clk_cb.set_key('spiClk', self.on_curr_config_change)
        self.crc_cb.set_key('crc', self.on_curr_config_change)
        self.tx_addr_width_sp.set_key('txAddrWidth', self.on_curr_config_change)
        self.length_filter_mode_cb.set_key('lengthFilterMode', self.on_curr_config_change)
        multi_pipe_addr_widgets = [
            self.pipex_addr4,
            self.pipex_addr3,
            self.pipex_addr2,
            self.pipex_addr1,
            self.pipe1_addr0,
            self.pipe2_addr0,
            self.pipe3_addr0,
            self.pipe4_addr0,
            self.pipe5_addr0]
        multi_pipe_addr_keys = [
            'pipex_addr4',
            'pipex_addr3',
            'pipex_addr2',
            'pipex_addr1',
            'pipe1_addr0',
            'pipe2_addr0',
            'pipe3_addr0',
            'pipe4_addr0',
            'pipe5_addr0']
        for index, p in enumerate(multi_pipe_addr_widgets):
            p.set_key(multi_pipe_addr_keys[index], self.on_curr_config_change)
        self.white_list_match_mode_cb.set_key('whiteListMatchMode', self.on_curr_config_change)
        self.en_multi_pipe_cb.currentIndexChanged.connect(self.on_multi_pipe_change)
        self.en_11dbm_cb.currentIndexChanged.connect(self.on_11dbm_en_changed)
        self.btn_gen_code.clicked.connect(self.show_preview_dialog)
        self.export_mode_cb.currentIndexChanged.connect(self.on_export_mode_changed)
        self.btn_export_project.clicked.connect(self.gen_code_thread)
        self.btn_start_dl_program.clicked.connect(partial(self.build_download_thread, False))
        self.btn_start_dl_oled_program.clicked.connect(partial(self.build_download_thread, True))
        self.dfu_dev_status_signal.connect(self.on_dfu_status_change)
        self.dfu_progress_signal.connect(self.dfu_dev_status_pb.setValue)
        self.dfu_progress_msg_signal.connect(self.on_dfu_msg_change)
        self.hid_dev_list = []
        self.hid_device_action_signal.connect(self.show_hid_devices)
        self.hid_device_name_signal.connect(self.show_select_hid_name)
        self.dl_program_btn_signal.connect(self.download_dfu_btn_disabed)

    
    def download_dfu_btn_disabed(self, is_disabled):
        self.btn_start_dl_program.setDisabled(is_disabled)
        self.btn_start_dl_oled_program.setDisabled(is_disabled)

    
    def show_disabled_when_download(self, is_downloading):
        self.actionRf_Communication_Test.setDisabled(is_downloading)
        self.actionDFU_Updater.setDisabled(is_downloading)
        actions = self.menuDevices.actions()
        for a in actions:
            a.setDisabled(is_downloading)
        actions = self.menuTests.actions()
        for a in actions:
            a.setDisabled(is_downloading)

    
    def on_export_mode_changed(self, val):
        if val:
            self.is_pms = True
            self.base_freq_cb.show()
            self.base_freq_label.show()
            self.cb_interface.setCurrentIndex(1)
            self.cb_interface.setDisabled(True)
            self.on_selection_changed(0)
            self.btn_start_dl_program.hide()
            self.rx_gain_cb.setCurrentIndex(1)
            self.rx_gain_cb.setDisabled(True)
        else:
            self.is_pms = False
            self.base_freq_cb.hide()
            self.base_freq_label.hide()
            self.cb_interface.setDisabled(False)
            self.rx_gain_cb.setCurrentIndex(0)
            self.rx_gain_cb.setDisabled(False)
        self.tx_len_sp.setDisabled(val > 0)
        self.rx_len_sp.setDisabled(val > 0)
        self.disable_configs_by_pms()

    
    def get_short_hid_name(self, full_name):
        short_name = ''
        if not full_name:
            short_name = ''
        elif '-dfu' not in full_name:
            short_name = full_name
        else:
            short_name = full_name.split('-dfu')[1].replace('&0&0000', '')
        return short_name

    
    def on_11dbm_en_changed(self, val):
        if val:
            self.power_spin.addItem('11dBm')
            self.power_table_widget.addItem('11dBm')
        else:
            index_11dBm = -1
            for i in range(self.power_spin.count()):
                if self.power_spin.itemText(i) == '11dBm':
                    index_11dBm = i
                
                if index_11dBm > 0:
                    self.power_spin.removeItem(index_11dBm)
                    self.power_table_widget.removeItem(index_11dBm)

    
    def showHideChipModeBle(self, is_show):
        index_ble = -1
        for i in range(self.chip_mode_cb.count()):
            if self.chip_mode_cb.itemText(i) == BLE_CHIP_MODE_NAME:
                index_ble = i
            
            if is_show and index_ble < 0:
                self.chip_mode_cb.addItem(BLE_CHIP_MODE_NAME)
                self.chip_mode_cb.setCurrentText(BLE_CHIP_MODE_NAME)
            elif is_show and index_ble >= 0:
                self.chip_mode_cb.removeItem(index_ble)

    
    def show_hid_devices(self):
        self.menuDevices.clear()
        self.menuTests.clear()
        self.hid_action_group = QActionGroup(self)
        self.hid_action_group.setExclusive(True)
        self.hid_action_group.triggered.connect(self.on_hid_action_triggered)
        self.rf_hid_action_group = QActionGroup(self)
        self.rf_hid_action_group.triggered.connect(self.find_serial_rf_test_dialog)
        index = 0
        select_devs = []
    # WARNING: Decompyle incomplete

    
    def add_radio_action(self, menu, group, text, checked = (False,)):
        action = QAction('EVB-{}'.format(text), self)
        action.setCheckable(True)
        action.setChecked(checked)
        group.addAction(action)
        menu.addAction(action)
        action.setDisabled(self.is_dfu_downloading)
        return action

    
    def add_rf_test_radios(self, menu, group, text):
        action = QAction('{}-{}'.format('RFTest', text))
        group.addAction(action)
        menu.addAction(action)
        action.setDisabled(self.is_dfu_downloading)
        return action

    
    def on_hid_selection_changed(self, hid_dev_name):
        print('hid_dev_name:{}'.format(hid_dev_name))

    
    def on_hid_action_triggered(self, action):
        self.hid_select_uuid = action.text().split('-')[1]
        print('hid_dev_action:{}'.format(self.hid_select_uuid))
        self.hid_device_name_signal.emit()

    
    def show_select_hid_name(self):
        show_dev = 'Unknown' if self.hid_select_uuid or len(self.hid_select_uuid) == 0 else 'EVB-{}'.format(self.hid_select_uuid)
        self.hid_device_id_label.setText('Device:{}'.format(show_dev))

    
    def on_dfu_status_change(self, is_online):
        self.dfu_dev_status_label.setText('EVB:Connected' if is_online else 'EVB:Disconnected')
        color = 'rgb(0,200,0)' if is_online else 'rgb(237,28,36)'
        self.dfu_dev_status_label.setStyleSheet('color:{}'.format(color))
        self.dfu_dev_status_img.setPixmap(QPixmap(':/images/icon_online.png') if is_online else QPixmap(':/images/icon_offline.png'))
        self.dfu_dev_status_img.setMaximumSize(18, 18)
        self.dfu_dev_status_img.setScaledContents(True)

    
    def on_dfu_msg_change(self, msg, success):
        colors = [
            'rgb(0,200,0)',
            'rgb(237,28,36)',
            'rgb(0,100,255)']
        color = colors[success]
        self.dfu_dev_pb_info_label.setStyleSheet('color:{}'.format(color))
        self.dfu_dev_pb_info_label.setText(msg)

    
    def find_serial_rf_test_dialog(self, action):
        pass
    # WARNING: Decompyle incomplete

    
    def show_rf_test_dialog(self, dev_path, dev_name, hid_uuid):
        self.show_dev_dfu_progress(True)
        if hid_uuid not in self.rf_dialog_dict:
            rf_dialog = RfTestDialog(self, dev_path, dev_name, hid_uuid, self.start_download_test_fw)
            self.rf_dialog_dict[hid_uuid] = rf_dialog
        else:
            rf_dialog = self.rf_dialog_dict[hid_uuid]
            rf_dialog.sel_dev_name = dev_name
        rf_dialog.setWindowFlags(QtCore.Qt.Window)
        rf_dialog.show_dialog()

    
    def start_serial_port_search_thread(self):
        self.serial_port_thread = threading.Thread(self.start_serial_port_search, **('target',))
        self.serial_port_thread.setDaemon(True)
        self.serial_port_thread.start()

    
    def show_confirm_dialog(self, type):
        if type == 0:
            self.confirmDialog = ConfirmDialog('', None, 'Yes', 'No')
            self.confirmDialog.setMsgAlignLeft()
            self.confirmDialog.showDialog('Not a test firmware', 'Warning: This firmware cannot be used for RF testing.\nStep 1: Download the test firmware\nStep 2: Proceed with RF test', self.start_dfu_atcmd_firmware_thread, True, **('title', 'message', 'onConfirm', 'is_show_confirm'))

    
    def start_serial_port_search(self):
        if self.is_serial_searching:
            return None
        self.is_serial_searching = None
        self.panATClass = PanATClass()
    # WARNING: Decompyle incomplete

    
    def get_serial_port_by_hid(self, hid_path):
        dm = DeviceManager()
        dm.root.rescan()
        all_devices = dm.all_devices
        usb_devices = (lambda .0: [ device for device in .0 ])(all_devices)
    # WARNING: Decompyle incomplete

    
    def on_interface_change(self, arg):
        self.clk_label.setText('SPI CLK' if self.cb_interface.currentIndex() == 0 else 'BUS CLK')
        self.show_warning_tips()
        self.message_info_signal.emit('Interface Changed', 'After switching the interface method, the device must be powered on again to ensure normal communication.')

    
    def on_power_table_change(self, v):
        power_table_arr = self.power_table_widget.currentTextList()
        if len(power_table_arr) == 0:
            return None
        config_zero_dbm = None in power_table_arr
        self.optmize_0dbm_cb.setEnabled(config_zero_dbm)
        if not config_zero_dbm:
            self.optmize_0dbm_cb.setCurrentIndex(0)
        power_table_index_arr = self.power_table_widget.currentIndexList()
        if self.power_spin.currentIndex() not in power_table_index_arr:
            self.power_spin.setCurrentIndex(power_table_index_arr[0])

    
    def show_dfu_dialog(self):
        self.show_dev_dfu_progress(True)
        self.start_dfu_atcmd_firmware_thread()

    
    def show_dfu_oled_dialog(self):
        self.show_dev_dfu_progress(True)
        self.start_dfu_oled_firmware_thread()

    
    def start_dfu_atcmd_firmware_thread(self):
        self.show_dev_dfu_progress(True)
        self.dfu_atcmd_thread = threading.Thread(self.start_download_test_fw, **('target',))
        self.dfu_atcmd_thread.setDaemon(True)
        self.dfu_atcmd_thread.start()
        return self.dfu_atcmd_thread

    
    def start_dfu_oled_firmware_thread(self):
        self.show_dev_dfu_progress(True)
        self.dfu_atcmd_thread = threading.Thread(self.start_download_oled_fw, **('target',))
        self.dfu_atcmd_thread.setDaemon(True)
        self.dfu_atcmd_thread.start()
        return self.dfu_atcmd_thread

    
    def get_atcmd_bin_name(self):
        if self.cb_interface.currentIndex() == 0:
            if self.xtal_freq_cb.currentIndex() > 0:
                return 'app.signed_spi3_32m'
            return None
        if None.cb_interface.currentIndex() == 1:
            if self.xtal_freq_cb.currentIndex() > 0:
                return 'app.signed_i2c_32m'
            return None
        if None.cb_interface.currentIndex() == 2:
            if self.xtal_freq_cb.currentIndex() > 0:
                return 'app.signed_spi4_32m'
            return None

    
    def get_oled_bin_name(self):
        if self.cb_interface.currentIndex() == 0:
            if self.xtal_freq_cb.currentIndex() > 0:
                return 'app.signed_spi3_32m_oled'
            return None
        if None.cb_interface.currentIndex() == 1:
            if self.xtal_freq_cb.currentIndex() > 0:
                return 'app.signed_i2c_32m_oled'
            return None
        if None.cb_interface.currentIndex() == 2:
            if self.xtal_freq_cb.currentIndex() > 0:
                return 'app.signed_spi4_32m_oled'
            return None

    
    def start_download_test_fw(self, hid_uuid = (None,)):
        pass
    # WARNING: Decompyle incomplete

    
    def start_download_oled_fw(self, hid_uuid = (None,)):
        pass
    # WARNING: Decompyle incomplete

    
    def show_whats_new(self):
        if STATE_211_MODE:
            self.whats_new_dialog.show()
        else:
            self.whats_new_dialog_216.show()

    
    def on_tab_change(self, v):
        if v == 3:
            self.time_seq_signal.emit()

    
    def on_sdk_ver_change(self, val):
        self.xtal_freq_cb.setDisabled(val == 0)

    
    def on_power_change(self, index):
        select_index_list = self.power_table_widget.currentIndexList()
        if index not in select_index_list:
            select_index_list.append(index)
        self.power_table_widget.setCheckedItems(select_index_list)

    
    def open_wiki_web(self):
        url = 'https://docs.panchip.com/pan211xdk-doc/' if STATE_211_MODE else 'https://wiki.panchip.com/ble-lite/2-4g-t-rx/pan216x-%e7%b3%bb%e5%88%97%e4%ba%a7%e5%93%81/'
        webbrowser.open(url)

    
    def sw_ver_update(self):
        self.updater.dealCheckUpdate(False)

    
    def set_trx_len_max(self):
        if self.check_if_ble_mode():
            self.tx_len_sp.setMaximum(37)
            self.tx_len_sp.setMinimum(6)
            self.rx_len_sp.setMaximum(37)
        elif self.enhance_cb.currentIndex():
            pass
        
        64(128)
        self.tx_len_sp.setMinimum(0)
        self.rx_len_sp.setMaximum(64 if self.enhance_cb.currentIndex() else 128)

    
    def set_white_offset_max_val(self):
        pass

    
    def set_addr_form_disabled(self):
        multi_addr_et_arr = [
            self.pipex_addr4,
            self.pipex_addr3,
            self.pipex_addr2,
            self.pipex_addr1]
        diabled_cnt = 5 - self.tx_addr_width_sp.value()
        for i in range(5):
            if i < len(multi_addr_et_arr) and self.en_multi_pipe_cb.currentIndex():
                pass
            multi_addr_et_arr[i].setDisabled(False)
            if i < len(multi_addr_et_arr) and not multi_addr_et_arr[i].text():
                pass
            multi_addr_et_arr[i].setText('CC')
        if diabled_cnt:
            for i in range(diabled_cnt):
                multi_addr_et_arr[i].setDisabled(True)
                multi_addr_et_arr[i].setText('')

    
    def reload_data_rate_cb(self):
        if self.daterate_cb.count() == 0:
            datarate_list = [
                '1M',
                '2M',
                '250K']
            model = QStandardItemModel(self.daterate_cb)
            for d in datarate_list:
                item = QStandardItem(d)
                item.setEnabled(True)
                if self.xtal_freq_cb.currentIndex() == 0 and d == '2M':
                    item.setEnabled(False)
                elif self.is_longrange:
                    item.setEnabled(False)
                model.appendRow(item)
            self.daterate_cb.setModel(model)
        else:
            model = self.daterate_cb.model()
            if isinstance(model, QStandardItemModel):
                index = model.index(1, 0)
                item = model.itemFromIndex(index)
                if item:
                    if not (self.xtal_freq_cb.currentIndex() == 0):
                        pass
                    item.setEnabled(not (self.is_longrange))
            if self.xtal_freq_cb.currentIndex() == 0 and self.daterate_cb.currentIndex() == 1:
                self.daterate_cb.setCurrentIndex(0)

    
    def reload_spi_clk_cb(self):
        self.spi_clk_cb.clear()
        datarate_list = [
            '1MHz',
            '2MHz',
            '4MHz',
            '8MHz',
            '10MHz']
        model = QStandardItemModel(self.spi_clk_cb)
        for d in datarate_list:
            item = QStandardItem(d)
            item.setEnabled(True)
            if self.xtal_freq_cb.currentIndex() == 0 and d == '10MHz':
                item.setEnabled(False)
            model.appendRow(item)
        self.spi_clk_cb.setModel(model)

    
    def reload_tx_mode_cb(self):
        self.tx_mode_cb.clear()
        data_list = [
            'SINGLE',
            'CONTINOUS']
        model = QStandardItemModel(self.tx_mode_cb)
        for index, d in enumerate(data_list):
            item = QStandardItem(d)
            item.setEnabled(True)
            if self.enhance_cb.currentIndex() > 0 and index > 0:
                item.setEnabled(False)
            model.appendRow(item)
        self.tx_mode_cb.setModel(model)
        self.tx_mode_cb.setCurrentIndex(0)

    
    def reload_rx_mode_cb(self):
        self.rx_mode_cb.clear()
        if self.is_longrange:
            data_list = [
                'SINGLE_WITH_TIMEOUT',
                'CONTINOUS']
            model = QStandardItemModel(self.rx_mode_cb)
            for index, d in enumerate(data_list):
                item = QStandardItem(d)
                item.setEnabled(True)
                model.appendRow(item)
            self.rx_mode_cb.setModel(model)
            self.rx_mode_cb.setCurrentIndex(1)
        elif self.enhance_mode_enable():
            data_list = [
                'CONTINOUS',
                'CONTINOUS_WITH_TIMEOUT']
            model = QStandardItemModel(self.rx_mode_cb)
            for index, d in enumerate(data_list):
                item = QStandardItem(d)
                item.setEnabled(True)
                model.appendRow(item)
            self.rx_mode_cb.setModel(model)
            self.rx_mode_cb.setCurrentIndex(0)
        else:
            data_list = [
                'SINGLE',
                'SINGLE_WITH_TIMEOUT',
                'CONTINOUS']
            model = QStandardItemModel(self.rx_mode_cb)
            for index, d in enumerate(data_list):
                item = QStandardItem(d)
                item.setEnabled(True)
                model.appendRow(item)
            self.rx_mode_cb.setModel(model)
            self.rx_mode_cb.setCurrentIndex(2)

    
    def reload_s2s8_mode_cb(self):
        if self.s2s8_cb.count() == 0:
            data_list = [
                'OFF',
                'S2',
                'S8']
            model = QStandardItemModel(self.s2s8_cb)
            for index, d in enumerate(data_list):
                item = QStandardItem(d)
                item.setEnabled(True)
                if index > 0 and self.daterate_cb.currentIndex() == 1:
                    item.setEnabled(False)
                model.appendRow(item)
            self.s2s8_cb.setModel(model)
        else:
            model = self.s2s8_cb.model()
            if isinstance(model, QStandardItemModel):
                index = model.index(1, 0)
                item = model.itemFromIndex(index)
                item.setEnabled(self.daterate_cb.currentIndex() != 1)
                index = model.index(2, 0)
                item = model.itemFromIndex(index)
                item.setEnabled(self.daterate_cb.currentIndex() != 1)
                if self.daterate_cb.currentIndex() == 1:
                    self.s2s8_cb.setCurrentIndex(0)

    
    def check_rx_len_is_show(self):
        if not self.check_if_ble_mode() and self.is_longrange:
            self.rx_len_label.setText('LegnthFilterThreshold')
        else:
            self.rx_len_label.setText('RxLen')

    
    def set_tx_rx_mode(self):
        if self.check_if_ble_mode():
            self.rx_mode_cb.setCurrentIndex(2)
            self.tx_mode_cb.setCurrentIndex(0)
            self.rx_mode_cb.setDisabled(True)
            self.tx_mode_cb.setDisabled(True)
        elif self.enhance_cb.currentIndex() == 0:
            pass
        
        2(0)
        self.tx_mode_cb.setCurrentIndex(0)
        self.rx_mode_cb.setDisabled(False)
        self.tx_mode_cb.setDisabled(False)

    
    def changeFormViewByChipMode(self):
        is_ble = self.check_if_ble_mode()
        ble_form_view_list = [
            self.btn_expand_ble_payload]
        for v in ble_form_view_list:
            v.show() if is_ble else v.hide()
        self.resetAdvaGroupView(True)

    
    def set_en_dpl_cb(self):
        self.en_dpl_cb.setCurrentIndex(1 if self.enhance_cb.currentText() == 'Enhance' else 0)

    
    def set_txnoack_cb(self):
        self.tx_noack_cb.setCurrentIndex(0 if self.enhance_cb.currentText() == 'Enhance' else 1)

    
    def on_curr_config_change(self, key_name, val):
        if self.is_loading_params:
            return None
        if None == 'txLen':
            self.set_trx_delay_time()
            if 'Payload' in self.frame_view.label_items:
                self.frame_view.label_items['Payload'].setPlainText('{}-byte'.format(self.tx_len_sp.value()))
            self.setTimeSeqTableDataList()
        elif key_name == 'chipMode':
            print('{}:{}'.format(key_name, val))
            self.show_addrs_or_ble_tab()
            self.showHideTxDeviation()
            self.changeFormViewByChipMode()
            if self.check_if_ble_mode():
                self.set_ble_channel()
                self.crc_cb.setCurrentIndex(3)
                self.enhance_cb.setCurrentIndex(0)
            self.set_trx_len_max()
            self.set_trx_delay_time()
            self.show_frame_struct()
            self.setTimeSeqTableDataList()
            self.en_white_cb.setCurrentIndex(1 if self.chip_mode_cb.currentText() != 'FS01' else 0)
        elif key_name == 'bleChannel':
            self.set_ble_channel()
        elif key_name == 'xtalFreq':
            self.reload_data_rate_cb()
            self.reload_spi_clk_cb()
        elif key_name == 'workMode':
            self.show_frame_struct()
            self.set_trx_delay_time()
            self.set_trx_len_max()
            self.time_seq_signal.emit()
            self.setTimeSeqTableDataList()
            self.reload_tx_mode_cb()
            self.reload_rx_mode_cb()
            self.set_en_dpl_cb()
            self.set_txnoack_cb()
        elif key_name == 'rxMode':
            self.showHideRxTimeOut()
        elif key_name == 'rxLen':
            self.set_trx_delay_time()
            self.set_white_offset_max_val()
        elif key_name == 'trxMode':
            self.set_trx_delay_time()
            self.time_seq_signal.emit()
            self.setTimeSeqTableDataList()
        elif key_name == 's2S8Mode':
            if self.curr_config:
                self.curr_config['s2S8Mode'] = self.s2s8_cb.currentIndex()
            self.setTimeSeqTableDataList()
            self.show_frame_struct()
        elif key_name == 'tRxDelayTimeUs':
            self.setTimeSeqTableDataList()
        elif key_name == 'bleHeadNum':
            self.show_frame_struct()
        elif key_name == 'dataRate':
            self.reload_s2s8_mode_cb()
            self.set_trx_delay_time()
            self.setTimeSeqTableDataList()
            self.showHideTxDeviation()
        elif key_name == 'txAddress' or self.check_if_ble_mode():
            self.frame_view.label_items['Access Address'].setPlainText('0x{}'.format(self.tx_addr_et.text()))
        elif key_name == 'crc':
            self.set_trx_delay_time()
            self.setTimeSeqTableDataList()
            self.show_frame_struct()
        elif key_name == 'txAddrWidth':
            self.on_focus_out_event('txAddress')
            self.on_focus_out_event('rxAddress')
            self.set_trx_delay_time()
            self.show_frame_struct()
            self.setTimeSeqTableDataList()
            self.set_addr_form_disabled()
            if 'Address' in self.frame_view.label_items:
                self.frame_view.label_items['Address'].setPlainText('{}-byte'.format(self.tx_addr_width_sp.value()))
            elif key_name == 'lengthFilterMode':
                self.check_rx_len_is_show()
            elif key_name == 'whiteListMatchMode':
                self.get_white_list()
            else:
                print('{}:{}'.format(key_name, val))
        if self.curr_config:
            self.curr_config[key_name] = val

    
    def showHideRxTimeOut(self):
        if self.is_longrange:
            hide_rx_timeout = True
        if not 'TIMEOUT' not in self.rx_mode_cb.currentText():
            pass
        hide_rx_timeout = self.is_longrange
        self.rx_timeout_sp.show() if not hide_rx_timeout else self.rx_timeout_sp.hide()
        self.rx_timeout_label.show() if not hide_rx_timeout else self.rx_timeout_label.hide()

    
    def init_enhance_form(self):
        enhance_views = [
            self.auto_delay_sp,
            self.auto_max_cnt_sp,
            self.trx_delay_time_sp,
            self.auto_delay_label,
            self.auto_max_cnt_label,
            self.trx_delay_time_label]
        for enhance_view in enhance_views:
            enhance_view.show() if self.enhance_mode_enable() else enhance_view.hide()

    
    def show_warning_tips(self):
        if self.cb_interface.currentIndex() > 0 and self.spi_clk_cb.currentIndex() > 1:
            self.show_warning_tips_by_content('IIC只支持2M内的速率配置')
        else:
            self.warning_label.hide()

    
    def show_warning_tips_by_content(self, content = (None,)):
        if content:
            self.warning_label.show()
            self.warning_label.setText(content)
        else:
            self.warning_label.hide()

    
    def get_datarate_hz(self):
        dataRate = self.daterate_cb.currentIndex()
        dataRate_int = 1000000
        if dataRate == 1:
            dataRate_int = 2000000
        elif dataRate == 2:
            dataRate_int = 250000
        return dataRate_int

    
    def get_current_chip_mode(self):
        chip_mode_name = self.chip_mode_cb.currentText()
        if chip_mode_name or len(chip_mode_name) == 0:
            return 0
        return None.index(self.chip_mode_cb.currentText())

    
    def enhance_mode_enable(self):
        return self.enhance_cb.currentIndex() > 0

    
    def set_trx_delay_time(self):
        chipMode = self.get_current_chip_mode()
        workMode = self.enhance_cb.currentIndex()
        rxLen = self.rx_len_sp.value()
        spiClk = self.spi_clk_cb.currentIndex()
        dataRate = self.daterate_cb.currentIndex()
        crc = self.crc_cb.currentIndex()
        txAddrWidth = self.tx_addr_width_sp.value()
        txLen = self.tx_len_sp.value()
        spiClkArr = [
            1,
            2,
            4,
            8,
            10]
        spiClk = spiClkArr[spiClk]
        if workMode == 1:
            if self.trx_mode == 1:
                total = 0
                twriteIfo = (rxLen * 8 / spiClk * 1000 * 1000) * 1000 * 1000
                dataRate_int = self.get_datarate_hz()
                if chipMode == 0:
                    total = (((3 + txAddrWidth + rxLen + crc) * 8 + 10) / dataRate_int) * 1 * 1000 * 1000
                elif chipMode == 1:
                    total = (((1 + txAddrWidth + rxLen + crc) * 8 + 9) / dataRate_int) * 1 * 1000 * 1000
                elif chipMode == 2:
                    total = (((1 + txAddrWidth + rxLen + crc) * 8 + 10) / dataRate_int) * 1 * 1000 * 1000
                if self.enhance_mode_enable():
                    rxTimeoutUs = int(100 + twriteIfo + total + 100)
                    self.rx_timeout_sp.setValue(rxTimeoutUs)
                self.rx_timeout_sp.setValue(32000)
                self.trx_delay_time_sp.setValue(0)
            elif self.cb_interface.currentIndex() == 0:
                twriteIfo = (((txLen + 1) * 8 + 132) / spiClk * 1000 * 1000) * 1000 * 1000
            else:
                twriteIfo = (((txLen + 2) * 9 + 132) / spiClk * 1000 * 1000) * 1000 * 1000
            tRxDelayTimeUs = int(twriteIfo * 1.1 if self.cb_interface.currentIndex() == 0 else 1.2)
            if self.enhance_mode_enable():
                rxTimeoutUs = 0
                self.rx_timeout_sp.setValue(rxTimeoutUs)
            self.trx_delay_time_sp.setValue(500)
            self.rx_timeout_sp.setValue(7000)
        else:
            self.rx_timeout_sp.setValue(0)
            self.trx_delay_time_sp.setValue(0)
        if not self.enhance_mode_enable():
            self.rx_timeout_sp.setValue(2000)

    
    def set_ble_channel(self):
        if self.check_if_ble_mode():
            ble_channel = self.ble_channel_cb.currentText()
            if ble_channel == '37':
                self.curr_config['channel'] = 2402 if not self.is_oled else 2483
            elif ble_channel == '38':
                self.curr_config['channel'] = 2426 if not self.is_oled else 2483
            elif ble_channel == '39':
                self.curr_config['channel'] = 2480 if not self.is_oled else 2483
            self.ble_channel_signal.emit(self.curr_config['channel'])

    
    def show_addrs_or_ble_tab(self):
        pass

    
    def on_multi_pipe_change(self, val):
        multi_pipe_addr_widgets = [
            self.pipex_addr4,
            self.pipex_addr3,
            self.pipex_addr2,
            self.pipex_addr1,
            self.pipe1_addr0,
            self.pipe2_addr0,
            self.pipe3_addr0,
            self.pipe4_addr0,
            self.pipe5_addr0]
        for w in multi_pipe_addr_widgets:
            w.setDisabled(not val)
        self.set_addr_form_disabled()

    
    def on_frame_hex_input_change(self, key_name):
        if self.check_if_ble_mode():
            text = self.frame_view.hex_inputs['Length'][0].text()
            if len(text) == 0:
                return None
            if None and key_name == 'Length':
                payload_length = int(text, 16) - 6
                if payload_length > 0:
                    self.frame_view.label_items['AdvD'].setPlainText('{}-byte'.format(payload_length))
                    self.tx_len_sp.setValue(payload_length + 6)
                    self.rx_len_sp.setValue(payload_length + 6)

    
    def show_frame_struct(self):
        work_mode = self.enhance_cb.currentIndex()
        if self.frame_view:
            clear_Layout(self.frame_struct_vlayout)
            self.frame_view = None
        if self.check_if_ble_mode():
            chipMode = self.get_current_chip_mode()
            preamble_length = self.get_preamble_length()
            field_widths = [
                100,
                120,
                100,
                100,
                100,
                100,
                80,
                60,
                60,
                60]
            
            try:
                ble_advd = self.check_advd_content()
                (ble_name_len, ble_name) = self.check_ble_dev_name()
            finally:
                pass
            return None
            advd_length = ble_name_len + 1 + int(len(ble_advd))
            field_labels = [
                ('{}-byte'.format(preamble_length), 'Preamble', 0),
                ('0x8E89BED6', 'Access Address', 0)]
            if self.s2s8_cb.currentIndex() > 0:
                field_labels.append(('{}-byte'.format(2 if self.s2s8_cb.currentIndex() == 1 else 4), 'CI', 0))
                field_labels.append(('{}-byte'.format(3), 'TERM1', 0))

            if int(self.ble_head_num_cb.currentText()) >= 2:
                field_labels.append(('', 'Header0', 1))
            if int(self.ble_head_num_cb.currentText()) >= 3:
                field_labels.append(('', 'Header1', 1))
            field_labels.extend([
                ('', 'Length', 1),
                ('{}-byte'.format(6), 'AdvA', 0),
                ('{}-byte'.format(advd_length), 'AdvD', 0)])
            if self.crc_cb.currentIndex() > 0:
                field_labels.append(('{}-byte'.format(self.crc_cb.currentIndex()), 'CRC', 0))
            if self.s2s8_cb.currentIndex() > 0:
                field_labels.append(('{}-bit'.format(6 if self.s2s8_cb.currentIndex() == 1 else 24), 'TERM2', 0))
            self.frame_view = FrameStructureView(field_labels, field_widths, 40, 10, [
                'Payload',
                'PDU'], [
                (4, 5),
                (2, 5)] if self.s2s8_cb.currentIndex() == 0 else [
                (6, 7),
                (4, 7)], **('field_labels', 'field_widths', 'box_height', 'y_position', 'group_labels', 'group_indices'))
            payload_length = 6 + advd_length
            if 'Header0' in self.frame_view.hex_inputs:
                pass
            self.frame_view.hex_inputs['Header0'][0].setText('42')
            if 'Header1' in self.frame_view.hex_inputs:
                pass
            self.frame_view.hex_inputs['Header1'][0].setText('0')
            self.frame_view.hex_inputs['Length'][0].setText('{:2x}'.format(payload_length))
            self.frame_view.hex_inputs['Length'][0].textChanged.connect(partial(self.on_frame_hex_input_change, 'Length'))
            self.frame_view.hex_inputs['Length'][0].setValidator(HexValidator(37, **('max_value',)))
            self.tx_len_sp.setValue(payload_length)
            self.rx_len_sp.setValue(payload_length)
            self.get_ble_adva()
        else:
            chipMode = self.get_current_chip_mode()
            preamble_length = self.get_preamble_length()
            if self.is_longrange:
                length_label = '1-byte'
            if chipMode == 0:
                length_label = '7-bit'
            elif chipMode == 1:
                length_label = '6-bit'
            elif chipMode == 2:
                length_label = '1-byte'
            field_widths = [
                100,
                100,
                80,
                80,
                80,
                100,
                100]
            field_labels = [
                ('{}-byte'.format(preamble_length), 'Preamble', 0),
                ('{}-byte'.format(self.tx_addr_width_sp.value()), 'Address')]
            if self.is_longrange:
                field_labels.extend([
                    (length_label, 'Length', 0)])
            if self.enhance_mode_enable():
                field_labels.extend([
                    (length_label, 'Length', 0),
                    ('2-bit', 'PID', 0),
                    ('1-bit', 'NoACK', 0)])
            field_labels.append(('{}-byte'.format(self.tx_len_sp.value()), 'Payload', 0))
            if self.crc_cb.currentIndex() > 0:
                field_labels.append(('{}-byte'.format(self.crc_cb.currentIndex()), 'CRC', 0))
            self.frame_view = FrameStructureView(field_labels, field_widths, 40, 10, **('field_labels', 'field_widths', 'box_height', 'y_position'))
            self.get_tx_addrs()
        self.frame_view.setMaximumHeight(120)
        self.frame_struct_vlayout.addWidget(self.frame_view)

    
    def get_preamble_length(self):
        chipMode = self.get_current_chip_mode()
        s2s8Mode = 0
        if self.check_if_ble_mode():
            if s2s8Mode == 1 or s2s8Mode == 2:
                return 10
            if None == 0:
                return 3
            return None

    
    def multi_pipe_ck_click(self, index):
        print(index)
        self.pipex_addr_ck_list[index].switch_active()
        active_cnt = 0
        for i in range(5):
            is_active = self.pipex_addr_ck_list[i].is_active
            if is_active:
                active_cnt += 1
                self.pipex_addr0_et_list[i].show()
                self.pipex_addr0_lb_list[i].show()
                continue
            pcy1 = self.pipex_addr0_et_list[i].sizePolicy()
            pcy2 = self.pipex_addr0_lb_list[i].sizePolicy()
            pcy1.setRetainSizeWhenHidden(True)
            pcy2.setRetainSizeWhenHidden(True)
            self.pipex_addr0_et_list[i].setSizePolicy(pcy1)
            self.pipex_addr0_lb_list[i].setSizePolicy(pcy2)
            self.pipex_addr0_et_list[i].hide()
            self.pipex_addr0_lb_list[i].hide()

    
    def get_tx_power(self):
        curr_text = self.power_spin.currentText()
        return self.get_tx_power_by_text(curr_text)

    
    def get_tx_power_by_text(self, curr_text):
        if curr_text == '0dBm' and self.optmize_0dbm_cb.currentIndex() > 0:
            return 99
        if None == '9.5dBm':
            return 9.5
        return None(curr_text.replace('dBm', ''))

    
    def on_ble_name_change(self, val):
        pass
    # WARNING: Decompyle incomplete

    
    def on_ble_advd_change(self, val):
        if len(val) % 2 != 0:
            return None
        None.on_ble_name_change(None)

    
    def attach_config(self):
        self.curr_config['chipMode'] = self.get_current_chip_mode()
        self.curr_config['workMode'] = self.enhance_cb.currentIndex()
        self.curr_config['channel'] = self.channel_sp.value()
        self.set_ble_channel()
        self.curr_config['txPower'] = self.get_tx_power()
        self.curr_config['dataRate'] = self.daterate_cb.currentIndex()
        self.curr_config['txLen'] = self.tx_len_sp.value()
        self.curr_config['rxLen'] = self.rx_len_sp.value()
        self.curr_config['bleChannel'] = self.ble_channel_cb.currentIndex()
        self.curr_config['crc'] = self.crc_cb.currentIndex()
        self.curr_config['txAddrWidth'] = self.tx_addr_width_sp.value()
        self.curr_config['enTxNoAck'] = self.tx_noack_cb.currentIndex()
        self.curr_config['enWhite'] = self.en_white_cb.currentIndex()
        self.curr_config['crcSkipAddr'] = 0
        self.curr_config['scrSkipAddr'] = 0
        self.curr_config['endian'] = 1
        self.curr_config['enDPL'] = self.en_dpl_cb.currentIndex()
        self.curr_config['tRxDelayTimeUs'] = self.trx_delay_time_sp.value()
        self.curr_config['rxTimeoutUs'] = self.trx_delay_time_sp.value()
        self.curr_config['autoDelayUs'] = self.auto_delay_sp.value()
        self.curr_config['autoMaxCnt'] = self.auto_max_cnt_sp.value()
        self.curr_config['enMultiPipe'] = self.en_multi_pipe_cb.currentIndex()
        self.curr_config['txMode'] = self.tx_mode_cb.currentIndex()
        self.curr_config['rxMode'] = self.rx_mode_cb.currentIndex()
        if self.en_multi_pipe_cb.currentIndex() > 0 and self.trx_mode == 0 and self.enhance_cb.currentIndex() == 0:
            self.curr_config['rxMode'] = 2
        self.curr_config['xtal_freq'] = 32 if self.xtal_freq_cb.currentIndex() else 16
        self.curr_config['XTALFreq'] = 32 if self.xtal_freq_cb.currentIndex() else 16
        self.curr_config['ioMux'] = self.cb_io_enable.currentIndex()
        self.curr_config['interruptMask'] = self.get_selected_interrupt_val()
        self.curr_config['bleHeadNum'] = 2
        whiteList = []
        if self.check_if_ble_mode() or self.is_longrange:
            bleHead0 = self.frame_view.hex_inputs['Header0'][0].text() if 'Header0' in self.frame_view.hex_inputs else '42'
            bleHead0 = '0' if len(bleHead0) == 0 else bleHead0
            bleHead0 = int(bleHead0, 16)
            self.curr_config['bleHead0'] = bleHead0
            bleHead1 = self.frame_view.hex_inputs['Header1'][0].text() if 'Header1' in self.frame_view.hex_inputs else '0'
            bleHead1 = '0' if len(bleHead1) == 0 else bleHead1
            bleHead1 = int(bleHead1, 16)
            self.curr_config['bleHead1'] = bleHead1
            self.curr_config['s2S8Mode'] = self.s2s8_cb.currentIndex()
            self.curr_config['whiteListOffset'] = self.white_list_offset_sp.value()
            self.curr_config['lengthFilterMode'] = self.length_filter_mode_cb.currentIndex()
            self.curr_config['whiteListMatchMode'] = self.white_list_match_mode_cb.currentIndex()
            whiteList = self.get_white_list()
        self.curr_config['advWhiteList'] = whiteList

    
    def get_white_list(self):
        whiteListMatchMode = self.white_list_match_mode_cb.currentIndex()
        if whiteListMatchMode == 0:
            self.adva_white_list_et.setText('')
            self.adva_white_list_et.setDisabled(True)
            return []
        None.adva_white_list_et.setDisabled(False)
        whiteList = self.adva_white_list_et.text()
        if len(whiteList) < whiteListMatchMode * 2:
            self.adva_white_list_et.setText(whiteList + ''.join((lambda .0: [ 'C' for item in .0 ])(range(whiteListMatchMode * 2 - len(whiteList)))))
        else:
            self.adva_white_list_et.setText(whiteList[0:whiteListMatchMode * 2])
        whiteList = self.adva_white_list_et.text()
        whiteList = split_by_two(whiteList)
        whiteList = (lambda .0: for item in .0:
passcontinueint(item, 16)[0])(whiteList)
        return whiteList

    
    def get_rx_address(self):
        LogUtils.write_log_file('get_rx_address:1')
        self.check_rx_addr_signal.emit()
        LogUtils.write_log_file('get_rx_address:2')
        time.sleep(0.15)
        multi_pipe_rx_addrs = [
            self.pipex_addr4.text(),
            self.pipex_addr3.text(),
            self.pipex_addr2.text(),
            self.pipex_addr1.text()]
        multi_pipe_rx_addrs = (lambda .0: [ int(item, 16) for item in .0 ])(multi_pipe_rx_addrs)
        pipex_addr0_values = [
            self.pipe1_addr0.text(),
            self.pipe2_addr0.text(),
            self.pipe3_addr0.text(),
            self.pipe4_addr0.text(),
            self.pipe5_addr0.text()]
        for index, p in enumerate(pipex_addr0_values):
            if len(p) == 0:
                pipex_addr0_values[index] = 'CC'
                continue
                pipex_addr0_values = (lambda .0: [ int(item, 16) for item in .0 ])(pipex_addr0_values)
                single_pipe_rx_addr = self.rx_addr_et.text()
                single_pipe_rx_addr = split_by_two(single_pipe_rx_addr)
                single_pipe_rx_addr = (lambda .0: [ int(item, 16) for item in .0 ])(single_pipe_rx_addr)
                if self.check_if_ble_mode():
                    single_pipe_rx_addr.reverse()
        pipe1_rx_addrs = [
            multi_pipe_rx_addrs + pipex_addr0_values[0:1],
            multi_pipe_rx_addrs + pipex_addr0_values[1:2],
            multi_pipe_rx_addrs + pipex_addr0_values[2:3],
            multi_pipe_rx_addrs + pipex_addr0_values[3:4],
            multi_pipe_rx_addrs + pipex_addr0_values[4:5]]
        for pas in pipe1_rx_addrs:
            pas.reverse()
        rxAddr = []
        enMultiPipe = self.en_multi_pipe_cb.currentIndex()
        if not self.check_if_ble_mode():
            if enMultiPipe == 0:
                rxAddr = [
                    [
                        True,
                        single_pipe_rx_addr],
                    [
                        False,
                        pipe1_rx_addrs[0]],
                    [
                        False,
                        pipe1_rx_addrs[1]],
                    [
                        False,
                        pipe1_rx_addrs[2]],
                    [
                        False,
                        pipe1_rx_addrs[3]],
                    [
                        False,
                        pipe1_rx_addrs[4]]]
            else:
                rx_addr_ck = (lambda .0: [ item.is_active for item in .0 ])(self.pipex_addr_ck_list)
                rxAddr = [
                    [
                        True,
                        single_pipe_rx_addr],
                    [
                        rx_addr_ck[0],
                        pipe1_rx_addrs[0]],
                    [
                        rx_addr_ck[1],
                        pipe1_rx_addrs[1]],
                    [
                        rx_addr_ck[2],
                        pipe1_rx_addrs[2]],
                    [
                        rx_addr_ck[3],
                        pipe1_rx_addrs[3]],
                    [
                        rx_addr_ck[4],
                        pipe1_rx_addrs[4]]]
        else:
            rxAddr = [
                [
                    True,
                    single_pipe_rx_addr],
                [
                    False,
                    pipe1_rx_addrs[0]],
                [
                    False,
                    pipe1_rx_addrs[1]],
                [
                    False,
                    pipe1_rx_addrs[2]],
                [
                    False,
                    pipe1_rx_addrs[3]],
                [
                    False,
                    pipe1_rx_addrs[4]]]
        rxAddrInt = copy.deepcopy(rxAddr)
        for item_list in rxAddr:
            for items in item_list:
                if isinstance(items, list):
                    for index, item in enumerate(items):
                        items[index] = '0x{:02X}'.format(item)
                    continue
                    continue
                    return (rxAddr, rxAddrInt)

    
    def parse_easy_rf_code(self):
        is_pms = self.is_pms
        self.base_freq = self.base_freq_cb.currentText()
        self.check_rx_addr_signal.emit()
        time.sleep(0.1)
        self.attach_config()
    # WARNING: Decompyle incomplete

    
    def show_preview_dialog(self):
        (content, req_content) = self.parse_easy_rf_code()
        self.codeViewDialog.set_content('c', content, self.is_pms, self.base_freq)
        self.codeViewDialog.show_dialog()

    
    def check_addrs(self):
        all_addr_widgets = [
            self.pipex_addr4,
            self.pipex_addr3,
            self.pipex_addr2,
            self.pipex_addr1]
        for w in all_addr_widgets:
            if w.text() == '':
                w.setText('CC')
                continue
                return None

    
    def gen_code_thread(self):
        is_select_empty = True
        export_path = QFileDialog.getExistingDirectory(self, 'Select Directory')
        if not export_path:
            return None
        self.is_oled = None
        self.start_gen_thread = threading.Thread(partial(self.gen_code, export_path, is_select_empty), **('target',))
        self.start_gen_thread.setDaemon(True)
        self.start_gen_thread.start()

    
    def start_progress_thread(self):
        self.progress_thread = threading.Thread(self.start_fake_dfu_progress, **('target',))
        self.progress_thread.setDaemon(True)
        self.progress_thread.start()

    
    def start_fake_dfu_progress(self):
        self.is_progress_running = True
        if self.is_progress_running:
            v = self.dfu_dev_status_pb.value()
            v += 1
            v = min(99, v)
            self.dfu_progress_signal.emit(v)
            time.sleep(0.16)
            continue

    
    def build_download_thread(self, is_oled):
        self.is_oled = is_oled
        if self.is_oled:
            self.show_dev_dfu_progress(True)
        self.dl_program_btn_signal.emit(True)
        LogUtils.write_log_file('build_download_thread')
        self.start_build_thread = threading.Thread(partial(self.start_build_download, '.\\.cache\\', True), **('target',))
        self.start_build_thread.setDaemon(True)
        self.start_build_thread.start()

    
    def start_build_download(self, export_path, is_select_empty):
        target_template_path = None
    # WARNING: Decompyle incomplete

    
    def get_select_dev_arr(self, hid_dev_list, hid_uuid = (None, None)):
        filter_devs = []
        for dev in self.hid_dev_list:
            if hid_uuid or dev == self.hid_select_uuid:
                filter_devs.append(self.hid_dev_list[dev])
                continue
                if dev == hid_uuid:
                    filter_devs.append(self.hid_dev_list[dev])
                    continue
                    return filter_devs

    
    def get_hid_vendor_uuid(self):
        cmd_datas = self.dfu_usb_com.getVendorUuidCmd(0)
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        if result[0]:
            vendor_uuid = (lambda .0: [ '{:02x}'.format(item) for item in .0 ])(result[1])
            vendor_uuid = ''.join(vendor_uuid)
            vendor_uuid = vendor_uuid[4:20]
            vendor_uuid = vendor_uuid.upper()
            self.usb_ctrl.list_ctrl.set_vendor_uuid(vendor_uuid)
            return vendor_uuid

    
    def get_select_devs(self, hid_uuid, dev_state = (None, None)):
        max_times = 0
        filter_devs = []
        hid_uuid = hid_uuid if hid_uuid and len(hid_uuid) > 0 else self.hid_select_uuid
        self.usb_ctrl.list_ctrl.findDevice()
        devs = self.usb_ctrl.list_ctrl.hid_dev_list
        find_result = False
    # WARNING: Decompyle incomplete

    
    def dfu_dl_firmware(self, bin_path, hid_uuid = (None,)):
        pass
    # WARNING: Decompyle incomplete

    
    def rebootDevice(self):
        cmd_datas = self.dfu_usb_com.getDFUEndCmd()
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        print('rebootDevice:{}'.format(result))
        app_jiangpu_ready = False
        if self.dfu_usb_com.checkDFUEndReport(result[1]):
            app_jiangpu_ready = True
        return app_jiangpu_ready

    
    def setDeviceDfuMode(self, dfu_en):
        enable_value = 0 if dfu_en else 1
        ret = False
        cmd_datas = self.dfu_usb_com.getSetMouseEnableCmd(enable_value)
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        if not result[0] == 1:
            raise AssertionError('Enter Dfu Mode Fail.')
        if None.dfu_usb_com.checkSetMouseEnable(result[1], enable_value):
            ret = True
        return ret

    
    def setDeviceToUpdatedMode(self):
        '''
        设置设备跳转到强制升级模式
        '''
        (ret, mode) = (False, 255)
        cmd_datas = self.dfu_usb_com.getSetDFUUpdatedMode(False)
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        if result[0] == 1:
            ck_ret = self.dfu_usb_com.checkSetInDFUUpdateModeReport(result[1])
            ret = ck_ret[0]
            if ret:
                mode = ck_ret[1]
                if mode == 255:
                    self.usb_ctrl.close_device()
        self.setResultView(ret)
        return [
            ret,
            mode]

    
    def checkDeviceVendorUuid(self):
        cmd_datas = self.dfu_usb_com.getVendorUuidCmd(0)
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        if result[0] == 1:
            vendor_uuid = (lambda .0: [ '{:02x}'.format(item) for item in .0 ])(result[1])
            vendor_uuid = ''.join(vendor_uuid)
            return [
                1 if vendor_uuid == self.usb_ctrl.list_ctrl.get_vendor_uuid() else 0]
        return [
            None]

    
    def waitAndCheckBootLoaderRun(self, view, timeout = (0, 10)):
        ret = False
        st_time = time.time()
        self.usb_ctrl.list_ctrl.findDevice()
        new_dev_paths = self.usb_ctrl.list_ctrl.getDevicePaths()
        for dev_path in new_dev_paths:
            result = self.usb_ctrl.dev_ctrl.open(dev_path)
            if result:
                result = self.checkDeviceVendorUuid()
                if result[0] > 0:
                    self.usb_ctrl.dev_ctrl.clearReadBuffer(0.05)
                    ret = True
                else:
                    self.usb_ctrl.dev_ctrl.close()
                if ret:
                    pass
                else:
                    time.sleep(0.2)
                return ret

    
    def waitDeviceRestart(self):
        self.usb_ctrl.close_device()
        ret = False
        t = 0
        self.usb_ctrl.list_ctrl.findDevice()
        new_dev_paths = self.usb_ctrl.list_ctrl.getDevicePaths()
        for dev_path in new_dev_paths:
            result = self.usb_ctrl.dev_ctrl.open(dev_path)
            if result:
                result = self.checkDeviceConnect()
                if result[0] > 0:
                    self.usb_ctrl.dev_ctrl.clearReadBuffer(0.05)
                    ret = True
                else:
                    self.usb_ctrl.dev_ctrl.close()
                p = (t / 20) * 9
                p = int(91 + p)
                p = min(100, p)
                self.progressUpdatedView(p, False)
                if not ret:
                    if t >= 20:
                        pass
                    else:
                        t += 1
                        time.sleep(0.2)
        if ret:
            self.progressUpdatedView(100, False)
        return ret

    
    def __pan108xZdk1_2_0(self):
        ''' PAN108x ZDK V1.2.0 以上版本 DFU 升级 '''
        (ret, in_updated_mode, in_dfu, reboot) = (True, False, False, False)
        if self.dfu_program.pg_mode == 1:
            ck_ret = self.setDeviceToUpdatedMode()
            ret = ck_ret[0]
            if ret:
                in_updated_mode = True
                ret = self.waitAndCheckBootLoaderRun(1)
        LogUtils.write_log_file('setDeviceToUpdatedMode:{}'.format('成功' if ret else '失败'))
        if ret:
            ret = self.setDeviceDfuMode(True)
            if not ret > 0:
                raise AssertionError('Enter DFU Mode Fail.')
        None.write_log_file('setDeviceDfuMode:{}'.format('成功' if ret else '失败'))
        if ret:
            (addr, version) = self.dfu_program.getHeadInfo()
            (size, crc32) = self.dfu_program.getProgramInfo()
            ret = self.__dealCheckDeviceInfo(addr, version, size, crc32)
            LogUtils.write_log_file('__dealCheckDeviceInfo:{}'.format('成功' if ret else '失败'))
            if ret:
                self.progressUpdatedView(3, True)
        if ret:
            cmd_datas = self.dfu_usb_com.getDFUStartCmd()
            result = self.usb_ctrl.send_and_read(cmd_datas, 200)
            if not self.dfu_usb_com.checkDFUStartReport(result[1]):
                raise AssertionError('DFU start check fail')
            None.write_log_file('__dealCheckDeviceInfo:{}'.format('成功' if ret else '失败'))
            result = self.usb_ctrl.dev_ctrl.read_once_package(3000)
            if not result[0] == 1:
                raise AssertionError('DFU start check fail.')
            if not None.dfu_usb_com.checkDFUStartResult(result[1]):
                raise AssertionError('DFU start check fail.')
            if None:
                self.progressUpdatedView(8, True)
        if ret:
            (ret, index) = (False, 0)
            size = self.dfu_program.file_datas_size
            pg_datas = self.dfu_program.file_datas
            (temp_buff_count, temp_percent, buff_size) = (0, 0, 256)
            if index < size:
                num = DfuUsbCom.DATAS_MAX_LEN
                if index + num > size:
                    num = size - index
                datas = list(pg_datas[index:index + num])
                cmd_datas = self.dfu_usb_com.getDFUTransferCmd(datas)
                ret = self.usb_ctrl.dev_ctrl.send_datas(cmd_datas)
                if ret:
                    LogUtils.write_log_file('切片传输结果:{}'.format('成功' if ret else '失败'))
                    index += num
                    count = int(index / buff_size)
                    if count > temp_buff_count or index == size:
                        if index == size and count == temp_buff_count:
                            count += 1
                        temp_buff_count = count
                        percent = int(index * 90 / size)
                        if percent > temp_percent:
                            self.progressUpdatedView(percent, True)
                            temp_percent = percent
                        result = self.usb_ctrl.dev_ctrl.read_once_package(1000)
                        LogUtils.write_log_file('切片上传返回:{}'.format('成功' if result[0] else '失败'))
                        if result[0] == 1:
                            ret = self.dfu_usb_com.checkDFUTransferReport(result[1], count - 1)
                            LogUtils.write_log_file('checkDFUTransferReport:{}'.format('成功' if ret else '失败'))
                            if ret and buff_size == DfuUsbCom.DATAS_MAX_LEN:
                                ret = self.dfu_usb_com.checkDFUTransferResult(result[1], datas)
                                LogUtils.write_log_file('checkDFUTransferResult:{}'.format('成功' if ret else '失败'))
                            if not ret:
                                pass
                            
                        else:
                            ret = False
                    
                else:
                    time.sleep(0.002)
            
        
        if not ret > 0:
            raise AssertionError('Download Program File Fail.')
        if None:
            LogUtils.write_log_file('Start DFU Finish')
            cmd_datas = self.dfu_usb_com.getDFUFinishCmd()
            result = self.usb_ctrl.send_and_read(cmd_datas, 200)
            LogUtils.write_log_file('检查DFU Finish 返回码:{}'.format('成功' if ret else '失败'))
            if result[0] == 1:
                if self.dfu_usb_com.checkDFUFinishReport(result[1]):
                    ret = True
                if ret:
                    result = self.usb_ctrl.dev_ctrl.read_once_package(1000)
                    LogUtils.write_log_file('检查DFU Finish其他格式:{}'.format('成功' if ret else '失败'))
                    if result[0] == 1:
                        result = self.dfu_usb_com.checkDFUFinishResult(result[1])
                        ret = result[0]
                    else:
                        ret = False
            if ret:
                self.progressUpdatedView(91, True)
        if ret:
            self.is_rebooting = True
            LogUtils.write_log_file('下载成功，开始rebootDevice')
            reboot = True
            ret = self.rebootDevice()
            LogUtils.write_log_file('rebootDevice:{}'.format('成功' if ret else '失败'))
            if ret:
                self.dfu_progress_signal.emit(100)
                self.dfu_progress_msg_signal.emit('Download Success.', MSG_SUCCESS)
            self.is_rebooting = False
        elif in_dfu:
            ret = self.setDeviceDfuMode(False)
            if not ret > 0:
                raise AssertionError('Exit DFU Mode Fail.')
            if None.dfu_program.pg_mode == 1 and in_updated_mode:
                if not reboot:
                    self.rebootDevice()
                    self.usb_ctrl.close_device()
                if ret:
                    out_ret = True
                else:
                    out_ret = self.waitAndCheckBootLoaderRun(2)
                LogUtils.write_log_file('boot启动结果:{}'.format('成功' if out_ret else '失败'))
                if out_ret:
                    out_ret = self.setDeviceOutUpdatedMode()
                if ret:
                    ret = out_ret
        return ret

    
    def setDeviceOutUpdatedMode(self):
        ret = False
        cmd_datas = self.dfu_usb_com.getSetDFUUpdatedMode(True)
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        if result[0] == 1 and self.dfu_usb_com.checkSetDFUUpdatedModeReport(result[1]):
            ret = True
        return ret

    
    def __dealCheckDeviceInfo(self, addr, version, size, crc32):
        cmd_datas = self.dfu_usb_com.getCheckVersionCmd(addr, version, size, crc32)
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        if result[0] == 1:
            return self.dfu_usb_com.checkCheckVersionReport(result[1])
        return [
            None]

    
    def progressUpdatedView(self, progress, is_finished):
        self.dfu_progress_signal.emit(progress)

    
    def openDeviceAndCheck(self):
        self.usb_device_open = self.openOrCloseUsbDevice(True)
        if self.usb_device_open:
            result = self.checkDeviceCom()
            if result[0]:
                if result[1] or self.rebootDevice():
                    self.usb_ctrl.close_device()
                    return self.waitAndCheckBootLoaderRun()
                return True
            return None

    
    def checkDeviceCom(self):
        '''
        尝试与设备进行通信，检查设备
        返回值：
            [ret, boot]
            ret: True 检测连接成功 False 检测连接失败
            boot: ret==True 有效， True 为需要跳转到 bootloader 运行
        '''
        ret = [
            False]
        result = self.checkDeviceConnect()
        if result[0] == 1:
            ret[0] = True
            ret += [
                result[1]]
        elif result[0] == -1:
            pass
        elif result[0] == -2:
            pass
        return ret

    
    def checkDeviceConnect(self):
        '''
        尝试与设备进行通信，检查设备
        返回值：
            [ret, boot]
            ret: 检测结果
                1 检测连接成功
                0 检测连接失败
                -1 设备无回复
                -2 未知设备
            boot: ret==1 有效， True 为需要跳转到 bootloader 运行
        '''
        cmd_datas = self.dfu_usb_com.getConnectCmd()
        result = self.usb_ctrl.send_and_read(cmd_datas, 200)
        if result[0] == 1:
            result = self.dfu_usb_com.checkConnectReport(result[1])
            if result[0]:
                return [
                    1,
                    result[1]]
            return [
                None]
        return [
            None[0]]

    
    def openOrCloseUsbDevice(self, con):
        ret = True
        if con:
            for i in range(2):
                ret = self.usb_ctrl.open_select_device()
                if ret:
                    result = self.checkDeviceConnect()
                    if result[0] != 1:
                        ret = False
                    
                elif not ret:
                    first = True
                    result = self.usb_ctrl.open_device(first)
                    if result[0] or result[1]:
                        idx = result[2]
                        result = self.checkDeviceConnect()
                        if result[0] == 1:
                            ret = True
                            continue
                        else:
                            self.usb_ctrl.close_device()
                first = False
        else:
            self.usb_ctrl.close_device()
        return ret

    
    def get_template_zip_file_name(self):
        if STATE_211_MODE:
            return 'SDK_Template_211_2.zip'
        return None

    
    def get_select_power_table(self):
        select_text_list = self.power_table_widget.currentTextList()
        power_table = []
        for text in select_text_list:
            power_table.append(get_tx_power_defined(self.get_tx_power_by_text(text)))
        return power_table

    
    def showHideTxDeviation(self):
        if self.chip_mode_cb.currentIndex() == 0 and self.daterate_cb.currentIndex() == 0 and self.password_checked:
            self.tx_deviation_cb.show()
            self.tx_deviation_label.show()
        else:
            self.tx_deviation_cb.hide()
            self.tx_deviation_label.hide()

    
    def get_tx_addrs(self):
        txAddr = self.tx_addr_et.text()
        txAddr = split_by_two(txAddr)
        if self.check_if_ble_mode():
            txAddr.reverse()
        return (lambda .0: [ '0x{}'.format(item) for item in .0 ])(txAddr)

    
    def get_ble_adva(self):
        txAddr = []
        advaList = self.adva_et.text()
        if len(advaList) < 12:
            self.adva_et.setText(advaList + ''.join((lambda .0: [ 'C' for item in .0 ])(range(12 - len(advaList)))))
        advaList = self.adva_et.text()
        advaList = split_by_two(advaList)
        advaList = (lambda .0: [ '0x' + item for item in .0 ])(advaList)
        return advaList

    
    def save_pan211_config_file(self, folder = ('.\\.cache',)):
        if not Path(folder).exists():
            Path(folder).mkdir()
        config_file_path = folder + '\\pan211_config.json'
        txAddr = self.get_tx_addrs()
        xtal_freq = 32 if self.xtal_freq_cb.currentIndex() else 16
        (rxAddr, rxAddrInt) = self.get_rx_address()
        powerTableList = self.get_select_power_table()
        s2s8_mode = self.s2s8_cb.currentIndex()
        endian = 1
        config_json = {
            'CONFIG_TYPE': 'BLE' if self.check_if_ble_mode() else 'RF',
            'Channel': self.curr_config['channel'] - 2400,
            'TxPower': get_tx_power_defined(self.get_tx_power()),
            'DataRate': get_datarate_defined(self.daterate_cb.currentIndex()),
            'ChipMode': get_chip_mode_defined(self.get_current_chip_mode()),
            'EnWhite': self.en_white_cb.currentIndex(),
            'Crc': get_crc_defined(self.crc_cb.currentIndex()),
            'TxLen': self.tx_len_sp.value(),
            'RxLen': self.rx_len_sp.value(),
            'AddrWidth': self.tx_addr_width_sp.value(),
            'TxAddr': txAddr,
            'RxAddrWidth': self.tx_addr_width_sp.value(),
            'RxAddr': rxAddr,
            'Endian': get_endian_defined(endian),
            'crcSkipAddr': 0,
            'EnRxPlLenLimit': 0,
            'WorkMode': get_work_mode_defined(self.enhance_cb.currentIndex()),
            'EnDPL': self.en_dpl_cb.currentIndex(),
            'EnTxNoAck': self.tx_noack_cb.currentIndex(),
            'EnManuPid': 0,
            'TRxDelayTimeUs': self.trx_delay_time_sp.value(),
            'RxTimeoutUs': self.rx_timeout_sp.value(),
            'AutoDelayUs': self.auto_delay_sp.value(),
            'AutoMaxCnt': self.auto_max_cnt_sp.value(),
            'TxMode': get_tx_mode_defined(self.tx_mode_cb.currentIndex()),
            'RxMode': get_rx_mode_defined(self.curr_config['rxMode'], self.enhance_cb.currentIndex(), self.is_longrange, **('is_longrange',)),
            'EASY_RF': '1' if not self.is_longrange else '0',
            'InterruptMask': self.get_selected_interrupt_val(),
            'IOMUX_EN': self.cb_io_enable.currentIndex(),
            'XTAL_FREQ': get_xtal_freq_defined(xtal_freq),
            'ENABLE_FS_MODE': 0 if STATE_211_MODE else 1,
            'INTERFACE_MODE': get_interface_defined(self.cb_interface.currentIndex()) if not self.is_pms else 'USE_I2C',
            'PowerTable': powerTableList,
            'TxDevSelect': self.tx_deviation_cb.currentText().replace('Hz', ''),
            'RxGain': self.rx_gain_cb.currentIndex() if not self.is_pms else 1,
            'BLEHeadNum': int(self.ble_head_num_cb.currentText()),
            'BLEHead0': hex_to_int(self.curr_config.get('bleHead0')),
            'BLEHead1': hex_to_int(self.curr_config.get('bleHead1')),
            'S2S8Mode': get_s2s8_mode_defined(s2s8_mode),
            'WhiteInit': get_ble_white_init_defined(self.ble_channel_cb.currentIndex()),
            'WhiteListMatchMode': get_ble_white_list_match_mode_defined(self.white_list_match_mode_cb.currentIndex()),
            'WhiteListOffset': self.white_list_offset_sp.value(),
            'WhiteListLen': self.white_list_match_mode_cb.currentIndex(),
            'WhiteList': self.curr_config.get('advWhiteList'),
            'LengthFilterMode': get_ble_len_filter_mode_defined(self.length_filter_mode_cb.currentIndex()),
            'EN_AGC': self.en_agc_cb.currentIndex() }
        if Path(config_file_path).exists():
            Path(config_file_path).unlink()
        FilesUtil.write_json_file(config_json, config_file_path)
        return config_file_path

    
    def gen_code(self, export_path, is_select_empty, need_feedback = (True,)):
        pass
    # WARNING: Decompyle incomplete

    
    def check_advd_content(self):
        self.show_warning_tips_by_content()
        ble_advd = self.ble_advd_et.text()
        if not len(ble_advd) > 0:
            raise AssertionError('AdvD内容未填写')
        if not None(ble_advd) % 2 == 0:
            raise AssertionError('AdvD填写内容长度必须是偶数')
        ble_advd = None(ble_advd)
        manu_data_len = '{:02X}'.format(len(ble_advd) + 1)
        ble_advd_arr = []
        ble_advd_arr.append(manu_data_len)
        ble_advd_arr.append('FF')
        ble_advd_arr.extend(ble_advd)
        return ble_advd_arr

    
    def check_ble_dev_name(self):
        ble_name = self.ble_name_et.text()
        if not len(ble_name) > 0:
            raise AssertionError('Device Name未填写')
        ble_name_hex = None(ble_name)
        name_list = (lambda .0: [ "'{}'".format(item) for item in .0 ])(ble_name)
        return (int(len(ble_name_hex) / 2) + 1, ', '.join(name_list))

    __classcell__ = None

if __name__ == '__main__':
    QApplication.setAttribute(Qt.AA_EnableHighDpiScaling)
    QApplication.setHighDpiScaleFactorRoundingPolicy(Qt.HighDpiScaleFactorRoundingPolicy.PassThrough)
    font = QApplication.font()
    font.setPointSize(9)
    font.setFamily('Times New Roman')
    QApplication.setFont(font)
    app = QApplication(sys.argv)
    gol.init(app)
    myWin = MyWindow()
    myWin.setWindowIcon(QIcon(':/images/favicon.ico'))
    updater = SW_Updater()
    ver = updater.versionInfo.calcVersionStr()
    version = Version()
    v = '{} V{}'.format(version.name, ver)
    myWin.setWindowTitle(v)
    myWin.show()
    sys.exit(app.exec_())
