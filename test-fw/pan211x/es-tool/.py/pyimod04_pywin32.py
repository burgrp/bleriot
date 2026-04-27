# Source Generated with Decompyle++
# File: pyimod04_pywin32.pyc (Python 3.8)

'''
Set search path for pywin32 DLLs. Due to the large number of pywin32 modules, we use a single loader-level script
instead of per-module runtime hook scripts.
'''
import os
import sys

def install():
    pywin32_ext_paths = ('win32', 'pythonwin')
    pywin32_ext_paths = (lambda .0: [ os.path.join(sys._MEIPASS, pywin32_ext_path) for pywin32_ext_path in .0 ])(pywin32_ext_paths)
    pywin32_ext_paths = (lambda .0: [ path for path in .0 if os.path.isdir(path) ])(pywin32_ext_paths)
    sys.path.extend(pywin32_ext_paths)
    pywin32_system32_path = os.path.join(sys._MEIPASS, 'pywin32_system32')
    if not os.path.isdir(pywin32_system32_path):
        return None
    None.path.append(pywin32_system32_path)
    os.add_dll_directory(pywin32_system32_path)
    path = os.environ.get('PATH', None)
    if not path:
        path = pywin32_system32_path
    else:
        path = pywin32_system32_path + os.pathsep + path
    os.environ['PATH'] = path

