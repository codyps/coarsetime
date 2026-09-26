import pathlib,subprocess,json,concurrent.futures
root=pathlib.Path('/tmp/coarsetime-distro-vdso')
def unpack(record):
 name=record['name'];pkg=root/(name+'.pkg');out=root/name;out.mkdir(exist_ok=True)
 if name.startswith(('ubuntu','debian')):
  members=subprocess.check_output(['bsdtar','-tf',str(pkg)],text=True).splitlines();data=next(x for x in members if x.startswith('data.tar'))
  archive=out/'data.tar';archive.write_bytes(subprocess.check_output(['bsdtar','-xOf',str(pkg),data]))
 else:archive=pkg
 members=subprocess.check_output(['bsdtar','-tf',str(archive)],text=True).splitlines()
 selected=[m for m in members if m.endswith(('vdso64.so','vdso64.so.dbg','vdso64.so.debug')) or '/vmlinuz' in m or '/config-' in m]
 for m in selected:
  dest=out/pathlib.Path(m).name
  with dest.open('wb') as f:subprocess.run(['bsdtar','-xOf',str(archive),m],stdout=f,check=True)
 print(name,selected,flush=True)
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as p:list(p.map(unpack,json.loads((root/'downloads.json').read_text())))
