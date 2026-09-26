"""Diagnostic address discovery by emulation; NOT a compatibility verifier."""
import io,json,pathlib,struct
from elftools.elf.elffile import ELFFile
from unicorn import Uc,UC_ARCH_X86,UC_MODE_64,UC_HOOK_MEM_READ,UC_HOOK_INSN,UC_HOOK_CODE,UcError
from unicorn.x86_const import UC_X86_REG_RIP,UC_X86_REG_RDI,UC_X86_REG_RSI,UC_X86_REG_RSP,UC_X86_REG_RAX,UC_X86_INS_SYSCALL
ROOT=pathlib.Path(__file__).resolve().parent
BASE=0x100000;OUT=0x210000;STACK=0x208000;STOP=0x220000

def run(path,retry=False):
 data=path.read_bytes();elf=ELFFile(io.BytesIO(data))
 symbol=next(s for s in elf.get_section_by_name('.dynsym').iter_symbols() if s.name=='__vdso_clock_gettime')
 u=Uc(UC_ARCH_X86,UC_MODE_64)
 u.mem_map(BASE-0x10000,0x20000);u.mem_map(0x200000,0x30000)
 for seg in elf.iter_segments():
  if seg['p_type']=='PT_LOAD':u.mem_write(BASE+seg['p_vaddr'],seg.data())
 u.reg_write(UC_X86_REG_RDI,5);u.reg_write(UC_X86_REG_RSI,OUT);u.reg_write(UC_X86_REG_RSP,STACK)
 u.mem_write(STACK,struct.pack('<Q',STOP))
 reads=[];seqaddr=None;seqreads=0;wide=[];instructions=[]
 def read(uc,access,addr,size,value,user):
  nonlocal seqaddr,seqreads
  if not BASE-0x10000<=addr<BASE:return
  if seqaddr is None:
   assert size==4,'unexpected first access'
   seqaddr=addr
  if addr==seqaddr:
   assert size==4
   value=2 if not retry or seqreads==0 else 4;seqreads+=1
  elif size==8:
   if addr not in wide:wide.append(addr)
   assert len(wide)<=2,'unexpected third data field'
   value=1700000000 if wide.index(addr)==0 else 123456789
  else:raise AssertionError('unexpected data access')
  uc.mem_write(addr,value.to_bytes(size,'little'))
  reads.append({'ip':hex(uc.reg_read(UC_X86_REG_RIP)-BASE),'address_relative_to_elf':addr-BASE,'size':size,'value':value})
 def syscall(*args):raise AssertionError('unexpected syscall')
 u.hook_add(UC_HOOK_CODE,lambda uc,addr,size,user: instructions.append((addr-BASE,size)))
 u.hook_add(UC_HOOK_MEM_READ,read);u.hook_add(UC_HOOK_INSN,syscall,None,1,0,UC_X86_INS_SYSCALL)
 u.emu_start(BASE+symbol['st_value'],STOP,count=1000)
 assert u.reg_read(UC_X86_REG_RIP)==STOP,'instruction budget exceeded'
 assert u.reg_read(UC_X86_REG_RAX)==0
 assert struct.unpack('<qq',u.mem_read(OUT,16))==(1700000000,123456789)
 assert seqreads==(4 if retry else 2)
 alt=elf.get_section_by_name('.altinstructions')
 altdata=alt.data();record_size=12 if path.parent.name in ('ubuntu-5.15','debian-6.1') else 14
 assert len(altdata)%record_size==0
 overlaps=[]
 for off in range(0,len(altdata),record_size):
  origin=alt['sh_addr']+off+struct.unpack_from('<i',altdata,off)[0]
  length=altdata[off+record_size-2]
  if any(ip<origin+length and origin<ip+n for ip,n in instructions):overlaps.append(hex(origin))
 return {'instruction_count':len(instructions),'alternative_overlaps':overlaps,'retry':retry,'seq_relative_to_elf':seqaddr-BASE,'sec_relative_to_seq':wide[0]-seqaddr,'nsec_relative_to_seq':wide[1]-seqaddr,'reads':reads}
if __name__=='__main__':
 results={}
 for path in sorted((ROOT.parent/'corpus').glob('*/vdso.so')):
  results[path.parent.name]=[run(path),run(path,True)]
 (ROOT/'emulated.json').write_text(json.dumps(results,indent=2)+'\n')
 for name,r in results.items():print(name,{k:v for k,v in r[0].items() if k!='reads'},'forced retry passed; alt overlaps',r[0]['alternative_overlaps'],r[1]['alternative_overlaps'])
