import {LatestUpdate} from './updates.mjs';
import {SourceText} from './source.mjs';

const updates=new LatestUpdate();
let sourceText = new SourceText();
const $ = id => document.getElementById(id);
const NS = 'http://www.w3.org/2000/svg';
const guides = {diagnostics:'All diagnostics',show_bboxes:'Ink bounds',show_baseline:'Baseline',show_origins:'Origins',show_labels:'Glyph labels',show_pens:'Pen positions',show_advances:'Advances',color_clusters:'Cluster colors'};
const token = location.hash.slice(1) || sessionStorage.getItem('workbench-token') || '';
if (token) sessionStorage.setItem('workbench-token', token);
history.replaceState(null, '', location.pathname);
let directory = '', timer, result = null, selected = null, svg, overlay;
const splitFeatures = value => value.trim().split(/[\s,]+/u).filter(Boolean);
function node(tag, text, cls) { const n=document.createElement(tag); if(text!==undefined)n.textContent=text; if(cls)n.className=cls; return n; }
function vector(tag, attrs={}) { const n=document.createElementNS(NS,tag); for(const [k,v] of Object.entries(attrs))n.setAttribute(k,v); return n; }
for (const [key,label] of Object.entries(guides)) {
 const el=node('label',undefined,'check'), check=node('input'); check.type='checkbox';check.id=key;
 el.append(check,document.createTextNode(label));$('guides').append(el);
}
function state(message, type='ready') { $('status').textContent=message; $('status').dataset.state=type; }
function exportsEnabled(enabled) { for(const id of ['save','png','json'])$(id).disabled=!enabled; }
function invalidate() {
 const revision=updates.invalidate(); clearTimeout(timer);
 result=null; selected=null; exportsEnabled(false);
 $('preview').setAttribute('aria-busy','true');
 // Do not allow selection against old text while the newer preview is pending.
 $('glyphs').replaceChildren();$('row').replaceChildren();$('source').replaceChildren();$('details').textContent='Waiting for the current proof.';
 $('cluster').textContent='Highlighting describes a cluster span, not an exact character-to-glyph mapping.';overlay?.replaceChildren();$('error').hidden=true;$('notes').hidden=true;
 return revision;
}
function fail(error, id) {
 if(id!==updates.revision)return;
 result=null;exportsEnabled(false);$('error').textContent=error.message;$('error').hidden=false;
 $('preview').replaceChildren(node('p','No current proof. Correct the error above.','empty'));$('preview').setAttribute('aria-busy','false');state('Proof could not be updated','error');
}
async function api(path, body, signal) {
 const response=await fetch('/api/'+path,{method:'POST',headers:{'Content-Type':'application/json','X-Workbench-Token':token},body:JSON.stringify(body),signal});
 if(!response.ok)throw new Error(await response.text());
 return response.json();
}
function numeric(id) { const value=$(id).valueAsNumber;if(!Number.isInteger(value))throw new Error(`${id} must be an integer`);return value; }
function readCase() {
 const render={ppem:numeric('ppem')};for(const key of Object.keys(guides))render[key]=$(key).checked;
 const variants=Array.from($('variants').children).map(row=>({label:row.querySelector('[data-label]').value,features:splitFeatures(row.querySelector('[data-features]').value)}));
 const text=sourceText.text;
 // Reject isolated UTF-16 surrogates instead of JSON silently replacing them.
 for(const scalar of text) {const cp=scalar.codePointAt(0);if(cp>=0xd800&&cp<=0xdfff)throw new Error('Text contains an isolated UTF-16 surrogate.');}
 return {version:1,font:$('font').value,font_sha256:$('fingerprint').value,text,script:$('script').value,language:$('language').value,direction:$('direction').value,features:splitFeatures($('features').value),variants,render,buffer:{flush:$('flush').value,high_watermark:numeric('high'),low_watermark:numeric('low'),max_buffer:numeric('max')},testfont:$('testfont').checked};
}
function addVariant(value={label:'Variant '+($('variants').children.length+1),features:[]}) {
 const row=node('div',undefined,'variant'), line=node('div',undefined,'inline');
 const label=node('label','Label'), name=node('input');name.dataset.label='';name.value=value.label;label.append(name);
 const remove=node('button','×');remove.type='button';remove.setAttribute('aria-label','Remove variant');remove.onclick=()=>{row.remove();schedule();};line.append(label,remove);
 const featuresLabel=node('label','Features'), features=node('input');features.dataset.features='';features.value=(value.features||[]).join(', ');features.placeholder='liga=0, liga[7:13]=1';features.spellcheck=false;featuresLabel.append(features);
 row.append(line,featuresLabel);$('variants').append(row);
}
function applyCase(c) {
 $('font').value=c.font||'';$('fingerprint').value=c.font_sha256||'';sourceText=new SourceText(c.text??'');$('text').value=sourceText.value;
 $('script').value=c.script||'Latn';$('language').value=c.language||'en';$('direction').value=c.direction||'ltr';$('features').value=(c.features||[]).join(', ');
 $('ppem').value=c.render?.ppem||72;for(const key of Object.keys(guides))$(key).checked=!!c.render?.[key];
 $('flush').value=c.buffer?.flush||'run';$('high').value=c.buffer?.high_watermark||0;$('low').value=c.buffer?.low_watermark||0;$('max').value=c.buffer?.max_buffer||0;$('testfont').checked=!!c.testfont;
 $('variants').replaceChildren();for(const v of c.variants||[{label:'Defaults'}])addVariant(v);
 countScalars();
}
function countScalars(){ $('scalarCount').textContent=`${Array.from(sourceText.text).length} original scalars · normalization and whitespace preserved`; }
function schedule() { sourceText.edit($('text').value); const id=invalidate();countScalars();state('Edits pending…','pending');timer=setTimeout(()=>update(id),300); }
async function update(id=updates.revision) {
 if(id!==updates.revision)return;
 state('Shaping and rendering…','pending');
 await updates.run(id, signal=>api('preview',readCase(),signal), next=>{
  // Image and geometry travel together. No independent image-load callback can
  // install an older result over this revision.
  result=next;$('fingerprint').value=next.report.case.font_sha256;
  $('preview').setAttribute('aria-busy','false');$('error').hidden=true;exportsEnabled(true);
  $('notes').textContent=(next.report.notes||[]).join(' ');$('notes').hidden=!next.report.notes?.length;
  renderPreview();renderRows();
  const first=next.report.rows.findIndex(row=>row.glyphs.length);
  if(first>=0)selectGlyph(first,0,false);else {$('source').textContent=next.report.case.text;$('details').textContent='No glyphs emitted.';}
  state(`${next.report.rows.length} variants · ${next.geometry.width} × ${next.geometry.height} px · current`);
 }, error=>fail(error,id));
}
function renderPreview() {
 const {width,height}=result.geometry;
 svg=vector('svg',{viewBox:`0 0 ${width} ${height}`,role:'img','aria-label':'Shaped glyph proof'});
 svg.append(vector('image',{href:'data:image/png;base64,'+result.png,width,height}));
 overlay=vector('g',{class:'selection'});svg.append(overlay);svg.addEventListener('click',pick);
 $('preview').replaceChildren(svg);zoom();
}
function zoom() {if(!svg||!result)return;const factor=Number($('zoom').value)/100;svg.setAttribute('width',result.geometry.width*factor);svg.setAttribute('height',result.geometry.height*factor);}
function renderRows() {
 $('row').replaceChildren();result.report.rows.forEach((r,i)=>{const o=node('option',r.label);o.value=i;$('row').append(o);});renderGlyphList(0);
}
function renderGlyphList(rowIndex) {
 $('row').value=rowIndex;$('glyphs').replaceChildren();
 for(const g of result.report.rows[rowIndex].glyphs) {
  const tr=node('tr');tr.dataset.index=g.index;
  const button=node('button',String(g.index));button.setAttribute('aria-label',`Select glyph ${g.index}`);button.onclick=()=>selectGlyph(rowIndex,g.index);
  const cell=node('td');cell.append(button);tr.append(cell);
  for(const value of [g.gid,g.cluster,g.visual_index,`${g.x_advance} / ${g.y_advance}`,`${g.x_offset} / ${g.y_offset}`])tr.append(node('td',String(value)));
  $('glyphs').append(tr);
 }
}
function selectGlyph(rowIndex,index,selectInput=true) {
 if(!result)return;
 const g=result.report.rows[rowIndex].glyphs[index], screen=result.geometry.rows[rowIndex].glyphs[index];if(!g||!screen)return;
 if(Number($('row').value)!==rowIndex)renderGlyphList(rowIndex);
 selected={row:rowIndex,index};for(const tr of $('glyphs').children)tr.classList.toggle('selected',Number(tr.dataset.index)===index);
 $('details').textContent=`${result.report.rows[rowIndex].label} · logical ${index} / visual ${g.visual_index} · GID ${g.gid} · cluster ${g.cluster}\nAdvance (${g.x_advance}, ${g.y_advance}) · offset (${g.x_offset}, ${g.y_offset})\nPen (${g.pen_x}, ${g.pen_y}) → origin (${g.origin_x}, ${g.origin_y}) · font units, Y ↑`;
 const text=result.report.case.text, start=screen.cluster_start, end=screen.cluster_end, scalars=Array.from(text), mark=node('mark',scalars.slice(start,end).join(''));
 $('source').replaceChildren(document.createTextNode(scalars.slice(0,start).join('')),mark,document.createTextNode(scalars.slice(end).join('')));
 // Automatic refresh must not move a caret while the user is typing.
 if(selectInput)$('text').setSelectionRange(sourceText.selection(start),sourceText.selection(end));
 const codepoints=scalars.slice(start,end).map(ch=>'U+'+ch.codePointAt(0).toString(16).toUpperCase().padStart(4,'0')).join(' ');
 $('cluster').textContent=`Original scalar span [${start}, ${end}): ${codepoints || '(empty)'}. Inferred from output cluster anchors; shared by marks, ligatures or merged clusters. No exact character-to-glyph mapping is implied.`;
 overlay.replaceChildren();
 const line=(a,b,cls)=>overlay.append(vector('line',{x1:a.x,y1:a.y,x2:b.x,y2:b.y,class:cls}));
 line({x:0,y:result.geometry.rows[rowIndex].baseline},{x:result.geometry.width,y:result.geometry.rows[rowIndex].baseline},'baseline');
 if(screen.ink)overlay.append(vector('rect',{x:screen.ink.x,y:screen.ink.y,width:screen.ink.width,height:screen.ink.height,class:'ink'}));
 line(screen.pen,screen.next,'pen');line({x:screen.pen.x,y:screen.pen.y-6},{x:screen.pen.x,y:screen.pen.y+6},'pen');line({x:screen.next.x,y:screen.next.y-6},{x:screen.next.x,y:screen.next.y+6},'pen');
 overlay.append(vector('rect',{x:screen.pen.x-3,y:screen.pen.y-3,width:6,height:6,class:'pen'}));
 line(screen.pen,screen.origin,'origin');line({x:screen.origin.x-6,y:screen.origin.y},{x:screen.origin.x+6,y:screen.origin.y},'origin');line({x:screen.origin.x,y:screen.origin.y-6},{x:screen.origin.x,y:screen.origin.y+6},'origin');
}
function pick(event) {
 if(!result)return;
 const point=svg.createSVGPoint();point.x=event.clientX;point.y=event.clientY;const p=point.matrixTransform(svg.getScreenCTM().inverse());
 const hits=[];
 result.geometry.rows.forEach((row,r)=>row.glyphs.forEach((g,i)=>{const b=g.hit;if(p.x>=b.x&&p.x<=b.x+b.width&&p.y>=b.y&&p.y<=b.y+b.height)hits.push({row:r,index:i,area:b.width*b.height});}));
 hits.sort((a,b)=>a.area-b.area||a.row-b.row||a.index-b.index);
 if(!hits.length)return;
 const prior=hits.findIndex(h=>selected?.row===h.row&&selected?.index===h.index), hit=hits[(prior+1)%hits.length];selectGlyph(hit.row,hit.index);
 if(hits.length>1)state(`${hits.length} overlapping targets · click again to cycle · current`);
}
async function openCase(path) {
 const id=invalidate();state('Opening case…','pending');
 await updates.run(id, signal=>api('open',{path},signal), async c=>{
  applyCase(c);$('casePath').value=path;await update(id);
 }, error=>fail(error,id));
}
function download(name, data, type) {
 const url=URL.createObjectURL(new Blob([data],{type})),link=node('a');link.href=url;link.download=name;document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
}
$('save').onclick=()=>{if(result)download('proof.case.json',JSON.stringify(result.report.case,null,2)+'\n','application/json');};
$('json').onclick=()=>{if(result)download('proof.json',JSON.stringify(result.report,null,2)+'\n','application/json');};
$('png').onclick=()=>{if(result)download('proof.png',Uint8Array.from(atob(result.png),c=>c.charCodeAt(0)),'image/png');};
$('open').onclick=()=>openCase($('casePath').value);
$('casePath').addEventListener('keydown',e=>{if(e.key==='Enter'){e.preventDefault();openCase($('casePath').value);}});
$('latinExample').onclick=()=>openCase(directory+'/testdata/proof-cases/latin-features.json');
$('hebrewExample').onclick=()=>openCase(directory+'/testdata/proof-cases/hebrew-marks.json');
$('newCase').onclick=()=>{invalidate();applyCase({text:'',variants:[{label:'Defaults'}],render:{ppem:72,show_baseline:true,color_clusters:true}});$('preview').replaceChildren(node('p','Enter text and a font path.','empty'));$('preview').setAttribute('aria-busy','false');state('New case');};
$('latinPreset').onclick=()=>{$('script').value='Latn';$('language').value='en';$('direction').value='ltr';schedule();};
$('hebrewPreset').onclick=()=>{$('script').value='Hebr';$('language').value='he';$('direction').value='rtl';schedule();};
$('addVariant').onclick=()=>{addVariant();schedule();};
$('refresh').onclick=()=>{sourceText.edit($('text').value);const id=invalidate();update(id);};
$('zoom').onchange=zoom;
$('row').onchange=()=>{if(!result)return;const row=Number($('row').value);renderGlyphList(row);if(result.report.rows[row].glyphs.length)selectGlyph(row,0);else{selected=null;overlay.replaceChildren();$('details').textContent='No glyphs emitted in this variant.';$('source').textContent=result.report.case.text;$('cluster').textContent='No output cluster to highlight.';}};
$('variants').addEventListener('input',schedule);
for(const id of ['font','fingerprint','text','script','language','direction','features','ppem','flush','high','low','max','testfont',...Object.keys(guides)])$(id).addEventListener('input',schedule);
// Guard init too: editing or opening while init is in flight takes precedence.
const initRevision=updates.revision;
updates.run(initRevision, signal=>api('init',{},signal), config=>{
 directory=config.directory;$('directory').textContent='Server directory: '+directory;
 if(config.case){applyCase(config.case);update();}else{applyCase({variants:[{label:'Defaults'}],render:{ppem:72,show_baseline:true,color_clusters:true}});state('Ready · open a case or choose a font');}
},error=>fail(error,initRevision));
