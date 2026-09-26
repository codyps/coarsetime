"""Extract symbol bytes and hashes from the saved ELF64 vDSO fixtures."""
import hashlib,json,pathlib,struct
root=pathlib.Path(__file__).resolve().parent
summary=[]
for folder in sorted(p for p in root.iterdir() if p.is_dir()):
 blob=(folder/'vdso.so').read_bytes()
 h=struct.unpack_from('<16sHHIQQQIHHHHHH',blob)
 assert blob[:6]==b'\x7fELF\x02\x01' and h[1:3]==(3,62)
 sections=[struct.unpack_from('<IIQQQQIIQQ',blob,h[6]+i*h[11]) for i in range(h[12])]
 def content(sh):return blob[sh[4]:sh[4]+sh[5]]
 names=content(sections[h[13]])
 def string(data,off):return data[off:data.index(0,off)].decode()
 named={string(names,s[0]):s for s in sections}
 dyn=named['.dynsym'];strings=content(sections[dyn[6]])
 syms=[struct.unpack_from('<IBBHQQ',content(dyn),i) for i in range(0,dyn[5],dyn[9])]
 sym=next(s for s in syms if string(strings,s[0])=='__vdso_clock_gettime')
 section=sections[sym[3]];offset=section[4]+sym[4]-section[3]
 code=blob[offset:offset+sym[5]];assert len(code)==sym[5]
 (folder/'clock-gettime.bin').write_bytes(code)
 jump_at=4 if code.startswith(bytes.fromhex('f30f1efa')) else 0
 target=None
 if code[jump_at:jump_at+1]==b'\xe9' and len(code)>=jump_at+5:
  target=sym[4]+jump_at+5+struct.unpack_from('<i',code,jump_at+1)[0]
 row={'sample':folder.name,'elf_sha256':hashlib.sha256(blob).hexdigest(),'entry':hex(sym[4]),'symbol_size':sym[5],'entry_jump_target':hex(target) if target is not None else None,'symbol_sha256':hashlib.sha256(code).hexdigest(),'text_sha256':hashlib.sha256(content(named['.text'])).hexdigest(),'has_alternatives':'.altinstructions' in named,'full_symbol_table':'.symtab' in named}
 summary.append(row)
 (folder/'analysis.json').write_text(json.dumps(row,indent=2)+'\n')
(root/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
for row in summary:print(row['sample'],row['entry'],row['symbol_size'],row['entry_jump_target'],row['symbol_sha256'][:16])
