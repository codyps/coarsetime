"""Read selected structure member offsets from raw little-endian kernel BTF."""
import json,pathlib,struct,sys

def inspect(blob):
 magic,ver,flags,hlen,toff,tlen,soff,slen=struct.unpack_from('<HBBIIIII',blob)
 assert magic==0xeb9f and ver==1
 strings=blob[hlen+soff:hlen+soff+slen]
 off=hlen+toff;end=off+tlen;found={};unions={};tid=0
 while off<end:
  no,info,size=struct.unpack_from('<III',blob,off);off+=12;tid+=1
  kind=(info>>24)&31;vlen=info&65535;kflag=info>>31
  name=strings[no:strings.find(b'\0',no)].decode()
  if kind==5 or (kind==4 and name in ('vdso_clock','vdso_time_data','vdso_data','vdso_timestamp','arch_vdso_time_data')):
   members=[]
   for n in range(vlen):
    mn,typ,bitoff=struct.unpack_from('<III',blob,off+n*12)
    members.append({'name':strings[mn:strings.find(b'\0',mn)].decode(),'type_id':typ,'bit_offset':bitoff&0xffffff if kflag else bitoff})
   if kind==5:unions[tid]=members
   else:found[name]={'size':size,'members':members}
  lengths={1:4,2:0,3:12,4:12*vlen,5:12*vlen,6:8*vlen,7:0,8:0,9:0,10:0,11:0,12:0,13:8*vlen,14:4,15:12*vlen,16:0,17:4,18:0,19:12*vlen}
  off+=lengths[kind]
 assert off==end
 for obj in found.values():
  for m in list(obj['members']):
   if m['name']=='' and m['type_id'] in unions:
    for u in unions[m['type_id']]:
     obj['members'].append(dict(u,bit_offset=m['bit_offset']+u['bit_offset']))
 return found
if __name__=='__main__':print(json.dumps(inspect(pathlib.Path(sys.argv[1]).read_bytes()),indent=2))
