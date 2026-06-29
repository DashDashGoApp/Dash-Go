#!/bin/sh
# Tiny boot/deferred-housekeeping storage canary. It writes one small file, fsyncs it,
# and records a conservative rollup. It never declares a card bad from one
# transient failure.
set -u
DASH="${DASH:-$HOME/dashboard}"; CACHE_DIR="$DASH/cache"; LOG_DIR="$DASH/logs"; STATE="$CACHE_DIR/storage-wear-state.json"; CANARY="$CACHE_DIR/.wear-canary"
mkdir -p "$CACHE_DIR" "$LOG_DIR" 2>/dev/null || true
now="$(date +%s 2>/dev/null || printf 0)"
ro=0
if command -v findmnt >/dev/null 2>&1; then findmnt -no OPTIONS -T "$DASH" 2>/dev/null | tr ',' '\n' | grep -qx ro && ro=1; fi
canary=ok
python3 - "$CANARY" "$now" <<'PY' >/dev/null 2>&1 || canary=failed
import os,sys
path,stamp=sys.argv[1:]
data=("dash-go-wear-canary "+stamp+"\n").encode()
fd=os.open(path, os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600)
try: os.write(fd,data); os.fsync(fd)
finally: os.close(fd)
with open(path,"rb") as f:
    if f.read()!=data: raise SystemExit(1)
PY
free_kb="$(df -Pk "$DASH" 2>/dev/null | awk 'NR==2{print $4}' | tr -cd '0-9')"; free_kb="${free_kb:-0}"
io_errors=0
# Housekeeping must not wait indefinitely on a slow/corrupt journal. Kernel-log
# evidence is advisory only, so a short bounded attempt is safer than a stall.
# Ordinary mmcblk boot/partition discovery lines are normal and must never count
# as storage failures. Match a concrete error signature on an mmc line instead.
storage_error_pattern='mmcblk[^:]*:.*(I/O error|timeout|timed out|error -[0-9]+|reset)|Buffer I/O error|EXT4-fs.*(error|remounting filesystem read-only)|filesystem.*read-only'
if command -v journalctl >/dev/null 2>&1; then
  if command -v timeout >/dev/null 2>&1; then
    io_errors="$(timeout 5 journalctl -k -b --no-pager 2>/dev/null | grep -Eic "$storage_error_pattern" || true)"
  else
    io_errors="$(journalctl -k -b --no-pager 2>/dev/null | grep -Eic "$storage_error_pattern" || true)"
  fi
fi
python3 - "$STATE" "$now" "$ro" "$canary" "$free_kb" "$io_errors" <<'PY'
import json,os,sys
path,now,ro,canary,free,errors=sys.argv[1:]
now=int(now); ro=int(ro); free=int(free); errors=int(errors)
try: old=json.load(open(path,encoding='utf-8'))
except Exception: old={}
history=[x for x in old.get('samples',[]) if isinstance(x,dict)][-7:]
history.append({'at':now,'freeKB':free,'kernelErrors':errors,'canary':canary})
consecutive=(int(old.get('consecutiveCanaryFailures',0) or 0)+1) if canary!='ok' else 0
level='ok'; reason='storage canary and current boot checks are normal'
if ro:
    level='failing'; reason='dashboard filesystem is read-only'
elif consecutive>=2:
    level='failing'; reason='storage write/read canary failed repeatedly'
elif errors>=3:
    level='warn'; reason='current boot kernel log contains storage I/O or filesystem errors'
elif canary!='ok':
    level='watch'; reason='one storage canary write/read failed; it will be checked again during housekeeping'
elif free and free<512*1024:
    level='watch'; reason='less than 512 MB free on dashboard filesystem'
p={'level':level,'reason':reason,'updated':now,'readOnly':bool(ro),'canary':canary,'consecutiveCanaryFailures':consecutive,'freeKB':free,'kernelErrorsCurrentBoot':errors,'samples':history}
tmp=path+'.tmp'
with open(tmp,'w',encoding='utf-8') as f: json.dump(p,f,indent=2); f.write('\n')
os.replace(tmp,path)
PY
