// Browser textareas normalize CR and CRLF to LF. Retain the exact source and
// map edits/selection explicitly instead of silently rewriting a loaded case.
export const scalarToUTF16 = (text, index) => Array.from(text).slice(0, index).join('').length;
export const clusterText = (text, start, end) => Array.from(text).slice(start, end).join('');
export function editorProjection(text) {
 let value='', boundaries=[0];
 for(let i=0;i<text.length;i++) {
  if(text[i]==='\r') {if(text[i+1]==='\n')i++;value+='\n';}
  else value+=text[i];
  boundaries.push(i+1);
 }
 return {value,boundaries};
}
export class SourceText {
 constructor(text=''){this.text=text;}
 get value(){return editorProjection(this.text).value;}
 edit(value) {
  const before=editorProjection(this.text);
  let left=0,right=0;
  while(left<before.value.length&&left<value.length&&before.value[left]===value[left])left++;
  while(right<before.value.length-left&&right<value.length-left&&before.value[before.value.length-1-right]===value[value.length-1-right])right++;
  this.text=this.text.slice(0,before.boundaries[left])+value.slice(left,value.length-right)+this.text.slice(before.boundaries[before.value.length-right]);
 }
 selection(scalar){return editorProjection(this.text.slice(0,scalarToUTF16(this.text,scalar))).value.length;}
}
