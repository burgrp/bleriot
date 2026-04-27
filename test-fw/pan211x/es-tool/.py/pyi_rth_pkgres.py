# Source Generated with Decompyle++
# File: pyi_rth_pkgres.pyc (Python 3.8)


def _pyi_rthook():
    import os
    import pathlib
    import sys
    import pkg_resources
    import pyimod02_importers
    SYS_PREFIX = pathlib.PurePath(sys._MEIPASS)
    
    def _TocFilesystem():
        '''_pyi_rthook.<locals>._TocFilesystem'''
        __qualname__ = '_pyi_rthook.<locals>._TocFilesystem'
        __doc__ = '\n        A prefix tree implementation for embedded filesystem reconstruction.\n\n        NOTE: as of PyInstaller 6.0, the embedded PYZ archive cannot contain data files anymore. Instead, it contains\n        only .pyc modules - which are by design not returned by `PyiFrozenProvider`. So this implementation has been\n        reduced to supporting only directories implied by collected packages.\n        '
        
        def __init__(self, tree_node):
            self._tree = tree_node

        
        def _get_tree_node(self = None, path = None):
            path = pathlib.PurePath(path)
            current = self._tree
            for component in path.parts:
                if component not in current:
                    return None
                current = None[component]
            return current

        
        def path_exists(self, path):
            node = self._get_tree_node(path)
            return isinstance(node, dict)

        
        def path_isdir(self, path):
            node = self._get_tree_node(path)
            return isinstance(node, dict)

        
        def path_listdir(self, path):
            node = self._get_tree_node(path)
            if not isinstance(node, dict):
                return []
            return (lambda 