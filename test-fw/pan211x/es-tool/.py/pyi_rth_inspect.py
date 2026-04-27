# Source Generated with Decompyle++
# File: pyi_rth_inspect.pyc (Python 3.8)


def _pyi_rthook():
    import inspect
    import os
    import sys
    SYS_PREFIX = os.path.normpath(sys._MEIPASS)
    _orig_inspect_getsourcefile = inspect.getsourcefile
    
    def _pyi_getsourcefile(object = None):
        filename = inspect.getfile(object)
        filename = os.path.normpath(filename)
        if not os.path.isabs(filename):
            main_file = getattr(sys.modules['__main__'], '__file__', None)
            if main_file and filename == os.path.basename(main_file):
                return main_file
            if None.endswith('.py'):
                filename = os.path.normpath(os.path.join(SYS_PREFIX, filename + 'c'))
                if filename.startswith(SYS_PREFIX):
                    return filename
                if filename.startswith(SYS_PREFIX) and filename.endswith('.pyc'):
                    return filename
                return None(object)

    inspect.getsourcefile = _pyi_getsourcefile

_pyi_rthook()
del _pyi_rthook
