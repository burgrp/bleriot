# Source Generated with Decompyle++
# File: pyi_rth_pkgutil.pyc (Python 3.8)


def _pyi_rthook():
    import pkgutil
    import pyimod02_importers
    
    def _iter_pyi_frozen_file_finder_modules(finder = None, prefix = None):
        pyz_toc_tree = pyimod02_importers.get_pyz_toc_tree()
        if finder._pyz_entry_prefix:
            pkg_name_parts = finder._pyz_entry_prefix.split('.')
        else:
            pkg_name_parts = []
        tree_node = pyz_toc_tree
        for pkg_name_part in pkg_name_parts:
            tree_node = tree_node.get(pkg_name_part)
            if not isinstance(tree_node, dict):
                tree_node = { }
            
            for entry_name, entry_data in tree_node.items():
                is_pkg = isinstance(entry_data, dict)
                yield (prefix + entry_name, is_pkg)
            if finder.fallback_finder is not None:
                yield from pkgutil.iter_importer_modules(finder.fallback_finder, prefix)

    pkgutil.iter_importer_modules.register(pyimod02_importers.PyiFrozenImporter, _iter_pyi_frozen_file_finder_modules)

_pyi_rthook()
del _pyi_rthook
