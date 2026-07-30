import { els } from './dom.js';
import {
  listNotes,
  getNote,
  createNote,
  updateNote,
  setNotePinned,
  setNoteArchived,
  deleteNote,
} from './api.js';
import { escapeHtml, hasWailsBinding } from './utils.js';

const NOTES_CHANGED_EVENT = 'notes:changed';
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
    renderEditor();
    renderNoteList();
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
  if (!window.confirm(`Permanently delete “${noteDisplayTitle(activeNote)}”?`)) return;
  try {
    await deleteNote(activeNote.id);
    activeNote = null;
    newDraft = false;
    dirty = false;
    conflict = false;
    await loadNotes();
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
  } catch (error) {
    setNoteError(error?.message || 'Latest note could not be loaded.');
  }
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
      return;
    }
    const selectedID = activeNote?.id;
    void loadNotes().then(() => selectedID ? reloadActiveNote() : undefined);
  });
}

export function initNotes() {
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
  document.addEventListener('keydown', (event) => {
    const notesActive = document.getElementById('view-notes')?.classList.contains('active');
    if (!notesActive) return;
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault();
      void flushNoteSave();
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'n') {
      event.preventDefault();
      startNewNote();
    }
  });
  listenForNoteChanges();
}
