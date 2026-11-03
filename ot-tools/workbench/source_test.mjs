import {test} from 'node:test';
import assert from 'node:assert/strict';
import {SourceText,scalarToUTF16,clusterText} from './source.mjs';
test('astral scalars, ligatures, marks and RTL spans address original text',()=>{
 const text='😀office cafe\u0301 שָׁ';
 assert.equal(scalarToUTF16(text,1),2);
 assert.equal(clusterText(text,2,5),'ffi');
 assert.equal(clusterText(text,11,13),'e\u0301');
 assert.equal(clusterText(text,14,17),'שָׁ');
 assert.equal(clusterText(text,0,1),'😀');
});
test('unchanged source roundtrips CRLF, CR, supplementary and NFD',()=>{
 const source=new SourceText('😀office\r\ncafe\u0301\rשָׁ\n');
 assert.equal(source.value,'😀office\ncafe\u0301\nשָׁ\n');
 source.edit(source.value);
 assert.equal(source.text,'😀office\r\ncafe\u0301\rשָׁ\n');
 assert.equal(source.selection(9),9); // c after CRLF in original scalar indexes
});
test('editing preserves untouched line endings and normalization',()=>{
 const source=new SourceText('office\r\ncafe\u0301\rשָׁ');
 source.edit('offices\ncafe\u0301\nשָׁ');
 assert.equal(source.text,'offices\r\ncafe\u0301\rשָׁ');
 source.edit('offices\ncafe\u0301!\nשָׁ');
 assert.equal(source.text,'offices\r\ncafe\u0301!\rשָׁ');
 source.edit('officescafe\u0301!\nשָׁ');
 assert.equal(source.text,'officescafe\u0301!\rשָׁ');
});
