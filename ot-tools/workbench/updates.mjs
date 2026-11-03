// A revision changes immediately on an edit, before the debounce fires. Abort
// saves work; the revision check also protects against responses already in
// flight or transports that cannot cancel. Success and error use the same gate.
export class LatestUpdate {
 revision=0;
 controller=null;
 invalidate() {
  this.revision++;
  this.controller?.abort();
  this.controller=null;
  return this.revision;
 }
 async run(revision, operation, success, failure) {
  if(revision!==this.revision)return;
  this.controller?.abort();
  const controller=new AbortController();this.controller=controller;
  try {
   const value=await operation(controller.signal);
   if(revision===this.revision&&!controller.signal.aborted)await success(value);
  } catch(error) {
   if(revision===this.revision&&!controller.signal.aborted)failure(error);
  } finally {
   if(this.controller===controller)this.controller=null;
  }
 }
}
