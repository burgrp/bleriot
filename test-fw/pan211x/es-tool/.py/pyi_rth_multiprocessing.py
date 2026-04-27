# Source Generated with Decompyle++
# File: pyi_rth_multiprocessing.pyc (Python 3.8)


def _pyi_rthook():
    import sys
    import multiprocessing
    import multiprocessing.spawn as multiprocessing
    _args_from_interpreter_flags = _args_from_interpreter_flags
    import subprocess
    multiprocessing.process.ORIGINAL_DIR = None
    
    def _freeze_support():
        if len(sys.argv) >= 2 and sys.argv[-2] == '-c' and sys.argv[-1].startswith(('from multiprocessing.resource_tracker import main', 'from multiprocessing.forkserver import main')) and set(sys.argv[1:-2]) == set(_args_from_interpreter_flags()):
            exec(sys.argv[-1])
            sys.exit()
    # WARNING: Decompyle incomplete

    multiprocessing.freeze_support = multiprocessing.spawn.freeze_support = _freeze_support

_pyi_rthook()
del _pyi_rthook
