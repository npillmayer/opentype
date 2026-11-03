import {test} from 'node:test';
import assert from 'node:assert/strict';
import {LatestUpdate} from './updates.mjs';
const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return {promise,resolve,reject};};
test('late success cannot replace a newer proof even if transport ignores abort',async()=>{
 const latest=new LatestUpdate(),old=deferred(),current=deferred(),seen=[];
 const a=latest.run(latest.invalidate(),()=>old.promise,v=>seen.push(v),e=>seen.push(e));
 const b=latest.run(latest.invalidate(),()=>current.promise,v=>seen.push(v),e=>seen.push(e));
 current.resolve('new proof');await b;old.resolve('old proof');await a;
 assert.deepEqual(seen,['new proof']);
});
test('editing invalidates both result and error before the next debounce request',async()=>{
 for(const reject of [false,true]){
  const latest=new LatestUpdate(),old=deferred(),seen=[];let signal;
  const a=latest.run(latest.invalidate(),s=>{signal=s;return old.promise;},v=>seen.push(v),e=>seen.push(e));
  latest.invalidate();assert.equal(signal.aborted,true);
  if(reject)old.reject(new Error('old error'));else old.resolve('old result');
  await a;assert.deepEqual(seen,[]);
 }
});
test('current validation failures surface and open-case can start its own preview',async()=>{
 const latest=new LatestUpdate(),seen=[],id=latest.invalidate();
 await latest.run(id,()=>{throw new Error('bad range');},()=>assert.fail(),e=>seen.push(e.message));
 await latest.run(id,async()=>({text:'case'}),()=>latest.run(id,async()=>'preview',v=>seen.push(v),()=>assert.fail()),()=>assert.fail());
 assert.deepEqual(seen,['bad range','preview']);
});
