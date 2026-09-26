"""Inspect BTF retained in the kernel images downloaded for the corpus."""
import json,pathlib,sys
from elftools.elf.elffile import ELFFile
from btf_probe import inspect
results={}
for path in sorted(pathlib.Path(sys.argv[1]).glob('*/vmlinux')):
 with path.open('rb') as stream:
  section=ELFFile(stream).get_section_by_name('.BTF')
  results[path.parent.name]=inspect(section.data()) if section else None
print(json.dumps(results,indent=2))
