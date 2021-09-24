import sys
import subprocess
from pathlib import Path


def open_editor(filename):
    subprocess.run('bspc rule -a Alacritty -o state=floating rectangle=1118x420+1361+700', shell=True)
    subprocess.run([
        'alacritty',
        '-e', 'nvim',
        f"{filename}",
    ])

def latex_document(latex):
    return r"""
        \documentclass[12pt,border=12pt]{standalone}
        \usepackage[utf8]{inputenc}
        \usepackage[T1]{fontenc}
        \usepackage{textcomp}
        \usepackage{amsmath, amssymb}
        \newcommand{\R}{\mathbb R}
        \begin{document}
    """ + latex + r"\end{document}"

config = {
    'rofi_theme': None,
    'font': 'monospace',
    'font_size': 10,
    'open_editor': open_editor,
    'latex_document': latex_document,
}


# From https://stackoverflow.com/a/67692
def import_file(name, path):
    import importlib.util as util
    spec = util.spec_from_file_location(name, path)
    module = util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

CONFIG_PATH = Path('~/.config/inktex').expanduser()

if (CONFIG_PATH / 'config.py').exists():
    userconfig = import_file('config', CONFIG_PATH / 'config.py').config
    config.update(userconfig)
