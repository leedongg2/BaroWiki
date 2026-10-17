// CDN 실패나 로딩 지연 중에도 기본 편집 내용이 보존되는지 확인한다.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

const elements = {
    opennamu_select_editor: { value: 'default' },
    opennamu_edit_textarea: { value: '저장할 본문', style: {} },
    opennamu_monaco_editor: { style: {} },
    opennamu_editor_markup: { value: 'namumark' },
    opennamu_edit_origin: { value: '이전 본문' }
};
const scripts = [];
const sandbox = {
    window: {}, console,
    document: {
        getElementById: id => elements[id],
        createElement: () => ({}),
        head: { appendChild: script => scripts.push(script) }
    },
    setTimeout: () => 1, clearTimeout: () => {}
};
vm.createContext(sandbox);
vm.runInContext(fs.readFileSync(path.join(__dirname, '../views/main_css/js/func/editor.js'), 'utf8'), sandbox);
const run = code => vm.runInContext(code, sandbox);
run("do_monaco_init(''); do_sync_monaco_and_textarea(); opennamu_do_sync_monaco_markup(); do_stop_exit();");
assert.equal(scripts.length, 0, 'default editor must not load Monaco');
assert.equal(sandbox.window.onbeforeunload(), '', 'unsaved-change warning must work before Monaco loads');
elements.opennamu_select_editor.value = 'monaco';
run('opennamu_edit_turn_off_monaco(); opennamu_edit_turn_off_monaco(); do_sync_monaco_and_textarea();');
assert.equal(scripts.length, 1, 'concurrent initialization must share one loader');
assert.equal(elements.opennamu_edit_textarea.style.display, 'block');
assert.equal(run('get_select_editor()'), 'textarea');
scripts[0].onerror();
assert.equal(elements.opennamu_select_editor.value, 'default');
assert.equal(elements.opennamu_edit_textarea.value, '저장할 본문');
run('do_sync_monaco_and_textarea(); opennamu_do_sync_monaco_markup();');
console.log('PASS: default editor, delayed CDN, failed CDN, draft preservation, save synchronization');
