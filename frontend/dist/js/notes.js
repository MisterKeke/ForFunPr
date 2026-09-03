import { els } from './dom.js';
import {
  listNotes,
  getNote,
  createNote,
  updateNote,
  setNotePinned,
  setNoteArchived,
  deleteNote,
  listNoteTopics,
  getNoteTopicBoard,
  createNoteTopic,
  renameNoteTopic,
  deleteNoteTopic,
  addNoteTopicBlock,
  updateNoteTopicBlockPosition,
  deleteNoteTopicBlock,
  createNoteTopicConnection,
  deleteNoteTopicConnection,
  listNoteTodos,
  getTodos,
  linkNoteTodo,
  unlinkNoteTodo,
} from './api.js';
import { escapeHtml, hasWailsBinding } from './utils.js';
import { showConfirmation } from './ui.js';
import { openTodoModal } from './todos.js';

const NOTES_CHANGED_EVENT = 'notes:changed';
const TODOS_CHANGED_EVENT = 'todos:changed';
const NOTE_PAGE_SIZE = 50;
const AUTOSAVE_DELAY = 800;

let summaries = [];
let total = 0;
let activeNote = null;
let newDraft = false;
let dirty = false;
let conflict = false;
let saveTimer = null;
let savePromise = null;
let listRequest = 0;
let searchTimer = null;
let activeNoteTodos = [];
let activeNoteTodosRequest = 0;
let noteTopics = [];
let activeTopicID = null;
let activeTopicBoard = null;
let topicBoardRequest = 0;
let topicModalMode = 'create';
let notePickerTimer = null;
let notePickerRequest = 0;
let connectionSourceBlockID = null;
let selectedConnectionID = null;

function noteFilters(offset = 0) {
  return {
    query: els.notesSearch.value.trim(),
    archive_status: els.notesArchiveFilter.value,
    pinned: els.notesPinnedFilter.checked ? true : undefined,
    limit: NOTE_PAGE_SIZE,
    offset,
  };
}

function setNoteError(message = '') {
  els.notesError.textContent = message;
  els.notesError.classList.toggle('hidden', !message);
}

function setSaveState(message, state = '') {
  els.notesSaveState.textContent = message;
  els.notesSaveState.dataset.state = state;
}

function formatNoteTime(value) {
  if (!value) return '';
  const normalized = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
    ? `${value.replace(' ', 'T')}Z`
    : value;
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function previewBody(value) {
  const compact = String(value || '').replace(/\s+/g, ' ').trim();
  return compact.length > 160 ? `${compact.slice(0, 160)}…` : compact;
}

function noteDisplayTitle(note) {
  const title = String(note?.title || '').trim();
  if (title) return title;
  const firstLine = String(note?.body || note?.preview || '').split(/\r?\n/)[0].trim();
  return firstLine || 'Untitled note';
}

function renderActiveNoteTodos() {
  const saved = Boolean(activeNote?.id);
  els.noteTaskConnect.disabled = !saved;
  els.noteTaskCreate.disabled = !saved;
  if (!saved) {
    els.noteTaskList.innerHTML = '<div class="note-task-empty">Save this note before connecting tasks.</div>';
    return;
  }
  if (activeNoteTodos.length === 0) {
    els.noteTaskList.innerHTML = '<div class="note-task-empty">No connected tasks.</div>';
    return;
  }
  els.noteTaskList.innerHTML = activeNoteTodos.map((todo) => `
    <div class="note-task-link ${todo.done ? 'done' : ''}" data-todo-id="${escapeHtml(todo.id)}">
      <div class="note-task-link-copy">
        <strong>${escapeHtml(todo.title || 'Untitled task')}</strong>
        <span class="note-task-link-meta">${todo.done ? 'Completed' : 'Pending'}${todo.due_date ? ` · Due ${escapeHtml(todo.due_date)}` : ''}</span>
      </div>
      <div class="note-task-link-buttons">
        <button class="secondary-btn small-btn" type="button" data-action="open-task">Open</button>
        <button class="secondary-btn small-btn" type="button" data-action="unlink-task">Disconnect</button>
      </div>
    </div>
  `).join('');
}

async function loadActiveNoteTodos() {
  const request = ++activeNoteTodosRequest;
  const noteID = activeNote?.id;
  if (!noteID) {
    activeNoteTodos = [];
    renderActiveNoteTodos();
    return;
  }
  try {
    const items = await listNoteTodos(noteID);
    if (request !== activeNoteTodosRequest || activeNote?.id !== noteID) return;
    activeNoteTodos = Array.isArray(items) ? items : [];
    renderActiveNoteTodos();
  } catch (error) {
    if (request === activeNoteTodosRequest) {
      setNoteError(error?.message || 'Connected tasks could not be loaded.');
    }
  }
}

function renderNoteList() {
  els.notesListSummary.textContent = total === 1 ? '1 note' : `${total} notes`;
  els.notesLoadMore.classList.toggle('hidden', summaries.length >= total);
  if (summaries.length === 0) {
    els.notesList.innerHTML = '<div class="notes-empty">No notes match these filters.</div>';
    return;
  }
  els.notesList.innerHTML = summaries.map((note) => `
    <button class="note-list-item ${activeNote?.id === note.id ? 'active' : ''}" type="button" data-note-id="${escapeHtml(note.id)}">
      <span class="note-list-title-row">
        <strong>${note.pinned ? '<span class="note-pin-mark" aria-label="Pinned">★</span>' : ''}${escapeHtml(noteDisplayTitle(note))}</strong>
        ${note.archived ? '<span class="note-state-badge">Archived</span>' : ''}
      </span>
      <span class="note-list-preview">${escapeHtml(note.preview || 'No body text')}</span>
      <span class="note-list-time">${escapeHtml(formatNoteTime(note.updated_at))}</span>
    </button>
  `).join('');
}

function renderEditor() {
  const visible = Boolean(activeNote) || newDraft;
  els.notesEditorEmpty.classList.toggle('hidden', visible);
  els.notesEditorContent.classList.toggle('hidden', !visible);
  if (!visible) return;

  els.notesTitle.value = activeNote?.title || '';
  els.notesBody.value = activeNote?.body || '';
  els.notesPin.textContent = activeNote?.pinned ? 'Unpin' : 'Pin';
  els.notesArchive.textContent = activeNote?.archived ? 'Restore' : 'Archive';
  els.notesDelete.disabled = !activeNote;
  els.notesUpdatedAt.textContent = activeNote?.updated_at
    ? `Updated ${formatNoteTime(activeNote.updated_at)}`
    : 'New note';
  els.notesConflict.classList.toggle('hidden', !conflict);
  els.notesReload.classList.toggle('hidden', !conflict || !activeNote);
  renderActiveNoteTodos();
  if (!dirty && !conflict) setSaveState(activeNote ? 'Saved' : 'Not saved');
}

function summaryFromNote(note) {
  return {
    id: note.id,
    title: note.title,
    preview: previewBody(note.body),
    pinned: Boolean(note.pinned),
    archived: Boolean(note.archived),
    revision: note.revision,
    created_at: note.created_at,
    updated_at: note.updated_at,
  };
}

function summaryMatchesFilters(summary, note) {
  const archive = els.notesArchiveFilter.value;
  if (archive === 'active' && summary.archived) return false;
  if (archive === 'archived' && !summary.archived) return false;
  if (els.notesPinnedFilter.checked && !summary.pinned) return false;
  const query = els.notesSearch.value.trim().toLowerCase();
  if (!query) return true;
  return String(summary.title || '').toLowerCase().includes(query) ||
    String(note?.body || summary.preview || '').toLowerCase().includes(query);
}

function updateSummary(note) {
  const summary = summaryFromNote(note);
  const index = summaries.findIndex((item) => item.id === note.id);
  const matches = summaryMatchesFilters(summary, note);
  if (index >= 0 && matches) {
    summaries[index] = summary;
  } else if (index >= 0) {
    summaries.splice(index, 1);
    total = Math.max(0, total - 1);
  } else if (matches) {
    summaries.unshift(summary);
    total += 1;
  }
  summaries.sort((left, right) => {
    if (left.pinned !== right.pinned) return left.pinned ? -1 : 1;
    return String(right.updated_at).localeCompare(String(left.updated_at)) || right.id - left.id;
  });
  renderNoteList();
}

export async function loadNotes({ append = false } = {}) {
  const request = ++listRequest;
  const offset = append ? summaries.length : 0;
  if (!append) setNoteError();
  try {
    const result = await listNotes(noteFilters(offset));
    if (request !== listRequest) return;
    const items = Array.isArray(result?.notes) ? result.notes : [];
    summaries = append ? summaries.concat(items) : items;
    total = Number(result?.total || 0);
    renderNoteList();
  } catch (error) {
    if (request === listRequest) setNoteError(error?.message || 'Notes could not be loaded.');
  }
}

async function selectNote(id) {
  if (activeNote?.id === id && !newDraft) return;
  if (dirty && !(await flushNoteSave())) return;
  setNoteError();
  try {
    activeNote = await getNote(id);
    newDraft = false;
    dirty = false;
    conflict = false;
    activeNoteTodos = [];
    renderEditor();
    renderNoteList();
    void loadActiveNoteTodos();
    els.notesBody.focus();
  } catch (error) {
    setNoteError(error?.message || 'Note could not be loaded.');
  }
}

function startNewNote() {
  if (dirty) {
    void flushNoteSave().then((saved) => {
      if (saved) startNewNote();
    });
    return;
  }
  activeNote = null;
  newDraft = true;
  dirty = false;
  conflict = false;
  activeNoteTodos = [];
  renderEditor();
  renderNoteList();
  els.notesTitle.focus();
}

function markNoteDirty() {
  dirty = true;
  conflict = false;
  els.notesConflict.classList.add('hidden');
  els.notesReload.classList.add('hidden');
  setSaveState('Unsaved', 'dirty');
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => void flushNoteSave(), AUTOSAVE_DELAY);
}

export async function flushNoteSave() {
  clearTimeout(saveTimer);
  saveTimer = null;
  if (!dirty) return true;
  if (conflict) return false;
  if (savePromise) return savePromise;

  const title = els.notesTitle.value;
  const body = els.notesBody.value;
  if (!title.trim() && !body.trim()) {
    setSaveState('Not saved');
    dirty = false;
    return true;
  }

  setSaveState('Saving…', 'saving');
  savePromise = (async () => {
    try {
      const note = activeNote
        ? await updateNote({
            id: activeNote.id,
            title,
            body,
            expected_revision: activeNote.revision,
          })
        : await createNote({ title, body, pinned: false });
      activeNote = note;
      newDraft = false;
      dirty = false;
      conflict = false;
      updateSummary(note);
      renderEditor();
      setSaveState('Saved', 'saved');
      void loadActiveNoteTodos();
      return true;
    } catch (error) {
      const message = error?.message || 'Note could not be saved.';
      if (/changed after it was loaded|conflict/i.test(message)) {
        conflict = true;
        els.notesConflict.textContent = 'This note changed elsewhere. Reload the latest version before saving again.';
        els.notesConflict.classList.remove('hidden');
        els.notesReload.classList.remove('hidden');
        setSaveState('Conflict', 'error');
      } else {
        setNoteError(message);
        setSaveState('Save failed', 'error');
      }
      return false;
    } finally {
      savePromise = null;
    }
  })();
  return savePromise;
}

async function changeNoteState(kind) {
  if (!(await flushNoteSave()) || !activeNote) return;
  setNoteError();
  try {
    const note = kind === 'pin'
      ? await setNotePinned(activeNote.id, !activeNote.pinned, activeNote.revision)
      : await setNoteArchived(activeNote.id, !activeNote.archived, activeNote.revision);
    activeNote = note;
    updateSummary(note);
    renderEditor();
    if (kind === 'archive' && els.notesArchiveFilter.value !== 'all') {
      activeNote = null;
      newDraft = false;
      await loadNotes();
      renderEditor();
    }
  } catch (error) {
    setNoteError(error?.message || 'Note state could not be changed.');
  }
}

async function removeActiveNote() {
  if (!activeNote) return;
  const confirmed = await showConfirmation({
    title: 'Delete note?',
    message: `“${noteDisplayTitle(activeNote)}” will be permanently deleted. This cannot be undone.`,
    confirmLabel: 'Delete note',
  });
  if (!confirmed) return;
  try {
    await deleteNote(activeNote.id);
    activeNote = null;
    newDraft = false;
    dirty = false;
    conflict = false;
    activeNoteTodos = [];
    await loadNotes();
    if (activeTopicID) void loadActiveTopicBoard();
    renderEditor();
  } catch (error) {
    setNoteError(error?.message || 'Note could not be deleted.');
  }
}

async function reloadActiveNote() {
  if (!activeNote) return;
  try {
    activeNote = await getNote(activeNote.id);
    dirty = false;
    conflict = false;
    renderEditor();
    updateSummary(activeNote);
    void loadActiveNoteTodos();
  } catch (error) {
    setNoteError(error?.message || 'Latest note could not be loaded.');
  }
}

async function setNotesMode(mode) {
  if (mode === 'topics' && !(await flushNoteSave())) return;
  const topicsActive = mode === 'topics';
  els.notesLibraryTab.classList.toggle('active', !topicsActive);
  els.notesLibraryTab.setAttribute('aria-selected', String(!topicsActive));
  els.notesTopicsTab.classList.toggle('active', topicsActive);
  els.notesTopicsTab.setAttribute('aria-selected', String(topicsActive));
  els.notesLibraryPanel.classList.toggle('hidden', topicsActive);
  els.notesTopicsPanel.classList.toggle('hidden', !topicsActive);
  if (topicsActive) await loadNoteTopics(activeTopicID);
}

function setTopicStatus(message = '', state = '') {
  els.noteTopicStatus.textContent = message;
  els.noteTopicStatus.dataset.state = state;
}

function selectedTopic() {
  return noteTopics.find((topic) => topic.id === activeTopicID) || null;
}

function renderTopicSelector() {
  if (noteTopics.length === 0) {
    els.noteTopicSelect.innerHTML = '<option value="">No topics yet</option>';
  } else {
    els.noteTopicSelect.innerHTML = noteTopics.map((topic) => `
      <option value="${escapeHtml(topic.id)}" ${topic.id === activeTopicID ? 'selected' : ''}>
        ${escapeHtml(topic.title)} (${escapeHtml(topic.block_count || 0)})
      </option>
    `).join('');
  }
  const hasTopic = Boolean(activeTopicID);
  els.noteTopicSelect.disabled = !hasTopic;
  els.noteTopicRename.disabled = !hasTopic;
  els.noteTopicDelete.disabled = !hasTopic;
  els.noteTopicAddNote.disabled = !hasTopic;
  els.noteTopicRelationType.disabled = !hasTopic;
  els.noteTopicDeleteConnection.disabled = !selectedConnectionID;
}

export async function loadNoteTopics(preferredTopicID = null) {
  try {
    const items = await listNoteTopics();
    noteTopics = Array.isArray(items) ? items : [];
    const preferred = Number(preferredTopicID || activeTopicID || 0);
    activeTopicID = noteTopics.some((topic) => topic.id === preferred)
      ? preferred
      : (noteTopics[0]?.id || null);
    renderTopicSelector();
    if (activeTopicID) {
      await loadActiveTopicBoard();
    } else {
      activeTopicBoard = null;
      renderTopicBoard();
    }
  } catch (error) {
    setTopicStatus(error?.message || 'Topics could not be loaded.', 'error');
  }
}

async function loadActiveTopicBoard() {
  const topicID = activeTopicID;
  if (!topicID) {
    activeTopicBoard = null;
    renderTopicBoard();
    return;
  }
  const request = ++topicBoardRequest;
  activeTopicBoard = null;
  renderTopicBoard();
  setTopicStatus('Loading topic…');
  try {
    const board = await getNoteTopicBoard(topicID);
    if (request !== topicBoardRequest || activeTopicID !== topicID) return;
    activeTopicBoard = board;
    connectionSourceBlockID = null;
    selectedConnectionID = null;
    renderTopicBoard();
  } catch (error) {
    if (request === topicBoardRequest) {
      setTopicStatus(error?.message || 'Topic could not be loaded.', 'error');
    }
  }
}

function blockDisplayTitle(block) {
  const title = String(block?.title || '').trim();
  return title || String(block?.preview || '').split(/\r?\n/)[0].trim() || 'Untitled note';
}

function topicBlockAction(block) {
  if (!connectionSourceBlockID) {
    return '<button class="secondary-btn" type="button" data-action="start-connection">Link</button>';
  }
  if (connectionSourceBlockID === block.id) {
    return '<button class="secondary-btn" type="button" data-action="cancel-connection">Cancel</button>';
  }
  return '<button class="primary-btn" type="button" data-action="finish-connection">Connect here</button>';
}

function renderTopicBoard() {
  const hasTopic = Boolean(activeTopicID && activeTopicBoard?.topic);
  renderTopicSelector();
  els.noteTopicEmpty.classList.toggle('hidden', hasTopic);
  els.noteTopicViewport.classList.toggle('hidden', !hasTopic);
  if (!hasTopic) {
    els.noteTopicBlocks.innerHTML = '';
    els.noteTopicEdges.innerHTML = '';
    els.noteTopicEmpty.textContent = noteTopics.length === 0
      ? 'Create a topic to arrange notes as connected blocks.'
      : 'Loading the selected topic…';
    if (noteTopics.length === 0) setTopicStatus('Create your first topic to begin.');
    return;
  }

  const blocks = Array.isArray(activeTopicBoard.blocks) ? activeTopicBoard.blocks : [];
  const surfaceWidth = Math.max(1800, ...blocks.map((block) => (Number(block.position_x) || 0) + 340));
  const surfaceHeight = Math.max(1100, ...blocks.map((block) => (Number(block.position_y) || 0) + 240));
  els.noteTopicSurface.style.width = `${surfaceWidth}px`;
  els.noteTopicSurface.style.height = `${surfaceHeight}px`;
  els.noteTopicBlocks.innerHTML = blocks.map((block) => {
    const taskMeta = block.linked_task_count > 0
      ? `${block.completed_task_count}/${block.linked_task_count} connected tasks complete`
      : 'No connected tasks';
    return `
      <article class="note-topic-block ${block.archived ? 'archived' : ''} ${connectionSourceBlockID === block.id ? 'connection-source' : ''}"
        data-block-id="${escapeHtml(block.id)}" data-note-id="${escapeHtml(block.note_id)}"
        style="left:${Number(block.position_x) || 0}px; top:${Number(block.position_y) || 0}px;">
        <div class="note-topic-block-heading">
          <strong>${block.pinned ? '<span class="note-pin-mark" aria-label="Pinned">★</span>' : ''}${escapeHtml(blockDisplayTitle(block))}</strong>
          ${block.archived ? '<span class="note-state-badge">Archived</span>' : ''}
        </div>
        <div class="note-topic-block-preview">${escapeHtml(block.preview || 'No body text')}</div>
        <div class="note-topic-block-meta">${escapeHtml(taskMeta)}</div>
        <div class="note-topic-block-actions">
          <button class="secondary-btn" type="button" data-action="open-note">Open</button>
          ${topicBlockAction(block)}
          <button class="secondary-btn" type="button" data-action="remove-block">Remove</button>
        </div>
      </article>
    `;
  }).join('');
  requestAnimationFrame(renderTopicEdges);

  const source = blocks.find((block) => block.id === connectionSourceBlockID);
  if (source) {
    setTopicStatus(`Choose “Connect here” on a target for “${blockDisplayTitle(source)}”.`, 'linking');
  } else if (selectedConnectionID) {
    setTopicStatus('Connection selected. Use “Delete selected line” to remove it.');
  } else if (blocks.length === 0) {
    setTopicStatus('This topic is empty. Add an existing note to create its first block.');
  } else {
    setTopicStatus('Drag blocks to arrange them. Use Link on a block to draw a connection.');
  }
  els.noteTopicCancelConnection.classList.toggle('hidden', !connectionSourceBlockID);
  els.noteTopicDeleteConnection.disabled = !selectedConnectionID;
}

function topicEdgePath(fromElement, toElement) {
  const fromCenterX = fromElement.offsetLeft + fromElement.offsetWidth / 2;
  const fromCenterY = fromElement.offsetTop + fromElement.offsetHeight / 2;
  const toCenterX = toElement.offsetLeft + toElement.offsetWidth / 2;
  const toCenterY = toElement.offsetTop + toElement.offsetHeight / 2;
  const forward = toCenterX >= fromCenterX;
  const startX = fromElement.offsetLeft + (forward ? fromElement.offsetWidth : 0);
  const endX = toElement.offsetLeft + (forward ? 0 : toElement.offsetWidth);
  const distance = Math.max(80, Math.abs(endX - startX) * 0.45);
  const firstControlX = startX + (forward ? distance : -distance);
  const secondControlX = endX + (forward ? -distance : distance);
  return `M ${startX} ${fromCenterY} C ${firstControlX} ${fromCenterY}, ${secondControlX} ${toCenterY}, ${endX} ${toCenterY}`;
}

function renderTopicEdges() {
  if (!activeTopicBoard) {
    els.noteTopicEdges.innerHTML = '';
    return;
  }
  const connections = Array.isArray(activeTopicBoard.connections) ? activeTopicBoard.connections : [];
  const paths = connections.map((connection) => {
    const from = els.noteTopicBlocks.querySelector(`[data-block-id="${connection.from_block_id}"]`);
    const to = els.noteTopicBlocks.querySelector(`[data-block-id="${connection.to_block_id}"]`);
    if (!from || !to) return '';
    const path = topicEdgePath(from, to);
    const selected = selectedConnectionID === connection.id ? 'selected' : '';
    const related = connection.relation_type === 'related' ? 'related' : '';
    const marker = connection.relation_type === 'leads_to' ? 'marker-end="url(#note-topic-arrow)"' : '';
    return `
      <path class="note-topic-edge-hit" data-connection-id="${escapeHtml(connection.id)}" d="${path}"></path>
      <path class="note-topic-edge-visible ${related} ${selected}" d="${path}" ${marker}></path>
    `;
  }).join('');
  els.noteTopicEdges.setAttribute('viewBox', `0 0 ${els.noteTopicSurface.offsetWidth} ${els.noteTopicSurface.offsetHeight}`);
  els.noteTopicEdges.innerHTML = `
    <defs>
      <marker id="note-topic-arrow" markerWidth="10" markerHeight="10" refX="8" refY="3" orient="auto" markerUnits="strokeWidth">
        <path d="M0,0 L0,6 L9,3 z" fill="rgba(124, 140, 255, 0.92)"></path>
      </marker>
    </defs>
    ${paths}
  `;
}

function setTopicModalError(message = '') {
  els.noteTopicModalError.textContent = message;
  els.noteTopicModalError.classList.toggle('hidden', !message);
}

function openTopicModal(mode) {
  topicModalMode = mode;
  const topic = selectedTopic();
  els.noteTopicModalHeading.textContent = mode === 'rename' ? 'Rename topic' : 'New topic';
  els.noteTopicModalSave.textContent = mode === 'rename' ? 'Save name' : 'Create topic';
  els.noteTopicModalTitle.value = mode === 'rename' ? (topic?.title || '') : '';
  setTopicModalError();
  els.noteTopicModal.classList.remove('hidden');
  els.noteTopicModal.setAttribute('aria-hidden', 'false');
  els.noteTopicModalTitle.focus();
  els.noteTopicModalTitle.select();
}

function closeTopicModal() {
  els.noteTopicModal.classList.add('hidden');
  els.noteTopicModal.setAttribute('aria-hidden', 'true');
  setTopicModalError();
}

async function saveTopicModal() {
  const title = els.noteTopicModalTitle.value.trim();
  if (!title) {
    setTopicModalError('Enter a topic name.');
    els.noteTopicModalTitle.focus();
    return;
  }
  try {
    const topic = topicModalMode === 'rename'
      ? await renameNoteTopic(activeTopicID, title)
      : await createNoteTopic(title);
    closeTopicModal();
    await loadNoteTopics(topic.id);
  } catch (error) {
    setTopicModalError(error?.message || 'Topic could not be saved.');
  }
}

async function removeSelectedTopic() {
  const topic = selectedTopic();
  if (!topic) return;
  const confirmed = await showConfirmation({
    title: 'Delete topic?',
    message: `“${topic.title}” and its layout will be deleted. Its notes and tasks will be kept.`,
    confirmLabel: 'Delete topic',
  });
  if (!confirmed) return;
  try {
    await deleteNoteTopic(topic.id);
    activeTopicID = null;
    activeTopicBoard = null;
    await loadNoteTopics();
  } catch (error) {
    setTopicStatus(error?.message || 'Topic could not be deleted.', 'error');
  }
}

function setNotePickerError(message = '') {
  els.notePickerError.textContent = message;
  els.notePickerError.classList.toggle('hidden', !message);
}

function openNotePicker() {
  if (!activeTopicID) return;
  els.notePickerSearch.value = '';
  setNotePickerError();
  els.notePickerModal.classList.remove('hidden');
  els.notePickerModal.setAttribute('aria-hidden', 'false');
  els.notePickerSearch.focus();
  void loadNotePickerResults();
}

function closeNotePicker() {
  clearTimeout(notePickerTimer);
  ++notePickerRequest;
  els.notePickerModal.classList.add('hidden');
  els.notePickerModal.setAttribute('aria-hidden', 'true');
}

async function loadNotePickerResults() {
  const request = ++notePickerRequest;
  try {
    const result = await listNotes({
      query: els.notePickerSearch.value.trim(), archive_status: 'all', limit: 200, offset: 0,
    });
    const existing = new Set((activeTopicBoard?.blocks || []).map((block) => block.note_id));
    const available = (Array.isArray(result?.notes) ? result.notes : []).filter((note) => !existing.has(note.id));
    if (request !== notePickerRequest || els.notePickerModal.classList.contains('hidden')) return;
    els.notePickerResults.innerHTML = available.length > 0
      ? available.map((note) => `
          <button class="note-relation-picker-item" type="button" data-note-id="${escapeHtml(note.id)}">
            <strong>${escapeHtml(noteDisplayTitle(note))}</strong>
            <span>${escapeHtml(note.preview || 'No body text')}</span>
          </button>
        `).join('')
      : '<div class="note-task-empty">No available notes match this search.</div>';
  } catch (error) {
    if (request === notePickerRequest) setNotePickerError(error?.message || 'Notes could not be loaded.');
  }
}

async function addNoteToActiveTopic(noteID) {
  const count = activeTopicBoard?.blocks?.length || 0;
  const column = count % 5;
  const row = Math.floor(count / 5);
  try {
    await addNoteTopicBlock({
      topic_id: activeTopicID,
      note_id: noteID,
      position_x: 40 + column * 290,
      position_y: 40 + row * 190,
    });
    closeNotePicker();
    await loadNoteTopics(activeTopicID);
  } catch (error) {
    setNotePickerError(error?.message || 'Note could not be added to this topic.');
  }
}

async function removeTopicBlock(blockID) {
  const block = activeTopicBoard?.blocks?.find((item) => item.id === blockID);
  if (!block) return;
  const confirmed = await showConfirmation({
    title: 'Remove note from topic?',
    message: `“${blockDisplayTitle(block)}” will be removed from this canvas. The note itself will be kept.`,
    confirmLabel: 'Remove block',
  });
  if (!confirmed) return;
  try {
    await deleteNoteTopicBlock(blockID);
    await loadNoteTopics(activeTopicID);
  } catch (error) {
    setTopicStatus(error?.message || 'Note block could not be removed.', 'error');
  }
}

async function finishTopicConnection(toBlockID) {
  if (!connectionSourceBlockID || connectionSourceBlockID === toBlockID) return;
  try {
    await createNoteTopicConnection({
      topic_id: activeTopicID,
      from_block_id: connectionSourceBlockID,
      to_block_id: toBlockID,
      relation_type: els.noteTopicRelationType.value,
    });
    connectionSourceBlockID = null;
    await loadNoteTopics(activeTopicID);
  } catch (error) {
    setTopicStatus(error?.message || 'Connection could not be created.', 'error');
  }
}

async function removeSelectedConnection() {
  if (!selectedConnectionID) return;
  try {
    await deleteNoteTopicConnection(selectedConnectionID);
    selectedConnectionID = null;
    await loadNoteTopics(activeTopicID);
  } catch (error) {
    setTopicStatus(error?.message || 'Connection could not be deleted.', 'error');
  }
}

function beginTopicBlockDrag(event) {
  if (event.button !== 0 || event.target.closest('button')) return;
  const element = event.target.closest('[data-block-id]');
  if (!element) return;
  const blockID = Number(element.dataset.blockId);
  const block = activeTopicBoard?.blocks?.find((item) => item.id === blockID);
  if (!block) return;
  const startPointerX = event.clientX;
  const startPointerY = event.clientY;
  const startX = Number(block.position_x) || 0;
  const startY = Number(block.position_y) || 0;
  let moved = false;
  event.preventDefault();
  element.setPointerCapture?.(event.pointerId);

  const move = (moveEvent) => {
    const dx = moveEvent.clientX - startPointerX;
    const dy = moveEvent.clientY - startPointerY;
    moved = moved || Math.abs(dx) > 2 || Math.abs(dy) > 2;
    block.position_x = Math.max(0, Math.min(els.noteTopicSurface.offsetWidth - element.offsetWidth, startX + dx));
    block.position_y = Math.max(0, Math.min(els.noteTopicSurface.offsetHeight - element.offsetHeight, startY + dy));
    element.style.left = `${block.position_x}px`;
    element.style.top = `${block.position_y}px`;
    renderTopicEdges();
  };
  const end = async () => {
    document.removeEventListener('pointermove', move);
    document.removeEventListener('pointerup', end);
    if (!moved) return;
    try {
      await updateNoteTopicBlockPosition({
        block_id: block.id,
        position_x: block.position_x,
        position_y: block.position_y,
      });
    } catch (error) {
      setTopicStatus(error?.message || 'Block position could not be saved.', 'error');
      await loadActiveTopicBoard();
    }
  };
  document.addEventListener('pointermove', move);
  document.addEventListener('pointerup', end, { once: true });
}

function setTaskPickerError(message = '') {
  els.noteTaskPickerError.textContent = message;
  els.noteTaskPickerError.classList.toggle('hidden', !message);
}

let taskPickerTodos = [];

function renderTaskPickerResults() {
  const query = els.noteTaskPickerSearch.value.trim().toLowerCase();
  const connected = new Set(activeNoteTodos.map((todo) => todo.id));
  const available = taskPickerTodos.filter((todo) => {
    if (connected.has(todo.id)) return false;
    return !query || String(todo.title || '').toLowerCase().includes(query) ||
      String(todo.description || '').toLowerCase().includes(query);
  });
  els.noteTaskPickerResults.innerHTML = available.length > 0
    ? available.map((todo) => `
        <button class="note-relation-picker-item" type="button" data-todo-id="${escapeHtml(todo.id)}">
          <strong>${escapeHtml(todo.title || 'Untitled task')}</strong>
          <span>${todo.done ? 'Completed' : 'Pending'}${todo.description ? ` · ${escapeHtml(todo.description)}` : ''}</span>
        </button>
      `).join('')
    : '<div class="note-task-empty">No available tasks match this search.</div>';
}

async function openTaskPicker() {
  if (!(await flushNoteSave()) || !activeNote?.id) {
    setNoteError('Save the note before connecting a task.');
    return;
  }
  els.noteTaskPickerSearch.value = '';
  setTaskPickerError();
  els.noteTaskPickerModal.classList.remove('hidden');
  els.noteTaskPickerModal.setAttribute('aria-hidden', 'false');
  els.noteTaskPickerResults.innerHTML = '<div class="note-task-empty">Loading tasks…</div>';
  els.noteTaskPickerSearch.focus();
  try {
    const items = await getTodos();
    taskPickerTodos = Array.isArray(items) ? items : [];
    renderTaskPickerResults();
  } catch (error) {
    setTaskPickerError(error?.message || 'Tasks could not be loaded.');
  }
}

function closeTaskPicker() {
  els.noteTaskPickerModal.classList.add('hidden');
  els.noteTaskPickerModal.setAttribute('aria-hidden', 'true');
  taskPickerTodos = [];
}

async function connectTaskToActiveNote(todoID) {
  const noteID = activeNote?.id;
  if (!noteID) return;
  try {
    await linkNoteTodo(noteID, todoID);
    closeTaskPicker();
    await loadActiveNoteTodos();
    if (activeTopicID) void loadNoteTopics(activeTopicID);
  } catch (error) {
    setTaskPickerError(error?.message || 'Task could not be connected.');
  }
}

async function createTaskForActiveNote() {
  if (!(await flushNoteSave()) || !activeNote?.id) {
    setNoteError('Save the note before creating a connected task.');
    return;
  }
  const noteID = activeNote.id;
  openTodoModal(null, {
    onSaved: async (todo) => {
      if (!todo?.id) throw new Error('The created task could not be identified.');
      await linkNoteTodo(noteID, todo.id);
      if (activeNote?.id === noteID) await loadActiveNoteTodos();
      if (activeTopicID) void loadNoteTopics(activeTopicID);
    },
  });
}

async function openNoteFromTopic(noteID) {
  await setNotesMode('library');
  await selectNote(noteID);
}

function listenForNoteChanges() {
  if (!hasWailsBinding() || !window.runtime?.EventsOn) return;
  window.runtime.EventsOn(NOTES_CHANGED_EVENT, () => {
    if (dirty && activeNote) {
      conflict = true;
      els.notesConflict.textContent = 'Notes changed through another interface. Save is paused until you reload this note.';
      els.notesConflict.classList.remove('hidden');
      els.notesReload.classList.remove('hidden');
      setSaveState('External change', 'error');
      void loadNotes();
      if (activeTopicID) void loadActiveTopicBoard();
      return;
    }
    const selectedID = activeNote?.id;
    void loadNotes().then(() => selectedID ? reloadActiveNote() : undefined);
    if (activeTopicID) void loadActiveTopicBoard();
  });
}

export function initNotes() {
  els.notesLibraryTab.addEventListener('click', () => void setNotesMode('library'));
  els.notesTopicsTab.addEventListener('click', () => void setNotesMode('topics'));
  els.notesNew.addEventListener('click', startNewNote);
  els.notesTitle.addEventListener('input', markNoteDirty);
  els.notesBody.addEventListener('input', markNoteDirty);
  els.notesList.addEventListener('click', (event) => {
    const item = event.target.closest('[data-note-id]');
    if (item) void selectNote(Number(item.dataset.noteId));
  });
  els.notesLoadMore.addEventListener('click', () => void loadNotes({ append: true }));
  els.notesArchiveFilter.addEventListener('change', () => void loadNotes());
  els.notesPinnedFilter.addEventListener('change', () => void loadNotes());
  els.notesSearch.addEventListener('input', () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => void loadNotes(), 250);
  });
  els.notesPin.addEventListener('click', () => void changeNoteState('pin'));
  els.notesArchive.addEventListener('click', () => void changeNoteState('archive'));
  els.notesDelete.addEventListener('click', () => void removeActiveNote());
  els.notesReload.addEventListener('click', () => void reloadActiveNote());
  els.noteTaskConnect.addEventListener('click', () => void openTaskPicker());
  els.noteTaskCreate.addEventListener('click', () => void createTaskForActiveNote());
  els.noteTaskList.addEventListener('click', (event) => {
    const row = event.target.closest('[data-todo-id]');
    if (!row) return;
    const todoID = Number(row.dataset.todoId);
    if (event.target.closest('[data-action="open-task"]')) {
      const todo = activeNoteTodos.find((item) => item.id === todoID);
      if (todo) openTodoModal(todo);
      return;
    }
    if (event.target.closest('[data-action="unlink-task"]') && activeNote?.id) {
      void unlinkNoteTodo(activeNote.id, todoID).then(async () => {
        await loadActiveNoteTodos();
        if (activeTopicID) void loadNoteTopics(activeTopicID);
      }).catch((error) => setNoteError(error?.message || 'Task could not be disconnected.'));
    }
  });

  els.noteTopicSelect.addEventListener('change', () => {
    activeTopicID = Number(els.noteTopicSelect.value) || null;
    connectionSourceBlockID = null;
    selectedConnectionID = null;
    void loadActiveTopicBoard();
  });
  els.noteTopicNew.addEventListener('click', () => openTopicModal('create'));
  els.noteTopicRename.addEventListener('click', () => openTopicModal('rename'));
  els.noteTopicDelete.addEventListener('click', () => void removeSelectedTopic());
  els.noteTopicAddNote.addEventListener('click', openNotePicker);
  els.noteTopicCancelConnection.addEventListener('click', () => {
    connectionSourceBlockID = null;
    renderTopicBoard();
  });
  els.noteTopicDeleteConnection.addEventListener('click', () => void removeSelectedConnection());
  els.noteTopicBlocks.addEventListener('pointerdown', beginTopicBlockDrag);
  els.noteTopicBlocks.addEventListener('click', (event) => {
    const element = event.target.closest('[data-block-id]');
    const action = event.target.closest('[data-action]')?.dataset.action;
    if (!element || !action) return;
    const blockID = Number(element.dataset.blockId);
    const noteID = Number(element.dataset.noteId);
    if (action === 'open-note') {
      void openNoteFromTopic(noteID);
    } else if (action === 'start-connection') {
      connectionSourceBlockID = blockID;
      selectedConnectionID = null;
      renderTopicBoard();
    } else if (action === 'cancel-connection') {
      connectionSourceBlockID = null;
      renderTopicBoard();
    } else if (action === 'finish-connection') {
      void finishTopicConnection(blockID);
    } else if (action === 'remove-block') {
      void removeTopicBlock(blockID);
    }
  });
  els.noteTopicEdges.addEventListener('click', (event) => {
    const edge = event.target.closest('[data-connection-id]');
    if (!edge) return;
    connectionSourceBlockID = null;
    selectedConnectionID = Number(edge.dataset.connectionId);
    renderTopicBoard();
  });

  [els.noteTopicModalClose, els.noteTopicModalCancel, els.noteTopicModalBackdrop]
    .forEach((element) => element.addEventListener('click', closeTopicModal));
  els.noteTopicModalSave.addEventListener('click', () => void saveTopicModal());
  els.noteTopicModalTitle.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') void saveTopicModal();
  });

  [els.notePickerModalClose, els.notePickerModalCancel, els.notePickerModalBackdrop]
    .forEach((element) => element.addEventListener('click', closeNotePicker));
  els.notePickerSearch.addEventListener('input', () => {
    clearTimeout(notePickerTimer);
    notePickerTimer = setTimeout(() => void loadNotePickerResults(), 200);
  });
  els.notePickerResults.addEventListener('click', (event) => {
    const item = event.target.closest('[data-note-id]');
    if (item) void addNoteToActiveTopic(Number(item.dataset.noteId));
  });

  [els.noteTaskPickerModalClose, els.noteTaskPickerModalCancel, els.noteTaskPickerModalBackdrop]
    .forEach((element) => element.addEventListener('click', closeTaskPicker));
  els.noteTaskPickerSearch.addEventListener('input', renderTaskPickerResults);
  els.noteTaskPickerResults.addEventListener('click', (event) => {
    const item = event.target.closest('[data-todo-id]');
    if (item) void connectTaskToActiveNote(Number(item.dataset.todoId));
  });

  window.addEventListener('resize', () => requestAnimationFrame(renderTopicEdges));
  document.addEventListener(TODOS_CHANGED_EVENT, () => {
    if (activeNote?.id) void loadActiveNoteTodos();
    if (activeTopicID) void loadActiveTopicBoard();
  });
  document.addEventListener('notes:open', (event) => {
    const noteID = Number(event.detail?.note_id);
    if (!noteID) return;
    document.querySelector('.menu-btn[data-view="notes"]')?.click();
    void setNotesMode('library').then(() => selectNote(noteID));
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      if (!els.noteTopicModal.classList.contains('hidden')) closeTopicModal();
      if (!els.notePickerModal.classList.contains('hidden')) closeNotePicker();
      if (!els.noteTaskPickerModal.classList.contains('hidden')) closeTaskPicker();
      if (connectionSourceBlockID) {
        connectionSourceBlockID = null;
        renderTopicBoard();
      }
    }
    const notesActive = document.getElementById('view-notes')?.classList.contains('active');
    if (!notesActive) return;
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault();
      void flushNoteSave();
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'n') {
      event.preventDefault();
      void setNotesMode('library').then(startNewNote);
    }
  });
  listenForNoteChanges();
}
