#!/usr/bin/env python3
"""Run preserved v1.4 source counterexamples, not a production backend test."""
from pathlib import Path
import argparse,hashlib,json,os,re,shutil,subprocess,tempfile

HERE=Path(__file__).resolve().parent
ap=argparse.ArgumentParser()
ap.add_argument('--go',default='go')
ap.add_argument('--output',type=Path,default=HERE/'runtime-results.json')
args=ap.parse_args()
manifest=json.loads((HERE/'source-manifest.json').read_text(encoding='utf-8'))
repo=HERE.parents[2]
current_guide=(repo/'docs/FastLedger-新手开发完整指南.md').read_text(encoding='utf-8')
current_plan=(repo/'docs/FastLedger-落地方案.md').read_text(encoding='utf-8')
def code(text,heading):
    return re.search(r'```go\n(.*?)\n```',text.split(heading,1)[1],re.S).group(1)
canonical=code(current_guide,'### 13.1')
current_schema=re.search(r'```sql\n(.*?)\n```',current_guide.split('### 6.4',1)[1],re.S).group(1)
version_method=re.search(r'func \(r \*VersionRepo\) Next.*?\n}',code(current_plan,'**版本号递增 + 原子改余额'),re.S).group(0)
for f in manifest['sources']:
    assert hashlib.sha256((HERE/'source'/f['file']).read_bytes()).hexdigest()==f['sha256'],f['file']

with tempfile.TemporaryDirectory(prefix='fastledger-v14-audit-') as td:
    root=Path(td)
    for name in ['go.mod','go.sum']:
        if (HERE/name).exists():shutil.copyfile(HERE/name,root/name)
    mapping={
      'database.go.txt':'internal/database/db.go',
      'models.go.txt':'internal/models/models.go',
      'authz_stub.go.txt':'internal/services/authz_test_bridge.go',
      'services_test.go.txt':'internal/services/transaction_test.go',
      'hub_test.go.txt':'internal/ws/hub_test.go',
      'canonical_test.go.txt':'internal/services/canonical_test.go',
      'source/transaction_v14.go.txt':'internal/services/transaction.go',
      'source/hub_v14.go.txt':'internal/ws/hub.go',
      'source/schema_v14.sql':'schema.sql',
    }
    for source,destination in mapping.items():
        p=root/destination;p.parent.mkdir(parents=True,exist_ok=True)
        text=(HERE/source).read_text(encoding='utf-8')
        if source=='source/hub_v14.go.txt':
            text+='\n// Test-only import-use shim: no business statement altered.\nvar _ = context.Canceled\n'
        p.write_text(text,encoding='utf-8')
    (root/'internal/services/canonical_current.go').write_text(canonical+'\n',encoding='utf-8')
    (root/'schema-current.sql').write_text(current_schema+'\n',encoding='utf-8')
    holding=code(current_guide,'### 16.1')
    (root/'internal/services/holding_current.go').write_text('package services\nimport ("errors";"fastledger/internal/database";"fastledger/internal/models";"github.com/shopspring/decimal")\n'+holding+'\n',encoding='utf-8')
    (root/'internal/services/version_current.go').write_text('package services\nimport ("time";"fastledger/internal/models";"xorm.io/xorm")\ntype VersionRepo struct{}\n'+version_method+'\n',encoding='utf-8')
    fixed=root/'compilecheckfixed';fixed.mkdir()
    (fixed/'db.go').write_text('package compilecheckfixed\nimport ("log";"xorm.io/xorm";_ "github.com/mattn/go-sqlite3")\n'+code(current_guide,'### 22.4'),encoding='utf-8')
    (fixed/'db_test.go').write_text('''package compilecheckfixed
import ("os";"testing")
func TestUnified_CurrentDSNAndAPI(t *testing.T){
 if err:=os.MkdirAll("data",0700);err!=nil{t.Fatal(err)}
 e:=initDB();defer e.Close()
 var fk int;var mode string;var sqliteVersion string
 if err:=e.DB().QueryRow("PRAGMA foreign_keys").Scan(&fk);err!=nil||fk!=1{t.Fatal(fk,err)}
 if err:=e.DB().QueryRow("PRAGMA journal_mode").Scan(&mode);err!=nil||mode!="wal"{t.Fatal(mode,err)}
 if e.DB().Stats().MaxOpenConnections!=1{t.Fatal("pool setting")}
 if err:=e.DB().QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion);err!=nil{t.Fatal(err)}
 t.Log("OBSERVATION Current DSN verified: foreign_keys=1, journal_mode=wal, pool=1, SQLite="+sqliteVersion)
}
''',encoding='utf-8')
    env=os.environ.copy();env['GOTOOLCHAIN']='local';env.setdefault('GOMAXPROCS','2')
    def run(argv):
        return subprocess.run([args.go,*argv],cwd=root,env=env,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
    version=run(['version']).stdout.strip()
    download=run(['mod','tidy'])
    if download.returncode:raise SystemExit(download.stdout)
    shutil.copyfile(root/'go.mod',HERE/'go.mod')
    shutil.copyfile(root/'go.sum',HERE/'go.sum')
    result=run(['test','-p=2','-race','-json','-count=1','./internal/...','./compilecheckfixed'])
    events=[]
    for line in result.stdout.splitlines():
        try:events.append(json.loads(line))
        except json.JSONDecodeError:pass
    checks=root/'compilecheck';checks.mkdir()
    snippet=(HERE/'source/db_init_v14.go.txt').read_text(encoding='utf-8')
    (checks/'db.go').write_text('package compilecheck\nimport ("log";"xorm.io/xorm")\n'+snippet,encoding='utf-8')
    pragma=(HERE/'source/legacy_pragmas_v14.go.txt').read_text(encoding='utf-8')
    (checks/'pragmas.go').write_text('package compilecheck\nimport "xorm.io/xorm"\n'+pragma,encoding='utf-8')
    compile_result=run(['test','-p=2','./compilecheck'])
    output={
      'source_commit':manifest['commit'],'go_version':version,
      'dependencies':{'xorm.io/xorm':'v1.4.3','github.com/mattn/go-sqlite3':'v1.14.52','github.com/shopspring/decimal':'v1.6.0'},
      'scope':'Extracted v1.4 transaction functions and Hub core with model/database/authz test bridges. Not the full app, real HTTP/WS or Flutter.',
      'counterexample_suite_exit_code':result.returncode,
      'current_create_source_sha256':hashlib.sha256(canonical.encode()).hexdigest(),
      'current_guide_schema_sha256':hashlib.sha256(current_schema.encode()).hexdigest(),
      'test_failure_output':result.stdout if result.returncode else '',
      'tests':[{'package':e.get('Package'),'test':e.get('Test'),'action':e['Action']} for e in events if e.get('Test') and e['Action'] in ['pass','fail']],
      'observations':[e['Output'].strip() for e in events if 'OBSERVATION' in e.get('Output','')],
      'compile_check':{'expected':'fail for unchanged v1.4 API/receiver errors','exit_code':compile_result.returncode,'output':compile_result.stdout},
      'limitations':['Counterexample tests assert defects are reproducible; passing does not mean the implementation is correct.','Race coverage is Hub core only; authorization is stubbed; no end-to-end WS/session validation.'],
    }
    args.output.write_text(json.dumps(output,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    print(json.dumps(output,ensure_ascii=False,indent=2))
    if result.returncode or compile_result.returncode==0:raise SystemExit(1)
