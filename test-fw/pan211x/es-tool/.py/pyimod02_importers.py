# Source Generated with Decompyle++
# File: pyimod02_importers.pyc (Python 3.8)

'''
PEP-302 and PEP-451 importers for frozen applications.
'''
import sys
import os
import io
import _frozen_importlib
import _thread
import pyimod01_archive
if sys.flags.verbose and sys.stderr:
    
    def trace(msg, *a):
        sys.stderr.write(msg % a)
        sys.stderr.write('\n')

else:
    
    def trace(msg, *a):
        pass


def _decode_source(source_bytes):
    """
    Decode bytes representing source code and return the string. Universal newline support is used in the decoding.
    Based on CPython's implementation of the same functionality:
    https://github.com/python/cpython/blob/3.9/Lib/importlib/_bootstrap_external.py#L679-L688
    """
    detect_encoding = detect_encoding
    import tokenize
    source_bytes_readline = io.BytesIO(source_bytes).readline
    encoding = detect_encoding(source_bytes_readline)
    newline_decoder = io.IncrementalNewlineDecoder(None, True, **('decoder', 'translate'))
    return newline_decoder.decode(source_bytes.decode(encoding[0]))

pyz_archive = None
_pyz_tree_lock = _thread.RLock()
_pyz_tree = None

def get_pyz_toc_tree():
    global _pyz_tree
    pass
# WARNING: Decompyle incomplete

_RESOLVED_TOP_LEVEL_DIRECTORY = os.path.realpath(sys._MEIPASS)
_is_macos_app_bundle = False
if sys.platform == 'darwin' and _RESOLVED_TOP_LEVEL_DIRECTORY.endswith('Contents/Frameworks'):
    _is_macos_app_bundle = True
    _ALTERNATIVE_TOP_LEVEL_DIRECTORY = os.path.join(os.path.dirname(_RESOLVED_TOP_LEVEL_DIRECTORY), 'Resources')

def _build_pyz_prefix_tree(pyz_archive):
    tree = dict()
    for entry_name, entry_data in pyz_archive.toc.items():
        name_components = entry_name.split('.')
        typecode = entry_data[0]
        current = tree
        if typecode in {
            pyimod01_archive.PYZ_ITEM_PKG,
            pyimod01_archive.PYZ_ITEM_NSPKG}:
            for name_component in name_components:
                current = current.setdefault(name_component, { })
            continue
        for name_component in name_components[:-1]:
            current = current.setdefault(name_component, { })
        current[name_components[-1]] = ''
    return tree


class PyiFrozenImporter:
    '''
    PyInstaller\'s frozen module importer (finder + loader) for specific search path.

    Per-path instances allow us to properly translate the given module name ("fullname") into full PYZ entry name.
    For example, with search path being `sys._MEIPASS`, the module "mypackage.mod" would translate to "mypackage.mod"
    in the PYZ archive. However, if search path was `sys._MEIPASS/myotherpackage/_vendored` (for example, if
    `myotherpacakge` added this path to `sys.path`), then "mypackage.mod" would need to translate to
    "myotherpackage._vendored.mypackage.mod" in the PYZ archive.
    '''
    
    def __repr__(self):
        return f'''{self.__class__.__name__}({self._path})'''

    
    def path_hook(cls, path):
        trace(f'''PyInstaller: running path finder hook for path: {path!r}''')
    # WARNING: Decompyle incomplete

    path_hook = classmethod(path_hook)
    
    def _compute_relative_path(path, top_level):
        pass
    # WARNING: Decompyle incomplete

    _compute_relative_path = staticmethod(_compute_relative_path)
    
    def __init__(self, path):
        self._path = path
        self._pyz_archive = pyz_archive
        resolved_path = os.path.realpath(path)
        
        try:
            relative_path = self._compute_relative_path(resolved_path, _RESOLVED_TOP_LEVEL_DIRECTORY)
        finally:
            pass
        except Exception:
            if _is_macos_app_bundle:
                relative_path = self._compute_relative_path(resolved_path, _ALTERNATIVE_TOP_LEVEL_DIRECTORY)
            else:
                raise 
        

        if os.path.isfile(path):
            raise ImportError('only directories are supported')
        if None == '.':
            self._pyz_entry_prefix = ''
        else:
            self._pyz_entry_prefix = '.'.join(relative_path.split(os.path.sep))

    
    def _compute_pyz_entry_name(self, fullname):
        """
        Convert module fullname into PYZ entry name, subject to the prefix implied by this finder's search path.
        """
        tail_module = fullname.rpartition('.')[2]
        if self._pyz_entry_prefix:
            return self._pyz_entry_prefix + '.' + tail_module
        return None

    
    def fallback_finder(self):
        '''
        Opportunistically create a *fallback finder* using `sys.path_hooks` entries that are located *after* our hook.
        The main goal of this exercise is to obtain an instance of python\'s FileFinder, but in theory any other hook
        that comes after ours is eligible to be a fallback.

        Having this fallback allows our finder to "cooperate" with python\'s FileFinder, as if the two were a single
        finder, which allows us to work around the python\'s PathFinder permitting only one finder instance per path
        without subclassing FileFinder.
        '''
        if hasattr(self, '_fallback_finder'):
            return self._fallback_finder
        our_hook_found = None
        self._fallback_finder = None
        for idx, hook in enumerate(sys.path_hooks):
            if hook == self.path_hook:
                our_hook_found = True
                continue
            if not our_hook_found:
                continue
            
            try:
                self._fallback_finder = hook(self._path)
            finally:
                pass
            continue
            except ImportError:
                continue
            
            return self._fallback_finder


    fallback_finder = property(fallback_finder)
    
    def _find_fallback_spec(self, fullname, target):
        '''
        Attempt to find the spec using fallback finder, which is opportunistically created here. Typically, this would
        be python\'s FileFinder, which can discover specs for on-filesystem modules, such as extension modules and
        modules that are collected only as source .py files.

        Having this fallback allows our finder to "cooperate" with python\'s FileFinder, as if the two were a single
        finder, which allows us to work around the python\'s PathFinder permitting only one finder instance per path
        without subclassing FileFinder.
        '''
        if not hasattr(self, '_fallback_finder'):
            self._fallback_finder = self._get_fallback_finder()
        if self._fallback_finder is None:
            return None
        return None._fallback_finder.find_spec(fullname, target)

    
    def invalidate_caches(self):
        '''
        A method which, when called, should invalidate any internal cache used by the finder. Used by
        importlib.invalidate_caches() when invalidating the caches of all finders on sys.meta_path.

        https://docs.python.org/3/library/importlib.html#importlib.abc.MetaPathFinder.invalidate_caches
        '''
        fallback_finder = getattr(self, '_fallback_finder', None)
        if fallback_finder is not None and hasattr(fallback_finder, 'invalidate_caches'):
            fallback_finder.invalidate_caches()

    
    def find_spec(self, fullname, target = (None,)):
        '''
        A method for finding a spec for the specified module. The finder will search for the module only within the
        path entry to which it is assigned. If a spec cannot be found, None is returned. When passed in, target is a
        module object that the finder may use to make a more educated guess about what spec to return.

        https://docs.python.org/3/library/importlib.html#importlib.abc.PathEntryFinder.find_spec
        '''
        trace(f'''{self}: find_spec: called with fullname={fullname!r}, target={fullname!r}''')
        pyz_entry_name = self._compute_pyz_entry_name(fullname)
        entry_data = self._pyz_archive.toc.get(pyz_entry_name)
        if entry_data is None:
            trace(f'''{self}: find_spec: {fullname!r} not found in PYZ...''')
            if self.fallback_finder is not None:
                trace(f'''{self}: find_spec: attempting resolve using fallback finder {self.fallback_finder!r}.''')
                fallback_spec = self.fallback_finder.find_spec(fullname, target)
                trace(f'''{self}: find_spec: fallback finder returned spec: {fallback_spec!r}.''')
                return fallback_spec
            None(f'''{self}: find_spec: fallback finder is not available.''')
            return None
        typecode = None[0]
        trace(f'''{self}: find_spec: found {fullname!r} in PYZ as {pyz_entry_name!r}, typecode={typecode}''')
        if typecode == pyimod01_archive.PYZ_ITEM_NSPKG:
            spec = _frozen_importlib.ModuleSpec(fullname, None)
            spec.submodule_search_locations = [
                os.path.join(sys._MEIPASS, pyz_entry_name.replace('.', os.path.sep))]
            return spec
        origin = None.get_filename(fullname)
        is_package = typecode == pyimod01_archive.PYZ_ITEM_PKG
        spec = _frozen_importlib.ModuleSpec(fullname, self, is_package, origin, **('is_package', 'origin'))
        spec.has_location = True
        if is_package:
            spec.submodule_search_locations = [
                os.path.dirname(origin)]
        return spec

    if sys.version_info[:2] < (3, 12):
        
        def find_loader(self, fullname):
            '''
            A legacy method for finding a loader for the specified module. Returns a 2-tuple of (loader, portion) where
            portion is a sequence of file system locations contributing to part of a namespace package. The loader may
            be None while specifying portion to signify the contribution of the file system locations to a namespace
            package. An empty list can be used for portion to signify the loader is not part of a namespace package. If
            loader is None and portion is the empty list then no loader or location for a namespace package were found
            (i.e. failure to find anything for the module).

            Deprecated since python 3.4, removed in 3.12.
            '''
            spec = self.find_spec(fullname)
            if spec is None:
                return (None, [])
            if not spec.submodule_search_locations:
                pass
            return (None.loader, [])

        
        def find_module(self, fullname):
            '''
            A concrete implementation of Finder.find_module() which is equivalent to self.find_loader(fullname)[0].

            Deprecated since python 3.4, removed in 3.12.
            '''
            (loader, portions) = self.find_loader(fullname)
            return loader

    
    def create_module(self, spec):
        '''
        A method that returns the module object to use when importing a module. This method may return None, indicating
        that default module creation semantics should take place.

        https://docs.python.org/3/library/importlib.html#importlib.abc.Loader.create_module
        '''
        pass

    
    def exec_module(self, module):
        '''
        A method that executes the module in its own namespace when a module is imported or reloaded. The module
        should already be initialized when exec_module() is called. When this method exists, create_module()
        must be defined.

        https://docs.python.org/3/library/importlib.html#importlib.abc.Loader.exec_module
        '''
        spec = module.__spec__
        bytecode = self.get_code(spec.name)
        if bytecode is None:
            raise RuntimeError(f'''Failed to retrieve bytecode for {spec.name!r}!''')
        if not None(module, '__file__'):
            raise AssertionError
        if None.submodule_search_locations is not None:
            module.__path__ = spec.submodule_search_locations
        exec(bytecode, module.__dict__)

    
    def load_module(self, fullname):
        """
            A legacy method for loading a module. If the module cannot be loaded, ImportError is raised, otherwise the
            loaded module is returned.

            Deprecated since python 3.4, slated for removal in 3.12 (but still present in python's own FileLoader in
            both v3.12.4 and v3.13.0rc1).
            """
        _bootstrap = _bootstrap
        import importlib._bootstrap
        return _bootstrap._load_module_shim(self, fullname)

    
    def get_filename(self, fullname):
        '''
        A method that is to return the value of __file__ for the specified module. If no path is available, ImportError
        is raised.

        If source code is available, then the method should return the path to the source file, regardless of whether a
        bytecode was used to load the module.

        https://docs.python.org/3/library/importlib.html#importlib.abc.ExecutionLoader.get_filename
        '''
        pyz_entry_name = self._compute_pyz_entry_name(fullname)
        entry_data = self._pyz_archive.toc.get(pyz_entry_name)
        if entry_data is None:
            raise ImportError(f'''Module {fullname!r} not found in PYZ archive (entry {pyz_entry_name!r}).''')
        typecode = None[0]
        if typecode == pyimod01_archive.PYZ_ITEM_PKG:
            return os.path.join(sys._MEIPASS, pyz_entry_name.replace('.', os.path.sep), '__init__.pyc')
        if None == pyimod01_archive.PYZ_ITEM_MODULE:
            return os.path.join(sys._MEIPASS, pyz_entry_name.replace('.', os.path.sep) + '.pyc')

    
    def get_code(self, fullname):
        '''
        Return the code object for a module, or None if the module does not have a code object (as would be the case,
        for example, for a built-in module). Raise an ImportError if loader cannot find the requested module.

        https://docs.python.org/3/library/importlib.html#importlib.abc.InspectLoader.get_code
        '''
        pyz_entry_name = self._compute_pyz_entry_name(fullname)
        entry_data = self._pyz_archive.toc.get(pyz_entry_name)
        if entry_data is None:
            raise ImportError(f'''Module {fullname!r} not found in PYZ archive (entry {pyz_entry_name!r}).''')
        return None._pyz_archive.extract(pyz_entry_name)

    
    def get_source(self, fullname):
        """
        A method to return the source of a module. It is returned as a text string using universal newlines, translating
        all recognized line separators into '
' characters. Returns None if no source is available (e.g. a built-in
        module). Raises ImportError if the loader cannot find the module specified.

        https://docs.python.org/3/library/importlib.html#importlib.abc.InspectLoader.get_source
        """
        filename = self.get_filename(fullname)
        filename = filename[:-1]
    # WARNING: Decompyle incomplete

    
    def is_package(self, fullname):
        '''
        A method to return a true value if the module is a package, a false value otherwise. ImportError is raised if
        the loader cannot find the module.

        https://docs.python.org/3/library/importlib.html#importlib.abc.InspectLoader.is_package
        '''
        pyz_entry_name = self._compute_pyz_entry_name(fullname)
        entry_data = self._pyz_archive.toc.get(pyz_entry_name)
        if entry_data is None:
            raise ImportError(f'''Module {fullname!r} not found in PYZ archive (entry {pyz_entry_name!r}).''')
        typecode = None[0]
        return typecode == pyimod01_archive.PYZ_ITEM_PKG

    
    def get_data(self, path):
        '''
        A method to return the bytes for the data located at path. Loaders that have a file-like storage back-end that
        allows storing arbitrary data can implement this abstract method to give direct access to the data stored.
        OSError is to be raised if the path cannot be found. The path is expected to be constructed using a module’s
        __file__ attribute or an item from a package’s __path__.

        https://docs.python.org/3/library/importlib.html#importlib.abc.ResourceLoader.get_data
        '''
        pass
    # WARNING: Decompyle incomplete

    
    def get_resource_reader(self, fullname):
        '''
        Return resource reader compatible with `importlib.resources`.
        '''
        pyz_entry_name = self._compute_pyz_entry_name(fullname)
        return PyiFrozen