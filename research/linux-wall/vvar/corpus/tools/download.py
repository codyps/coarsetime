import pathlib,subprocess,concurrent.futures,json,hashlib
root=pathlib.Path('/tmp/coarsetime-distro-vdso')
items=[('ubuntu-5.15', 'https://archive.ubuntu.com/ubuntu/pool/main/l/linux/linux-image-unsigned-5.15.0-25-generic_5.15.0-25.25_amd64.deb'), ('ubuntu-6.8', 'https://archive.ubuntu.com/ubuntu/pool/main/l/linux/linux-image-unsigned-6.8.0-31-generic_6.8.0-31.31_amd64.deb'), ('debian-6.1', 'https://deb.debian.org/debian/pool/main/l/linux/linux-image-6.1.0-47-amd64-unsigned_6.1.170-3_amd64.deb'), ('debian-6.12', 'https://deb.debian.org/debian/pool/main/l/linux/linux-image-6.12.43+deb12-amd64-unsigned_6.12.43-1~bpo12+1_amd64.deb'), ('fedora-43', 'https://dl.fedoraproject.org/pub/fedora/linux/releases/43/Everything/x86_64/os/Packages/k/kernel-core-6.17.1-300.fc43.x86_64.rpm'), ('arch', 'https://mirror.moson.org/arch/core/os/x86_64/linux-headers-7.2.3.arch1-2-x86_64.pkg.tar.zst')]
def fetch(item):
 name,url=item;p=root/(name+'.pkg')
 result=subprocess.run(['curl','-fLsS','--retry','2','--max-time','180','-w','%{url_effective}','-o',str(p),url],capture_output=True,text=True)
 record=dict(name=name,url=url,effective_url=result.stdout,status=result.returncode)
 if result.returncode==0:record.update(size=p.stat().st_size,sha256=hashlib.sha256(p.read_bytes()).hexdigest())
 else:record['error']=result.stderr
 print(record,flush=True);return record
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:records=list(pool.map(fetch,items))
(root/'downloads.json').write_text(json.dumps(records,indent=2)+'\n')
