const state = {
  info: null,
  tables: [],
  open: [],         // [{name}]
  active: null,     // table name
  cache: {},        // name -> tableData
  page: {},         // name -> {limit, offset}
  selected: {},     // name -> Set<rowIndex>
  editing: null,    // { name, index, pk } or null
  confirmResolve: null, // resolver for confirm modal
};

const $ = (sel, root=document) => root.querySelector(sel);
const $$ = (sel, root=document) => Array.from(root.querySelectorAll(sel));
const el = (tag, attrs={}, ...kids) => {
  const e = document.createElement(tag);
  for (const [k,v] of Object.entries(attrs)) {
    if (k === 'class') e.className = v;
    else if (k === 'html') e.innerHTML = v;
    else if (k.startsWith('on') && typeof v === 'function') e.addEventListener(k.slice(2), v);
    else if (v !== false && v != null) e.setAttribute(k, v);
  }
  for (const k of kids) {
    if (k == null) continue;
    e.appendChild(typeof k === 'string' ? document.createTextNode(k) : k);
  }
  return e;
};

const fmt = {
  num: (n) => n.toLocaleString(),
};

function toast(msg) {
  const t = $('#toast');
  t.textContent = msg;
  t.style.display = 'block';
  clearTimeout(toast._t);
  toast._t = setTimeout(()=> t.style.display='none', 4500);
}

async function api(path) {
  const res = await fetch(path);
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'request failed');
  return data;
}

// ── Init ─────────────────────────────────────────
async function init() {
  try {
    state.info = await api('/api/info');
    $('#conn-driver').textContent = state.info.driver;
    $('#conn-schema').textContent = state.info.schema || '—';
    $('#conn-schema').title = state.info.schema || '';
    const v = (state.info.version || '').split(/[\s,(]/)[0] || '—';
    $('#conn-version').textContent = v;
    $('#conn-version').title = state.info.version || '';
  } catch(e) { toast('Failed to load info: ' + e.message); }
  await loadTables();
}

async function loadTables() {
  try {
    state.tables = await api('/api/tables');
    renderTables();
  } catch(e) { toast('Failed to load tables: ' + e.message); }
}

function renderTables() {
  const list = $('#tables-list');
  list.innerHTML = '';
  
  // Visualizations Group
  if (state.info && state.info.driver && state.info.driver.toLowerCase() !== 'mongodb') {
    list.appendChild(el('div',{class:'group-label'},'Visualizations'));
    const erdItem = el('div',{class:'table-item' + (state.active === '@erd' ? ' active' : ''), onclick: () => openERD() },
      el('div',{class:'name'},
        el('span',{html: erdIconSVG()}),
        el('span',{}, 'ERD Diagram')
      )
    );
    list.appendChild(erdItem);
  }

  list.appendChild(el('div',{class:'group-label'},'Tables'));
  const q = $('#search').value.trim().toLowerCase();
  const items = state.tables.filter(t => !q || t.name.toLowerCase().includes(q));
  if (!items.length) {
    list.appendChild(el('div',{class:'group-label', style:'color:var(--text-muted)'}, q ? 'No matches' : 'No tables'));
    return;
  }
  for (const t of items) {
    const item = el('div',{class:'table-item' + (t.name === state.active ? ' active' : ''), onclick: () => openTable(t.name) },
      el('div',{class:'name'},
        el('span',{html: tableIconSVG()}),
        el('span',{title: t.name}, t.name)
      ),
      el('span',{class:'count'}, fmt.num(t.rowCount))
    );
    list.appendChild(item);
  }
}

$('#search').addEventListener('input', renderTables);

// ── Tabs ─────────────────────────────────────────
function openTable(name) {
  if (!state.open.find(t => t.name === name)) state.open.push({name});
  state.active = name;
  state.page[name] = state.page[name] || {limit: 100, offset: 0};
  renderTables();
  renderTabs();
  renderActive();
}

function closeTab(name, ev) {
  ev && ev.stopPropagation();
  state.open = state.open.filter(t => t.name !== name);
  delete state.cache[name];
  if (state.active === name) {
    state.active = state.open.length ? state.open[state.open.length-1].name : null;
  }
  renderTables();
  renderTabs();
  renderActive();
}

function renderTabs() {
  const tabs = $('#tabs');
  tabs.innerHTML = '';
  
  // Permanent ERD tab if it was opened
  if (state.open.find(t => t.name === '@erd')) {
    tabs.appendChild(el('div',{class:'tab' + (state.active === '@erd' ? ' active' : ''), onclick: ()=>{ state.active = '@erd'; renderTabs(); renderActive(); }},
      el('span', {html: erdIconSVG()}),
      'ERD Diagram',
      el('span',{class:'close', onclick: (e)=>closeTab('@erd', e), html: closeIconSVG()})
    ));
  }

  for (const t of state.open) {
    if (t.name === '@erd') continue;
    tabs.appendChild(el('div',{class:'tab' + (t.name === state.active ? ' active' : ''), onclick: ()=>{ state.active = t.name; renderTabs(); renderActive(); }},
      el('span', {html: tableIconSVG()}),
      t.name,
      el('span',{class:'close', onclick: (e)=>closeTab(t.name, e), html: closeIconSVG()})
    ));
  }
}

// ── Active view ──────────────────────────────────
async function renderActive() {
  const content = $('#content');
  content.innerHTML = '';
  if (!state.active) {
    content.appendChild(el('div',{class:'splash'},
      el('div',{class:'inner'},
        el('h1', {}, 'Welcome to GMS Studio'),
        el('p', {}, 'Select a table from the left to start browsing.')
      )));
    return;
  }

  if (state.active === '@erd') {
    await renderERD();
    return;
  }

  const name = state.active;
  // toolbar
  const toolbar = el('div',{class:'toolbar'});
  const refreshBtn = el('button',{class:'btn icon-only', title:'Refresh', onclick: ()=>refresh(name)}, el('span',{html: refreshIconSVG()}));
  const filtersBtn = el('button',{class:'btn'}, el('span',{html: filterIconSVG()}), 'Filters', el('span',{class:'label-dim'}, 'None'));
  const fieldsBtn  = el('button',{class:'btn'}, el('span',{html: columnsIconSVG()}), 'Fields', el('span',{class:'label-dim'}, 'All'));
  const info = el('div',{class:'info', id:'rangeInfo'}, 'Loading…');
  const addBtn = el('button',{class:'btn primary', onclick: ()=> openAddModal(name)}, el('span',{html: plusIconSVG()}), 'Add record');
  const delSelBtn = el('button',{class:'btn', id:'delSelBtn', style:'display:none', onclick: ()=> deleteSelected(name)}, el('span',{html: trashIconSVG()}), 'Delete Selected');
  toolbar.append(refreshBtn, filtersBtn, fieldsBtn, info, el('div',{class:'spacer'}), delSelBtn, addBtn);
  content.appendChild(toolbar);

  // body wrap
  const wrap = el('div',{class:'data-wrap', id:'dataWrap'},
    el('div',{class:'splash'}, el('div',{}, el('span',{class:'spinner'}), ' Loading…')));
  content.appendChild(wrap);

  // pagination
  const pager = el('div',{class:'pagination', id:'pagerBar'});
  content.appendChild(pager);

  await loadAndRender(name);
}

async function loadAndRender(name) {
  const page = state.page[name];
  try {
    const data = await api(`/api/table/${encodeURIComponent(name)}?limit=${page.limit}&offset=${page.offset}`);
    state.cache[name] = data;
    drawTable(data);
  } catch(e) {
    toast('Failed to load table: ' + e.message);
    $('#dataWrap').innerHTML = '';
    $('#dataWrap').appendChild(el('div',{class:'splash'}, el('div',{class:'inner'},
      el('p',{style:'color:var(--red)'}, 'Error: ' + e.message))));
  }
}

function refresh(name) {
  delete state.cache[name];
  loadAndRender(name);
  loadTables();
}

function typeBadgeClass(t) {
  const x = (t || '').toLowerCase();
  if (/(int|serial|float|double|decimal|numeric|real|number)/.test(x)) return 'num';
  if (/(char|text|uuid|json|enum|string)/.test(x)) return 'text';
  if (/(date|time|stamp)/.test(x)) return 'date';
  if (/(bool|bit)/.test(x)) return 'bool';
  return '';
}
function typeBadgeChar(t) {
  const cls = typeBadgeClass(t);
  return cls === 'num' ? '#'
       : cls === 'text' ? 'A'
       : cls === 'date' ? '◷'
       : cls === 'bool' ? '⊙'
       : '?';
}

function drawTable(data) {
  if (state.info && state.info.driver && state.info.driver.toLowerCase() === 'mongodb') return drawMongoTable(data);
  const wrap = $('#dataWrap');
  wrap.innerHTML = '';
  const table = el('table',{class:'data'});
  const thead = el('thead');
  const trh = el('tr');

  // Select-all checkbox
  const selAllCheck = el('input',{type:'checkbox', onchange: function(e) {
    const checked = e.target.checked;
    if (!state.selected[data.name]) state.selected[data.name] = new Set();
    state.selected[data.name].clear();
    if (checked) {
      data.rows.forEach((_, idx) => state.selected[data.name].add(idx));
    }
    // Update all checkboxes
    $$('.row-check', tbody).forEach(cb => cb.checked = checked);
    updateDelBtn(data.name);
  }});
  trh.appendChild(el('th',{class:'check-col'}, selAllCheck));

  for (const c of data.columns) {
    const badgeClass = c.isPK ? 'pk' : typeBadgeClass(c.type);
    const badgeChar = c.isPK ? '⚷' : typeBadgeChar(c.type);
    trh.appendChild(el('th', {},
      el('div',{class:'col-name'},
        el('span',{class:'badge ' + badgeClass}, badgeChar),
        c.name,
        c.nullable ? el('span',{class:'nullable', title:'nullable'}, '?') : null,
        el('span',{class:'typename'}, c.type)
      )
    ));
  }
  // Actions header
  trh.appendChild(el('th',{style:'width:80px'}, ''));
  thead.appendChild(trh);
  table.appendChild(thead);

  const tbody = el('tbody');
  if (data.rows.length === 0) {
    const tr = el('tr');
    tr.appendChild(el('td',{colspan: data.columns.length+2, style:'padding:24px; text-align:center; color:var(--text-muted); font-style:italic;'}, 'No rows.'));
    tbody.appendChild(tr);
  }

  if (!state.selected[data.name]) state.selected[data.name] = new Set();

  for (let ri = 0; ri < data.rows.length; ri++) {
    const row = data.rows[ri];
    const isEditing = state.editing && state.editing.name === data.name && state.editing.index === ri;
    const tr = el('tr');
    tr.setAttribute('data-row-id', ri);
    if (isEditing) tr.classList.add('editing-row');
    const rowSel = state.selected && state.selected[data.name] && state.selected[data.name].has(ri);
    if (rowSel) tr.classList.add('selected');

    // Checkbox
    const cb = el('input',{type:'checkbox', class:'row-check', checked: rowSel, onchange: function(e) {
      if (e.target.checked) state.selected[data.name].add(ri);
      else state.selected[data.name].delete(ri);
      updateDelBtn(data.name);
    }});
    tr.appendChild(el('td',{class:'check-col'}, cb));

    // PK info
    const pk = {};
    data.columns.forEach((c, i) => { if (c.isPK) pk[c.name] = row[i]; });
    const hasPK = Object.keys(pk).length > 0;

    // Data columns
    row.forEach((v, i) => {
      const c = data.columns[i];
      const td = el('td');
      td.setAttribute('data-col-pos', i);
      let cls = '';
      let text = '';
      if (v === null || v === undefined) { cls = 'null'; text = 'null'; }
      else if (typeof v === 'number') { cls = 'num'; text = String(v); }
      else if (typeof v === 'boolean') { cls = v ? 'bool-true' : 'bool-false'; text = v ? 'true' : 'false'; }
      else { text = String(v); }
      td.className = cls + (isEditing && !c.isPK ? ' editable-cell' : '');
      td.textContent = text;
      td.title = text;

      // Click non-PK cell to edit inline
      if (hasPK && !c.isPK) {
        td.style.cursor = 'pointer';
        td.addEventListener('click', function(e) {
          if (e.target.closest('input') || e.target.closest('select')) return;
          if (state.editing && state.editing.name === data.name && state.editing.index === ri) {
            // Already editing this row — replace just this cell
            makeCellEditable(td, c, v);
          } else {
            // Start editing this row
            state.editing = { name: data.name, index: ri, pk };
            // Update actions cell for this row
            const atd = tr.querySelector('.actions-cell');
            if (atd) {
              atd.innerHTML = '';
              atd.appendChild(el('button',{class:'btn', style:'color:var(--green);padding:4px 8px', onclick: (e2)=>{ e2.stopPropagation(); saveEditing(data.name); }}, 'Save'));
              atd.appendChild(el('button',{class:'btn', style:'color:var(--red);padding:4px 8px;margin-left:4px', onclick: (e2)=>{ e2.stopPropagation(); cancelEditing(data.name); }}, 'Cancel'));
            }
            tr.classList.add('editing-row');
            makeCellEditable(td, c, v);
          }
        });
      }
      tr.appendChild(td);
    });

    // Action buttons
    const actionTd = el('td',{style:'white-space:nowrap; padding:4px 8px', class:'actions-cell'});
    if (isEditing && hasPK) {
      actionTd.appendChild(el('button',{class:'btn', style:'color:var(--green);padding:4px 8px', onclick: (e)=>{ e.stopPropagation(); saveEditing(data.name); }}, 'Save'));
      actionTd.appendChild(el('button',{class:'btn', style:'color:var(--red);padding:4px 8px;margin-left:4px', onclick: (e)=>{ e.stopPropagation(); cancelEditing(data.name); }}, 'Cancel'));
    } else if (hasPK) {
      actionTd.appendChild(el('button',{class:'btn icon-only', title:'Delete', onclick: (e)=>{ e.stopPropagation(); deleteRecord(data.name, pk); }}, el('span',{html: trashIconSVG()})));
    }
    tr.appendChild(actionTd);
    tbody.appendChild(tr);
  }
  table.appendChild(tbody);
  wrap.appendChild(table);

  // info + pager
  const page = state.page[data.name];
  const start = data.total === 0 ? 0 : page.offset + 1;
  const end   = page.offset + data.rows.length;
  $('#rangeInfo').innerHTML = `Showing <b>${fmt.num(start)}–${fmt.num(end)}</b> of <b>${fmt.num(data.total)}</b>`;
  const pager = $('#pagerBar');
  pager.innerHTML = '';
  pager.append(
    el('span',{}, `Page size: `),
    pageSizeSelect(data.name, page.limit),
    el('span',{class:'grow'}),
    el('span',{}, `${fmt.num(start)}–${fmt.num(end)} of ${fmt.num(data.total)}`),
    pageBtn('« First', page.offset === 0, () => { page.offset = 0; loadAndRender(data.name); }),
    pageBtn('‹ Prev',  page.offset === 0, () => { page.offset = Math.max(0, page.offset - page.limit); loadAndRender(data.name); }),
    pageBtn('Next ›',  end >= data.total, () => { page.offset = page.offset + page.limit; loadAndRender(data.name); }),
    pageBtn('Last »',  end >= data.total, () => { page.offset = Math.max(0, Math.floor((data.total-1)/page.limit) * page.limit); loadAndRender(data.name); }),
    el('span',{style:'margin-left:10px; opacity:.6'}, `(${data.took})`)
  );
}

function pageSizeSelect(name, current) {
  const sel = el('select',{class:'btn', style:'padding:4px 8px', onchange: (e)=>{
    const page = state.page[name];
    page.limit = parseInt(e.target.value, 10);
    page.offset = 0;
    loadAndRender(name);
  }});
  for (const n of [25, 50, 100, 250, 500]) {
    const o = el('option',{value: n}, String(n));
    if (n === current) o.selected = true;
    sel.appendChild(o);
  }
  return sel;
}
function pageBtn(label, disabled, onclick) {
  const b = el('button',{class:'btn', onclick}, label);
  if (disabled) { b.disabled = true; b.style.opacity = .35; b.style.cursor = 'not-allowed'; b.onclick = null; }
  return b;
}

// ── CRUD Helpers ─────────────────────────────────
function updateDelBtn(name) {
  const btn = $('#delSelBtn');
  if (!btn) return;
  const sel = state.selected[name];
  const count = sel ? sel.size : 0;
  btn.style.display = count > 0 ? 'inline-flex' : 'none';
  btn.innerHTML = '';
  btn.appendChild(el('span',{html: trashIconSVG()}));
  btn.appendChild(document.createTextNode(' Delete (' + count + ')'));
}

function pkFromRow(row, cols) {
  const pk = {};
  cols.forEach((c, i) => { if (c.isPK) pk[c.name] = row[i]; });
  return pk;
}

function renderJsonTree(val, container) {
  if (val === null) {
    container.appendChild(el('span', {class: 'json-val null'}, 'null'));
    container.appendChild(el('span', {class: 'json-type'}, 'Null'));
  } else if (typeof val === 'boolean') {
    container.appendChild(el('span', {class: 'json-val boolean'}, String(val)));
    container.appendChild(el('span', {class: 'json-type'}, 'Boolean'));
  } else if (typeof val === 'number') {
    container.appendChild(el('span', {class: 'json-val number'}, String(val)));
    container.appendChild(el('span', {class: 'json-type'}, 'Number'));
  } else if (typeof val === 'string') {
    container.appendChild(el('span', {class: 'json-val string'}, '"' + val + '"'));
    container.appendChild(el('span', {class: 'json-type'}, 'String'));
  } else if (Array.isArray(val)) {
    const collapser = el('span', {class: 'json-collapser'}, '▼');
    const head = el('span', {}, 'Array [' + val.length + ']');
    const block = el('div', {class: 'json-tree'});
    container.append(collapser, head, block);
    collapser.onclick = () => {
      const isHidden = block.style.display === 'none';
      block.style.display = isHidden ? 'block' : 'none';
      collapser.textContent = isHidden ? '▼' : '▶';
    };
    val.forEach((item, i) => {
      const line = el('div', {class: 'json-line'});
      line.appendChild(el('span', {class: 'json-key'}, i + ':'));
      renderJsonTree(item, line);
      block.appendChild(line);
    });
  } else if (typeof val === 'object') {
    const collapser = el('span', {class: 'json-collapser'}, '▼');
    const head = el('span', {}, 'Object {' + Object.keys(val).length + '}');
    const block = el('div', {class: 'json-tree'});
    container.append(collapser, head, block);
    collapser.onclick = () => {
      const isHidden = block.style.display === 'none';
      block.style.display = isHidden ? 'block' : 'none';
      collapser.textContent = isHidden ? '▼' : '▶';
    };
    for (const [k, v] of Object.entries(val)) {
      const line = el('div', {class: 'json-line'});
      line.appendChild(el('span', {class: 'json-key'}, k + ':'));
      renderJsonTree(v, line);
      block.appendChild(line);
    }
  }
}

function drawMongoTable(data) {
  const wrap = $('#dataWrap');
  wrap.innerHTML = '';
  
  if (data.rows.length === 0) {
    wrap.appendChild(el('div',{style:'padding:24px; text-align:center; color:var(--text-muted); font-style:italic;'}, 'No documents.'));
  }

  for (let ri = 0; ri < data.rows.length; ri++) {
    const doc = data.rows[ri][0]; // single json col
    const _id = doc._id;
    const docEl = el('div', {class: 'mongo-doc'});
    
    // Header
    const hdr = el('div', {class: 'mongo-doc-header'});
    hdr.appendChild(el('div', {}, el('strong', {}, '_id: '), String(_id)));
    
    const actions = el('div', {class: 'mongo-actions'});
    const editBtn = el('button', {onclick: () => editMongoDoc(data.name, doc)}, el('span', {html: editIconSVG?editIconSVG():'✎'}), ' Edit Document');
    const delBtn = el('button', {onclick: () => deleteMongoDoc(data.name, _id)}, el('span', {html: trashIconSVG?trashIconSVG():'✕'}), ' Delete');
    actions.append(editBtn, delBtn);
    hdr.appendChild(actions);
    
    // Body
    const body = el('div', {class: 'mongo-doc-body'});
    renderJsonTree(doc, body);
    
    docEl.append(hdr, body);
    wrap.appendChild(docEl);
  }

  // Pager
  const page = state.page[data.name];
  const start = data.total === 0 ? 0 : page.offset + 1;
  const end   = page.offset + data.rows.length;
  $('#rangeInfo').innerHTML = `Showing <b>${fmt.num(start)}–${fmt.num(end)}</b> of <b>${fmt.num(data.total)}</b>`;
  const pager = $('#pagerBar');
  pager.innerHTML = '';
  pager.append(
    el('span',{}, `Page size: `),
    pageSizeSelect(data.name, page.limit),
    el('span',{class:'grow'}),
    el('span',{}, `${fmt.num(start)}–${fmt.num(end)} of ${fmt.num(data.total)}`),
    pageBtn('« First', page.offset === 0, () => { page.offset = 0; loadAndRender(data.name); }),
    pageBtn('‹ Prev',  page.offset === 0, () => { page.offset = Math.max(0, page.offset - page.limit); loadAndRender(data.name); }),
    pageBtn('Next ›',  end >= data.total, () => { page.offset = page.offset + page.limit; loadAndRender(data.name); }),
    pageBtn('Last »',  end >= data.total, () => { page.offset = Math.max(0, Math.floor((data.total-1)/page.limit) * page.limit); loadAndRender(data.name); }),
    el('span',{style:'margin-left:10px; opacity:.6'}, `(${data.took})`)
  );
}

async function deleteMongoDoc(name, _id) {
  if (!await showConfirm('Delete Document', 'Are you sure you want to delete this document?')) return;
  try {
    await apiPost(`/api/table/${encodeURIComponent(name)}/delete`, { pks: [{_id: _id}] });
    refresh(name);
    toast('Document deleted');
  } catch(e) {
    showConfirm('Error', 'Failed to delete: ' + e.message, 'OK');
  }
}

function buildMongoForm(doc, container, isAdd = false) {
  for (const [k, v] of Object.entries(doc)) {
    if (k === '_id') {
      if (isAdd) continue;
      // Show _id as readonly in edit
      const grp = el('div', {class: 'form-group'});
      grp.appendChild(el('label', {}, k));
      const inp = el('input', {type: 'text', name: 'mongo_'+k, value: v, readonly: true});
      inp.style.opacity = '0.7';
      grp.appendChild(inp);
      container.appendChild(grp);
      continue;
    }
    const grp = el('div', {class: 'form-group'});
    grp.appendChild(el('label', {}, k));
    
    let type = typeof v;
    if (v === null) type = 'null';
    else if (Array.isArray(v)) type = 'array';
    
    if (type === 'object' || type === 'array') {
      const inp = el('textarea', {name: 'mongo_'+k, rows: 3});
      inp.value = isAdd ? (type === 'array' ? '[]' : '{}') : JSON.stringify(v);
      inp.dataset.type = 'json';
      grp.appendChild(inp);
    } else if (type === 'boolean') {
      const inp = el('select', {name: 'mongo_'+k});
      if (isAdd) inp.appendChild(el('option', {value: '', selected: true}, ''));
      inp.appendChild(el('option', {value: 'true', selected: !isAdd && v===true}, 'true'));
      inp.appendChild(el('option', {value: 'false', selected: !isAdd && v===false}, 'false'));
      inp.dataset.type = 'boolean';
      grp.appendChild(inp);
    } else if (type === 'number') {
      const inp = el('input', {type: 'number', step: 'any', name: 'mongo_'+k});
      inp.value = isAdd ? '' : v;
      inp.dataset.type = 'number';
      grp.appendChild(inp);
    } else {
      const inp = el('input', {type: 'text', name: 'mongo_'+k});
      inp.value = isAdd ? '' : (v !== undefined && v !== null ? String(v) : '');
      inp.dataset.type = 'string';
      grp.appendChild(inp);
    }
    container.appendChild(grp);
  }
}

function getMongoFormData(container, isAdd = false) {
  const result = {};
  const inputs = $$('[name^="mongo_"]', container);
  for (const inp of inputs) {
    const key = inp.name.replace('mongo_', '');
    const val = inp.value;
    const type = inp.dataset.type;
    if (inp.readOnly) {
      result[key] = val;
      continue;
    }
    
    if (type === 'json') {
      if (isAdd && (val === '' || val === '{}' || val === '[]')) continue;
      try { result[key] = JSON.parse(val); }
      catch(e) { result[key] = val; }
    } else if (type === 'boolean') {
      if (isAdd && val === '') continue;
      result[key] = val === 'true';
    } else if (type === 'number') {
      if (isAdd && val === '') continue;
      result[key] = val === '' ? null : Number(val);
    } else {
      if (isAdd && val === '') continue;
      result[key] = val;
    }
  }
  return result;
}

function editMongoDoc(name, doc) {
  const body = $('#addModalBody');
  body.innerHTML = '';
  $('#addModalTitle').textContent = 'Edit Document — ' + name;
  
  buildMongoForm(doc, body, false);
  
  $('#addModal').style.display = 'flex';
  
  const footer = body.nextElementSibling;
  footer.innerHTML = '';
  footer.appendChild(el('button', {class: 'btn', onclick: closeAddModal}, 'Cancel'));
  footer.appendChild(el('button', {class: 'btn primary', onclick: async () => {
    try {
      const parsed = getMongoFormData(body, false);
      const pk = {_id: parsed._id};
      delete parsed._id;
      await apiPost(`/api/table/${encodeURIComponent(name)}/update`, { pk: pk, values: parsed });
      closeAddModal();
      refresh(name);
      toast('Document updated');
    } catch (e) {
      alert('Error updating document: ' + e.message);
    }
  }}, 'Save Document'));
}

const originalOpenAddModal = openAddModal;
openAddModal = function(name) {
  if (state.info && state.info.driver && state.info.driver.toLowerCase() === 'mongodb') {
    const data = state.cache[name];
    const body = $('#addModalBody');
    body.innerHTML = '';
    $('#addModalTitle').textContent = 'Insert Document — ' + name;
    
    // Infer schema from first 50 rows
    const schema = {};
    if (data && data.rows) {
      for (let i = 0; i < Math.min(data.rows.length, 50); i++) {
        const d = data.rows[i][0];
        for (const [k, v] of Object.entries(d)) {
          if (k !== '_id' && schema[k] === undefined) {
             schema[k] = (v === null || v === undefined) ? '' : v;
          }
        }
      }
    }
    // If empty collection, add a placeholder
    if (Object.keys(schema).length === 0) {
      schema['field1'] = '';
    }
    
    buildMongoForm(schema, body, true);
    
    $('#addModal').style.display = 'flex';
    
    const footer = body.nextElementSibling;
    footer.innerHTML = '';
    footer.appendChild(el('button', {class: 'btn', onclick: closeAddModal}, 'Cancel'));
    footer.appendChild(el('button', {class: 'btn primary', onclick: async () => {
      try {
        const parsed = getMongoFormData(body, true);
        await apiPost(`/api/table/${encodeURIComponent(name)}/insert`, { values: parsed });
        closeAddModal();
        refresh(name);
        toast('Document inserted');
      } catch (e) {
        alert('Error inserting document: ' + e.message);
      }
    }}, 'Insert Document'));
    return;
  }
  originalOpenAddModal(name);
}

function colInputType(c) {
  const t = c.type.toLowerCase();
  if (c.enumValues && c.enumValues.length > 0) return 'enum';
  if (/(int|serial|float|double|decimal|numeric|real|number)/.test(t)) return 'number';
  if (/(bool|bit)/.test(t)) return 'checkbox';
  if (/(date|time|stamp)/.test(t)) return 'datetime-local';
  return 'text';
}

// ── Add Modal ────────────────────────────────────
function openAddModal(name) {
  const data = state.cache[name];
  if (!data) return;
  const body = $('#addModalBody');
  body.innerHTML = '';
  $('#addModalTitle').textContent = 'Add Record — ' + name;
  for (const c of data.columns) {
    if (c.isPK && c.autoIncrement) continue;
    const grp = el('div',{class:'form-group'});
    const isAuto = c.hasDefault;
    grp.appendChild(el('label',{}, c.name, ' ', el('span',{class:'hint'}, '(' + c.type + (c.nullable ? ', nullable' : '') + (isAuto ? ', auto-default' : '') + ')')));
    const itype = colInputType(c);
    if (itype === 'enum') {
      const sel = el('select',{name: 'add_'+c.name});
      if (c.nullable) sel.appendChild(el('option',{value: ''}, '(null)'));
      for (const ev of c.enumValues) sel.appendChild(el('option',{value: ev}, ev));
      grp.appendChild(sel);
    } else if (itype === 'checkbox') {
      grp.appendChild(el('input',{type:'checkbox', name: 'add_'+c.name}));
    } else {
      const inp = el('input',{type: itype, name: 'add_'+c.name, step: itype==='number'?'any':undefined});
      grp.appendChild(inp);
    }
    body.appendChild(grp);
  }
  $('#addModal').style.display = 'flex';
}

function closeAddModal() {
  $('#addModal').style.display = 'none';
}

function showConfirm(title, message, btnText='Proceed', btnClass='primary') {
  return new Promise(resolve => {
    $('#confirmTitle').textContent = title;
    $('#confirmBody').textContent = message;
    const btn = $('#confirmBtn');
    btn.textContent = btnText;
    btn.className = 'btn ' + btnClass;
    btn.onclick = () => closeConfirm(true);
    state.confirmResolve = resolve;
    $('#confirmModal').style.display = 'flex';
  });
}

function closeConfirm(ok) {
  $('#confirmModal').style.display = 'none';
  if (state.confirmResolve) {
    state.confirmResolve(ok);
    state.confirmResolve = null;
  }
}

async function saveAddRecord() {
  const name = state.active;
  if (!name) return;
  const data = state.cache[name];
  if (!data) return;
  const values = {};
  for (const c of data.columns) {
    if (c.isPK && c.autoIncrement) continue;
    const inp = document.querySelector('[name="add_'+c.name+'"]');
    if (!inp) continue;
    
    if (inp.type !== 'checkbox' && inp.value === '' && c.hasDefault) {
      continue; // omit from values entirely to trigger db default
    }

    if (inp.type === 'checkbox') values[c.name] = inp.checked;
    else if (inp.type === 'number') values[c.name] = inp.value ? (inp.value.includes('.') ? parseFloat(inp.value) : parseInt(inp.value, 10)) : null;
    else values[c.name] = inp.value || null;
  }
  try {
    await apiPost('/api/table/' + encodeURIComponent(name) + '/insert', { values });
    closeAddModal();
    refresh(name);
  } catch(e) { toast('Failed: ' + e.message); }
}

// ── Inline Editing (per-cell) ────────────────────
function makeCellEditable(td, c, v) {
  const itype = colInputType(c);
  if (itype === 'enum' && c.enumValues) {
    const sel = el('select',{'data-col':c.name});
    for (const ev of c.enumValues) {
      const opt = el('option',{value: ev}, ev);
      if (String(v) === ev) opt.selected = true;
      sel.appendChild(opt);
    }
    td.textContent = '';
    td.appendChild(sel);
    sel.focus();
  } else if (itype === 'checkbox') {
    td.textContent = '';
    td.appendChild(el('input',{type:'checkbox', 'data-col':c.name, checked: v === true || v === 1}));
  } else if (itype === 'datetime-local') {
    const inp = el('input',{type:'datetime-local', 'data-col':c.name, step:'1'});
    if (v !== null && v !== undefined) {
      const d = new Date(v);
      if (!isNaN(d.getTime())) {
        const pad = (n)=>String(n).padStart(2,'0');
        inp.value = d.getFullYear()+'-'+pad(d.getMonth()+1)+'-'+pad(d.getDate())+'T'+pad(d.getHours())+':'+pad(d.getMinutes());
      }
    }
    td.textContent = '';
    td.appendChild(inp);
    inp.focus();
  } else {
    const inp = el('input',{type: itype, 'data-col':c.name, step: itype==='number'?'any':undefined});
    if (v !== null && v !== undefined) inp.value = String(v);
    td.textContent = '';
    td.appendChild(inp);
    inp.focus();
    inp.select();
  }
}

function cancelEditing(name) {
  state.editing = null;
  loadAndRender(name);
}

async function saveEditing(name) {
  const ed = state.editing;
  if (!ed) return;
  const data = state.cache[name];
  if (!data) return;
  const tr = document.querySelector(`tr[data-row-id="${ed.index}"]`);
  if (!tr) return;
  const values = {};
  for (const c of data.columns) {
    if (c.isPK) continue;
    const inp = tr.querySelector(`[data-col="${c.name}"]`);
    if (!inp) continue;
    const itype = colInputType(c);
    if (itype === 'checkbox') values[c.name] = inp.checked;
    else if (itype === 'enum' || inp.tagName === 'SELECT') values[c.name] = inp.value || null;
    else if (inp.type === 'number' && inp.value.trim() !== '') values[c.name] = inp.value.includes('.') ? parseFloat(inp.value) : parseInt(inp.value, 10);
    else values[c.name] = inp.value.trim() || null;
  }
  try {
    await apiPost('/api/table/' + encodeURIComponent(name) + '/update', { values, pk: ed.pk });
    state.editing = null;
    refresh(name);
  } catch(e) { toast('Update failed: ' + e.message); }
}

async function deleteRecord(name, pk) {
  const ok = await showConfirm('Delete Record', 'Are you sure you want to delete this record? This action cannot be undone.', 'Delete', 'danger');
  if (!ok) return;
  try {
    await apiPost('/api/table/' + encodeURIComponent(name) + '/delete', { pks: [pk] });
    refresh(name);
  } catch(e) { toast('Delete failed: ' + e.message); }
}

async function deleteSelected(name) {
  const sel = state.selected[name];
  if (!sel || sel.size === 0) return;
  const ok = await showConfirm('Delete Selected', `Are you sure you want to delete ${sel.size} selected record(s)?`, 'Delete All', 'danger');
  if (!ok) return;
  const data = state.cache[name];
  if (!data) return;
  const pks = [];
  for (const idx of sel) {
    pks.push(pkFromRow(data.rows[idx], data.columns));
  }
  try {
    await apiPost('/api/table/' + encodeURIComponent(name) + '/delete', { pks });
    state.selected[name].clear();
    refresh(name);
  } catch(e) { toast('Delete failed: ' + e.message); }
}

async function apiPost(path, body) {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'request failed');
  return data;
}

// ── Icons (inline SVG) ───────────────────────────
function tableIconSVG()   { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="3" width="12" height="10" rx="1.5"/><path d="M2 7h12M2 10h12M6 3v10"/></svg>`; }
function refreshIconSVG() { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M3 8a5 5 0 0 1 8.5-3.5L13 6"/><path d="M13 3v3h-3"/><path d="M13 8a5 5 0 0 1-8.5 3.5L3 10"/><path d="M3 13v-3h3"/></svg>`; }
function filterIconSVG()  { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M2 4h12l-4.5 5v4l-3 1.5V9z"/></svg>`; }
function columnsIconSVG() { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="3" width="3.5" height="10" rx="1"/><rect x="6.25" y="3" width="3.5" height="10" rx="1"/><rect x="10.5" y="3" width="3.5" height="10" rx="1"/></svg>`; }
function plusIconSVG()    { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M8 3v10M3 8h10"/></svg>`; }
function closeIconSVG()   { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M4 4l8 8M12 4l-8 8"/></svg>`; }
function editIconSVG()    { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M11 2l3 3-9 9H2v-3z"/></svg>`; }
function trashIconSVG()   { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M2 4h12M5 4V3a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v1M4 4l.7 9.3a1 1 0 0 0 1 .7h4.6a1 1 0 0 0 1-.7L12 4M6 7v5M10 7v5"/></svg>`; }
function erdIconSVG()     { return `<svg class="icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="2" y="3" width="5" height="4.5" rx="1"/><rect x="9" y="8.5" width="5" height="4.5" rx="1"/><rect x="9" y="3" width="5" height="4.5" rx="1"/><path d="M7 5.25h2M11.5 7.5v1M5 7.5v3.5a1 1 0 0 0 1 1h3"/></svg>`; }

function openERD() {
  if (!state.open.find(t => t.name === '@erd')) state.open.push({name: '@erd'});
  state.active = '@erd';
  renderTables();
  renderTabs();
  renderActive();
}

async function renderERD() {
  const content = $('#content');
  content.innerHTML = '';
  
  const container = el('div', {class: 'erd-container'},
    el('div', {id: 'erd-canvas'}),
    el('div', {class: 'erd-toolbar'},
      el('button', {class: 'btn', onclick: () => cytoscapeInstance.layout({name: 'dagre'}).run()}, 'Auto Layout'),
      el('button', {class: 'btn', onclick: () => cytoscapeInstance.fit()}, 'Fit View')
    )
  );
  content.appendChild(container);

  const elements = [];
  // Build Nodes (Tables)
  for (const t of state.tables) {
    try {
      const data = await api(`/api/table/${encodeURIComponent(t.name)}?limit=1`);
      
      // HTML for columns
      const colHtml = data.columns.map(c => `
        <div class="erd-column">
          <span class="erd-pk-icon">${c.isPK ? '🔑' : '&nbsp;&nbsp;'}</span>
          <span class="erd-col-name">${c.name}</span>
          <span class="erd-col-type">${c.type.split('(')[0]}</span>
        </div>
      `).join('');

      elements.push({
        data: { 
          id: t.name, 
          label: t.name,
          html: `
            <div class="erd-table">
              <div class="erd-table-header">
                <span>${t.name}</span>
                <span style="opacity:0.5; font-size:9px">#${data.total}</span>
              </div>
              <div class="erd-table-body">
                ${colHtml}
              </div>
            </div>
          `,
          colCount: data.columns.length
        }
      });
      
      // Build Edges (Foreign Keys)
      if (data.constraints) {
        for (const cname in data.constraints) {
          const c = data.constraints[cname];
          if (c.kind === 'FOREIGN_KEY' || c.kind === 2) { // 2 is migrate.ConstraintForeignKey
            elements.push({
              data: {
                id: cname,
                source: t.name,
                target: c.refTable,
                label: c.columns.join(', ') + ' → ' + c.refColumns.join(', ')
              }
            });
          }
        }
      }
    } catch(e) {
      console.error('Failed to load table info for ERD:', e);
    }
  }

  cytoscape.use(cytoscapeDagre);
  
  window.cytoscapeInstance = cytoscape({
    container: document.getElementById('erd-canvas'),
    elements: elements,
    style: [
      {
        selector: 'node',
        style: {
          'width': '220px',
          'height': (ele) => (40 + (ele.data('colCount') * 22)) + 'px',
          'shape': 'rectangle',
          'background-opacity': 0,
          'border-width': 0,
          'label': '' // Label is handled by HTML plugin
        }
      },
      {
        selector: 'edge',
        style: {
          'width': 2,
          'line-color': '#6aa3ff',
          'target-arrow-color': '#6aa3ff',
          'target-arrow-shape': 'triangle',
          'curve-style': 'taxi',
          'taxi-direction': 'auto',
          'label': 'data(label)',
          'font-size': '9px',
          'color': '#8a93a6',
          'text-rotation': 'autorotate',
          'text-background-opacity': 1,
          'text-background-color': '#0d1117',
          'text-background-padding': '2px'
        }
      }
    ],
    layout: {
      name: 'dagre',
      padding: 50
    }
  });

  // Initialize HTML Label plugin
  window.cytoscapeInstance.nodeHtmlLabel([
    {
      query: 'node',
      halign: 'center',
      valign: 'center',
      halignBox: 'center',
      valignBox: 'center',
      tpl: (data) => data.html
    }
  ]);

  window.cytoscapeInstance.on('tap', 'node', function(evt){
    const node = evt.target;
    openTable(node.id());
  });
}

init();
