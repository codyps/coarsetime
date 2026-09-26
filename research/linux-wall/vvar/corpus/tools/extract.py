import pathlib,subprocess,struct,json,hashlib
root=pathlib.Path('/tmp/coarsetime-distro-vdso')
def elf_end(data,pos):
 try:
  if data[pos:pos+6]!=b'\x7fELF\x02\x01':return 0
  h=struct.unpack_from('<16sHHIQQQIHHHHHH',data,pos)
  if h[1]!=3 or h[2]!=62 or h[11]!=64 or not 0<h[12]<128:return 0
  shoff,num=h[6],h[12];end=shoff+num*64
  if end>1000000:return 0
  for i in range(num):
   sh=struct.unpack_from('<IIQQQQIIQQ',data,pos+shoff+i*64)
   if sh[1]!=8:end=max(end,sh[4]+sh[5])
  if end>1000000 or pos+end>len(data):return 0
  if b'__vdso_clock_gettime\0' not in data[pos:pos+end]:return 0
  return end
 except struct.error:return 0
for record in json.loads((root/'downloads.json').read_text()):
 name=record['name'];out=root/name
 if name=='arch':continue
 kernel=next(out.glob('vmlinuz*'));data=kernel.read_bytes();expanded=None
 for magic,cmd in [(b'\x28\xb5\x2f\xfd',['zstd','-d','-c']),(b'\xfd7zXZ\0',['xz','-d','-c']),(b'\x1f\x8b\x08',['gzip','-d','-c']),(b'\x02\x21\x4c\x18',['lz4','-d','-c'])]:
  start=0
  for _ in range(20):
   pos=data.find(magic,start)
   if pos<0:break
   result=subprocess.run(cmd,input=data[pos:],capture_output=True)
   if result.stdout.startswith(b'\x7fELF'):
    expanded=result.stdout;print(name,'decompressed',len(expanded),'offset',pos,flush=True);break
   start=pos+1
  if expanded is not None:break
 if expanded is None:raise RuntimeError(name+' no decompressed kernel')
 (out/'vmlinux').write_bytes(expanded)
 start=0;found=[]
 while True:
  pos=expanded.find(b'\x7fELF',start)
  if pos<0:break
  end=elf_end(expanded,pos)
  if end:
   blob=expanded[pos:pos+end];path=out/f'vdso64-{len(found)}.so';path.write_bytes(blob)
   found.append(dict(offset=pos,size=end,sha256=hashlib.sha256(blob).hexdigest()))
  start=pos+4
 (out/'extraction.json').write_text(json.dumps(found,indent=2)+'\n')
 print(name,found,flush=True)
