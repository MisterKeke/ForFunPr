import {
  cancelGitWorkspaceJob,
  chooseGitWorkspaceFolder,
  getGitRepositoryDetails,
  getGitRepositoryHistory,
  getGitWorkspaceJob,
  getGitWorkspaceSettings,
  listDesktopApps,
  listGitRepositories,
  listGitWorkspaces,
  openGitRepositoryFolder,
  openGitRepositoryInEditor,
  openGitRepositoryRemote,
  pruneMissingGitRepositories,
  removeGitWorkspace,
  startGitFetch,
  startGitPull,
  startGitStatusRefresh,
  startGitSync,
  startGitWorkspaceRescan,
  updateGitWorkspaceSettings,
} from './api.js';
import { els } from './dom.js';
import { showConfirmation } from './ui.js';
import { hasWailsBinding } from './utils.js';

const EVENTS = {
  inventory: 'git-workspaces:inventory-changed',
  status: 'git-workspaces:status-updated',
  progress: 'git-workspaces:job-progress',
  complete: 'git-workspaces:job-complete',
};

const JOB_LABELS = {
  scan: 'Rescanning repositories',
  status_refresh: 'Refreshing repository status',
  fetch: 'Fetching remotes',
  pull: 'Pulling repositories',
  sync: 'Syncing repositories',
};

const state = {
  initialized: false,
  loading: false,
  repositories: [],
  workspaces: [],
  settings: null,
  desktopApps: [],
  activeJob: null,
  detailRepositoryID: 0,
  detailPreviousFocus: null,
  eventsBound: false,
  initialLoadPromise: null,
};

function value(source, ...keys) {
  for (const key of keys) {
    if (source?.[key] !== undefined && source?.[key] !== null) return source[key];
  }
  return undefined;
}

function clean(valueToClean) {
  return String(valueToClean ?? '').trim();
}

function numberValue(valueToRead, fallback = 0) {
  const number = Number(valueToRead);
  return Number.isFinite(number) ? number : fallback;
}

function node(tag, className = '', text = '') {
  const element = document.createElement(tag);
  if (className) element.className = className;
  if (text !== '') element.textContent = String(text);
  return element;
}

function button(label, action, repositoryID = 0, className = 'secondary-btn small-btn') {
  const element = node('button', className, label);
  element.type = 'button';
  if (action) element.dataset.gitAction = action;
  if (repositoryID) element.dataset.repositoryId = String(repositoryID);
  return element;
}

function normalizeWorkspace(workspace) {
  return {
    id: numberValue(value(workspace, 'id'), 0),
    display_name: clean(value(workspace, 'display_name', 'displayName')) || 'Workspace',
    last_scanned_at: clean(value(workspace, 'last_scanned_at', 'lastScannedAt')),
    revision: numberValue(value(workspace, 'revision'), 0),
  };
}

function normalizeStatus(status) {
  if (!status) return null;
  return {
    branch: clean(value(status, 'branch')),
    dirty: Boolean(value(status, 'dirty')),
    modified_count: numberValue(value(status, 'modified_count', 'modifiedCount')),
    added_count: numberValue(value(status, 'added_count', 'addedCount')),
    deleted_count: numberValue(value(status, 'deleted_count', 'deletedCount')),
    renamed_count: numberValue(value(status, 'renamed_count', 'renamedCount')),
    untracked_count: numberValue(value(status, 'untracked_count', 'untrackedCount')),
    upstream: clean(value(status, 'upstream')),
    ahead_count: numberValue(value(status, 'ahead_count', 'aheadCount')),
    behind_count: numberValue(value(status, 'behind_count', 'behindCount')),
    sync_state: clean(value(status, 'sync_state', 'syncState')).toLowerCase(),
    remote_display: clean(value(status, 'remote_display', 'remoteDisplay')),
    remote_web_url: clean(value(status, 'remote_web_url', 'remoteWebURL')),
    latest_commit_summary: clean(value(status, 'latest_commit_summary', 'latestCommitSummary')),
    checked_at: clean(value(status, 'checked_at', 'checkedAt')),
  };
}

function normalizeRepository(repository, workspaceIDs = [], workspaceNames = []) {
  return {
    id: numberValue(value(repository, 'id'), 0),
    name: clean(value(repository, 'name')) || 'Untitled repository',
    missing: Boolean(value(repository, 'missing')),
    last_seen_at: clean(value(repository, 'last_seen_at', 'lastSeenAt')),
    workspace_ids: [...new Set(workspaceIDs.map(numberValue).filter(Boolean))],
    workspace_names: [...new Set(workspaceNames.map(clean).filter(Boolean))],
    cached_status: normalizeStatus(value(repository, 'cached_status', 'cachedStatus')),
  };
}

function normalizeJob(job) {
  if (!job) return null;
  return {
    id: clean(value(job, 'id')),
    kind: clean(value(job, 'kind')).toLowerCase(),
    state: clean(value(job, 'state')).toLowerCase(),
    workspace_id: numberValue(value(job, 'workspace_id', 'workspaceId')),
    attempted: numberValue(value(job, 'attempted')),
    completed: numberValue(value(job, 'completed')),
    succeeded: numberValue(value(job, 'succeeded')),
    skipped: numberValue(value(job, 'skipped')),
    failed: numberValue(value(job, 'failed')),
    error: clean(value(job, 'error')),
  };
}

function isJobActive(job = state.activeJob) {
  return Boolean(job && (job.state === 'queued' || job.state === 'running'));
}

function showError(message = '') {
  if (!els.gitWorkspaceError) return;
  els.gitWorkspaceError.textContent = clean(message);
  els.gitWorkspaceError.classList.toggle('hidden', !clean(message));
}

function showRootError(message = '') {
  if (!els.gitWorkspaceRootError) return;
  els.gitWorkspaceRootError.textContent = clean(message);
  els.gitWorkspaceRootError.classList.toggle('hidden', !clean(message));
}

function showStatus(message = '') {
  if (!els.gitWorkspaceStatus) return;
  els.gitWorkspaceStatus.textContent = clean(message);
}

function formatDate(valueToFormat) {
  const source = clean(valueToFormat);
  if (!source) return 'Never';
  const date = new Date(source.includes(' ') && !source.includes('T') ? `${source.replace(' ', 'T')}Z` : source);
  return Number.isNaN(date.getTime()) ? source : date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

function workspaceNames(repository) {
  return repository.workspace_names.length > 0 ? repository.workspace_names.join(', ') : 'Unassigned';
}

function selectedWorkspaceID() {
  return numberValue(els.gitWorkspaceWorkspaceFilter?.value, 0);
}

function visibleRepositories() {
  const query = clean(els.gitWorkspaceSearch?.value).toLowerCase();
  const workspaceID = selectedWorkspaceID();
  const branch = clean(els.gitWorkspaceBranchFilter?.value);
  const workingTree = clean(els.gitWorkspaceStatusFilter?.value) || 'all';
  const sync = clean(els.gitWorkspaceSyncFilter?.value) || 'all';
  return state.repositories.filter((repository) => {
    const status = repository.cached_status;
    const searchText = [repository.name, workspaceNames(repository), status?.branch, status?.remote_display].join(' ').toLowerCase();
    if (query && !searchText.includes(query)) return false;
    if (workspaceID && !repository.workspace_ids.includes(workspaceID)) return false;
    if (branch && branch !== 'all' && clean(status?.branch) !== branch) return false;
    if (workingTree === 'missing' && !repository.missing) return false;
    if (workingTree === 'clean' && (repository.missing || !status || status.dirty)) return false;
    if (workingTree === 'dirty' && (repository.missing || !status?.dirty)) return false;
    if (workingTree === 'unknown' && (repository.missing || status)) return false;
    if (sync !== 'all' && clean(status?.sync_state) !== sync) return false;
    return true;
  });
}

function renderCapability() {
  if (!els.gitWorkspaceGitAvailability) return;
  const capability = window.somethingCapabilities?.git_workspaces;
  if (!capability) {
    els.gitWorkspaceGitAvailability.textContent = hasWailsBinding()
      ? 'Git availability will be reported after the desktop backend starts.'
      : 'Git Workspaces are available only in the Something desktop app.';
    els.gitWorkspaceGitAvailability.dataset.state = hasWailsBinding() ? 'unknown' : 'unavailable';
    return;
  }
  els.gitWorkspaceGitAvailability.textContent = capability.available
    ? 'Git is available. Repository operations are ready.'
    : (capability.warning || 'Git is unavailable on this system.');
  els.gitWorkspaceGitAvailability.dataset.state = capability.available ? 'ready' : 'unavailable';
}

function renderWorkspaceOptions() {
  if (!els.gitWorkspaceWorkspaceFilter) return;
  const selected = selectedWorkspaceID();
  els.gitWorkspaceWorkspaceFilter.replaceChildren();
  els.gitWorkspaceWorkspaceFilter.append(node('option', '', 'All workspaces'));
  state.workspaces.forEach((workspace) => {
    const option = node('option', '', workspace.display_name);
    option.value = String(workspace.id);
    els.gitWorkspaceWorkspaceFilter.append(option);
  });
  els.gitWorkspaceWorkspaceFilter.value = selected && state.workspaces.some((item) => item.id === selected)
    ? String(selected) : '';
}

function renderBranchOptions() {
  if (!els.gitWorkspaceBranchFilter) return;
  const selected = clean(els.gitWorkspaceBranchFilter.value);
  const branches = [...new Set(state.repositories.map((item) => clean(item.cached_status?.branch)).filter(Boolean))].sort((a, b) => a.localeCompare(b));
  els.gitWorkspaceBranchFilter.replaceChildren(node('option', '', 'All branches'));
  els.gitWorkspaceBranchFilter.firstElementChild.value = 'all';
  branches.forEach((branch) => {
    const option = node('option', '', branch);
    option.value = branch;
    els.gitWorkspaceBranchFilter.append(option);
  });
  els.gitWorkspaceBranchFilter.value = branches.includes(selected) ? selected : 'all';
}

function renderSummary() {
  if (!els.gitWorkspaceSummary) return;
  const totals = { total: state.repositories.length, clean: 0, dirty: 0, ahead: 0, behind: 0, diverged: 0, noUpstream: 0, missing: 0 };
  state.repositories.forEach((repository) => {
    if (repository.missing) totals.missing += 1;
    const status = repository.cached_status;
    if (!status) return;
    if (status.dirty) totals.dirty += 1;
    else totals.clean += 1;
    if (status.sync_state === 'ahead') totals.ahead += 1;
    if (status.sync_state === 'behind') totals.behind += 1;
    if (status.sync_state === 'diverged') totals.diverged += 1;
    if (status.sync_state === 'no-upstream') totals.noUpstream += 1;
  });
  const cards = [
    ['total', totals.total, 'Total', 'accent'],
    ['clean', totals.clean, 'Clean', 'success'],
    ['dirty', totals.dirty, 'Dirty', 'warning'],
    ['ahead', totals.ahead, 'Ahead', 'accent'],
    ['behind', totals.behind, 'Behind', 'warning'],
    ['diverged', totals.diverged, 'Diverged', 'error'],
    ['no-upstream', totals.noUpstream, 'No upstream', 'warning'],
    ['missing', totals.missing, 'Missing', 'error'],
  ];
  els.gitWorkspaceSummary.replaceChildren(...cards.map(([key, total, label, tone]) => {
    const card = node('div', 'git-workspace-summary-card');
    card.dataset.tone = tone;
    card.dataset.summary = key;
    card.append(node('strong', '', total), node('span', '', label));
    return card;
  }));
}

function statusPill(text, tone = '') {
  return node('span', `git-workspace-pill${tone ? ` ${tone}` : ''}`, text);
}

function syncTone(syncState) {
  if (syncState === 'synced') return 'success';
  if (syncState === 'diverged') return 'error';
  if (syncState === 'ahead' || syncState === 'behind' || syncState === 'no-upstream') return 'warning';
  return '';
}

function changeCounts(status) {
  const wrapper = node('div', 'git-workspace-change-counts');
  if (!status) return node('span', 'git-workspace-muted', '—');
  const counts = [
    ['M', status.modified_count], ['A', status.added_count], ['D', status.deleted_count],
    ['R', status.renamed_count], ['U', status.untracked_count],
  ];
  counts.filter(([, count]) => count > 0).forEach(([label, count]) => {
    const item = node('span');
    item.append(node('strong', '', label), document.createTextNode(` ${count}`));
    wrapper.append(item);
  });
  if (!wrapper.childElementCount) wrapper.append(node('span', 'git-workspace-muted', 'No changes'));
  return wrapper;
}

function tableCell(row, label, content, className = '') {
  const cell = node('td', className);
  cell.dataset.label = label;
  if (content instanceof Node) cell.append(content);
  else cell.append(node('span', '', content));
  row.append(cell);
  return cell;
}

function renderRepositories() {
  if (!els.gitWorkspaceList) return;
  const repositories = visibleRepositories();
  els.gitWorkspaceList.replaceChildren();
  if (state.loading && state.repositories.length === 0) return;
  if (state.repositories.length === 0) {
    els.gitWorkspaceEmpty?.classList.remove('hidden');
    return;
  }
  els.gitWorkspaceEmpty?.classList.add('hidden');
  if (repositories.length === 0) {
    const empty = node('section', 'card git-workspace-empty');
    empty.append(node('h2', '', 'No repositories match these filters'), node('p', '', 'Try a different workspace, branch, or search term.'));
    els.gitWorkspaceList.append(empty);
    return;
  }

  const wrap = node('div', 'git-workspace-table-wrap');
  const table = node('table', 'git-workspace-table');
  table.append(node('caption', 'sr-only', 'Tracked Git repositories'));
  const head = node('thead');
  const headerRow = node('tr');
  ['Repository', 'Workspace', 'Branch', 'Working tree', 'Changes', 'Sync', 'Ahead / behind', 'Last checked', 'Actions'].forEach((label) => headerRow.append(node('th', '', label)));
  head.append(headerRow);
  const body = node('tbody');
  const jobBusy = isJobActive();
  repositories.forEach((repository) => {
    const row = node('tr');
    const status = repository.cached_status;
    const repositoryCopy = node('div');
    repositoryCopy.append(node('strong', 'git-workspace-repository-name', repository.name));
    if (repository.missing) repositoryCopy.append(statusPill('Missing', 'error'));
    tableCell(row, 'Repository', repositoryCopy);
    tableCell(row, 'Workspace', workspaceNames(repository));
    tableCell(row, 'Branch', status?.branch || 'Not checked');
    tableCell(row, 'Working tree', repository.missing ? statusPill('Missing', 'error') : status ? statusPill(status.dirty ? 'Dirty' : 'Clean', status.dirty ? 'warning' : 'success') : statusPill('Not checked'));
    tableCell(row, 'Changes', changeCounts(status));
    tableCell(row, 'Sync', repository.missing ? statusPill('Unavailable', 'error') : status ? statusPill(status.sync_state || 'Unknown', syncTone(status.sync_state)) : statusPill('Not checked'));
    const aheadBehind = node('span', 'git-workspace-cell-main', status ? `${status.ahead_count} / ${status.behind_count}` : '—');
    tableCell(row, 'Ahead / behind', aheadBehind);
    tableCell(row, 'Last checked', formatDate(status?.checked_at));

    const actionsCell = node('td', 'git-workspace-actions-cell');
    actionsCell.dataset.label = 'Actions';
    const actions = node('div', 'git-workspace-row-actions');
    actions.append(button('Details', 'details', repository.id));
    const folder = button('Folder', 'folder', repository.id);
    const editor = button('Editor', 'editor', repository.id);
    const remote = button('Remote', 'remote', repository.id);
    folder.disabled = repository.missing || jobBusy;
    editor.disabled = repository.missing || jobBusy;
    remote.disabled = repository.missing || !status?.remote_web_url || jobBusy;
    actions.append(folder, editor, remote);
    const fetch = button('Fetch', 'fetch', repository.id);
    const pull = button('Pull', 'pull', repository.id);
    const sync = button('Sync', 'sync', repository.id);
    fetch.disabled = repository.missing || jobBusy;
    pull.disabled = repository.missing || jobBusy;
    sync.disabled = repository.missing || jobBusy;
    actions.append(fetch, pull, sync);
    actionsCell.append(actions);
    row.append(actionsCell);
    body.append(row);
  });
  table.append(head, body);
  wrap.append(table);
  els.gitWorkspaceList.append(wrap);
}

function renderRoots() {
  if (!els.gitWorkspaceRoots) return;
  els.gitWorkspaceRoots.replaceChildren();
  if (state.workspaces.length === 0) {
    els.gitWorkspaceRoots.append(node('p', 'git-workspace-muted', 'No workspace roots are being tracked.'));
    return;
  }
  state.workspaces.forEach((workspace) => {
    const row = node('div', 'git-workspace-root');
    const copy = node('div', 'git-workspace-root-copy');
    copy.append(node('strong', '', workspace.display_name), node('span', '', `Last scanned ${formatDate(workspace.last_scanned_at)}`));
    const actions = node('div', 'git-workspace-root-actions');
    const rescan = button('Rescan', 'rescan-root', workspace.id);
    const remove = button('Remove', 'remove-root', workspace.id, 'danger-btn small-btn');
    rescan.disabled = isJobActive();
    remove.disabled = isJobActive();
    actions.append(rescan, remove);
    row.append(copy, actions);
    els.gitWorkspaceRoots.append(row);
  });
}

function renderJob() {
  if (!els.gitWorkspaceJob) return;
  const job = state.activeJob;
  els.gitWorkspaceJob.classList.toggle('hidden', !job);
  if (!job) return;
  const running = isJobActive(job);
  els.gitWorkspaceJob.dataset.running = String(running);
  els.gitWorkspaceJobTitle.textContent = JOB_LABELS[job.kind] || 'Git workspace job';
  els.gitWorkspaceCancel.disabled = !running;
  els.gitWorkspaceCancel.setAttribute('aria-label', running ? 'Cancel Git workspace job' : 'Job is no longer running');
  const processed = job.completed || job.attempted;
  els.gitWorkspaceJobProgress.style.width = running ? (processed > 0 ? '42%' : '22%') : '100%';
  const outcome = `${job.succeeded} succeeded · ${job.skipped} skipped · ${job.failed} failed`;
  const stateLabel = job.state === 'complete' ? 'Complete' : job.state === 'failed' ? 'Failed' : job.state === 'cancelled' ? 'Canceled' : 'In progress';
  els.gitWorkspaceJobStatus.textContent = `${stateLabel}. ${processed} processed. ${outcome}${job.error ? ` ${job.error}` : ''}`;
  [els.gitWorkspaceAdd, els.gitWorkspaceRescan, els.gitWorkspaceRefreshStatus, els.gitWorkspaceFetch, els.gitWorkspaceSync, els.gitWorkspacePrune].forEach((control) => {
    if (control) control.disabled = running;
  });
}

function render() {
  renderCapability();
  renderWorkspaceOptions();
  renderBranchOptions();
  renderSummary();
  renderRepositories();
  renderRoots();
  renderJob();
}

function mergeInventory(rootResults, allResult, preserveCache) {
  const previous = new Map(state.repositories.map((repository) => [repository.id, repository]));
  const combined = new Map();
  const addRepository = (raw, workspace) => {
    const repositoryID = numberValue(value(raw, 'id'), 0);
    const current = combined.get(repositoryID) || (preserveCache ? previous.get(repositoryID) : null) || normalizeRepository(raw);
    const incoming = normalizeRepository(raw, workspace ? [workspace.id] : [], workspace ? [workspace.display_name] : []);
    if (!incoming.cached_status && previous.get(repositoryID)?.cached_status) {
      current.cached_status = previous.get(repositoryID).cached_status;
    }
    current.name = incoming.name;
    current.missing = incoming.missing;
    current.last_seen_at = incoming.last_seen_at;
    if (incoming.cached_status) current.cached_status = incoming.cached_status;
    current.workspace_ids = [...new Set([...current.workspace_ids, ...incoming.workspace_ids])];
    current.workspace_names = [...new Set([...current.workspace_names, ...incoming.workspace_names])];
    combined.set(current.id, current);
  };
  if (allResult?.status === 'fulfilled') (Array.isArray(allResult.value) ? allResult.value : []).forEach((repository) => addRepository(repository));
  rootResults.forEach((result) => {
    if (result.status !== 'fulfilled') return;
    const workspace = result.workspace;
    (Array.isArray(result.value) ? result.value : []).forEach((repository) => addRepository(repository, workspace));
  });
  if (preserveCache) {
    previous.forEach((repository, id) => {
      if (!combined.has(id)) combined.set(id, repository);
    });
  }
  state.repositories = [...combined.values()].filter((repository) => repository.id > 0).sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
}

async function loadCachedStatuses() {
  const results = await Promise.allSettled(state.repositories.map((repository) => getGitRepositoryDetails(repository.id)));
  results.forEach((result, index) => {
    if (result.status !== 'fulfilled') return;
    const cached = value(result.value, 'cached_status', 'cachedStatus');
    if (cached) state.repositories[index].cached_status = normalizeStatus(cached);
  });
}

async function loadInventory() {
  const roots = [...state.workspaces];
  const allPromise = listGitRepositories({});
  const rootPromises = roots.map((workspace) => listGitRepositories({ workspace_id: workspace.id }).then((valueToUse) => ({ status: 'fulfilled', value: valueToUse, workspace })).catch((error) => ({ status: 'rejected', reason: error, workspace })));
  const [allResult, ...rootResults] = await Promise.all([allPromise.then((valueToUse) => ({ status: 'fulfilled', value: valueToUse })).catch((error) => ({ status: 'rejected', reason: error })), ...rootPromises]);
  const failed = allResult.status === 'rejected' || rootResults.some((result) => result.status === 'rejected');
  mergeInventory(rootResults, allResult, failed);
  await loadCachedStatuses();
  if (failed) {
    showError('Some workspace results could not be refreshed. Cached repositories were kept.');
    showStatus('Partial refresh; showing the last available results.');
  }
}

async function loadSettings() {
  try {
    const settingsResponse = await getGitWorkspaceSettings();
    state.settings = value(settingsResponse, 'settings') || settingsResponse;
    state.settings = {
      editor_application_id: value(state.settings, 'editor_application_id', 'editorApplicationID') ?? null,
      worker_count: numberValue(value(state.settings, 'worker_count', 'workerCount'), 1),
      stale_days: numberValue(value(state.settings, 'stale_days', 'staleDays'), 30),
      revision: numberValue(value(state.settings, 'revision'), 0),
    };
    const desktopApps = await listDesktopApps();
    state.desktopApps = Array.isArray(desktopApps) ? desktopApps : [];
    els.gitWorkspaceEditor.replaceChildren(node('option', '', 'No editor selected'));
    els.gitWorkspaceEditor.firstElementChild.value = '';
    state.desktopApps.forEach((app) => {
      const option = node('option', '', clean(value(app, 'display_name', 'displayName', 'name')) || 'Saved application');
      option.value = String(numberValue(value(app, 'id'), 0));
      els.gitWorkspaceEditor.append(option);
    });
    els.gitWorkspaceEditor.value = state.settings.editor_application_id ? String(state.settings.editor_application_id) : '';
    els.gitWorkspaceWorkers.value = String(state.settings.worker_count);
    els.gitWorkspaceStaleDays.value = String(state.settings.stale_days);
  } catch (error) {
    els.gitWorkspaceSettingsStatus.textContent = clean(error?.message || error);
  }
}

export async function loadGitWorkspaces() {
  renderCapability();
  if (state.initialized) {
    if (isJobActive()) {
      try { state.activeJob = normalizeJob(await getGitWorkspaceJob(state.activeJob.id)); } catch { /* the completion event will settle the view */ }
      renderJob();
    }
    return state.initialLoadPromise || Promise.resolve();
  }
  if (state.initialLoadPromise) return state.initialLoadPromise;
  state.initialLoadPromise = (async () => {
    state.loading = true;
    els.gitWorkspaceLoading?.classList.remove('hidden');
    showError('');
    try {
      const workspaceResponse = await listGitWorkspaces();
      state.workspaces = (Array.isArray(workspaceResponse) ? workspaceResponse : []).map(normalizeWorkspace);
      renderRoots();
      renderWorkspaceOptions();
      await loadInventory();
      await loadSettings();
      state.initialized = true;
      render();
      if (state.repositories.length > 0 && !isJobActive()) {
        await startJob('status_refresh', operationRequest());
      }
    } catch (error) {
      showError(error?.message || error);
      render();
    } finally {
      state.loading = false;
      els.gitWorkspaceLoading?.classList.add('hidden');
      renderRepositories();
    }
  })().finally(() => { state.initialLoadPromise = null; });
  return state.initialLoadPromise;
}

function operationRequest(repositoryID = 0) {
  const selected = selectedWorkspaceID();
  const ids = repositoryID ? [repositoryID] : visibleRepositories().filter((repository) => !repository.missing).map((repository) => repository.id);
  return { workspace_id: selected, repository_ids: ids };
}

async function startJob(kind, request, confirmation = null) {
  if (isJobActive()) return;
  if (confirmation) {
    const confirmed = await showConfirmation(confirmation);
    if (!confirmed) return;
  }
  const starters = { scan: startGitWorkspaceRescan, status_refresh: startGitStatusRefresh, fetch: startGitFetch, pull: startGitPull, sync: startGitSync };
  const starter = starters[kind];
  if (!starter) return;
  showError('');
  try {
    state.activeJob = normalizeJob(await starter(request));
    showStatus(`${JOB_LABELS[kind] || 'Git workspace job'} started.`);
    render();
  } catch (error) {
    showError(error?.message || error);
  }
}

async function chooseWorkspace() {
  showRootError('');
  try {
    const workspace = await chooseGitWorkspaceFolder();
    if (!workspace) return;
    const normalized = normalizeWorkspace(workspace);
    state.workspaces = [...state.workspaces.filter((item) => item.id !== normalized.id), normalized].sort((a, b) => a.display_name.localeCompare(b.display_name));
    state.initialized = true;
    render();
    await startJob('scan', { workspace_id: normalized.id, repository_ids: [] });
    await loadInventory();
    render();
  } catch (error) {
    showRootError(error?.message || error);
  }
}

async function removeWorkspace(workspaceID) {
  const workspace = state.workspaces.find((item) => item.id === workspaceID);
  if (!workspace) return;
  const confirmed = await showConfirmation({
    title: 'Remove workspace root?',
    confirmLabel: 'Remove root',
    message: `Remove “${workspace.display_name}” from Something’s tracking? This only changes Something’s tracking data. Repository files are never deleted.`,
  });
  if (!confirmed) return;
  try {
    await removeGitWorkspace({ id: workspace.id, expected_revision: workspace.revision });
    state.workspaces = state.workspaces.filter((item) => item.id !== workspace.id);
    await loadInventory();
    render();
  } catch (error) {
    showRootError(error?.message || error);
  }
}

async function pruneMissing() {
  const confirmed = await showConfirmation({
    title: 'Prune missing repositories?',
    confirmLabel: 'Prune tracking data',
    message: 'Pruning only changes Something’s tracking data for missing repositories. Repository files are never deleted.',
  });
  if (!confirmed) return;
  try {
    const count = numberValue(await pruneMissingGitRepositories());
    showStatus(`${count} missing ${count === 1 ? 'repository' : 'repositories'} pruned from tracking.`);
    await loadInventory();
    render();
  } catch (error) {
    showRootError(error?.message || error);
  }
}

async function openDetails(repositoryID) {
  const repository = state.repositories.find((item) => item.id === repositoryID);
  if (!repository || !els.gitWorkspaceDetailsModal) return;
  state.detailRepositoryID = repositoryID;
  state.detailPreviousFocus = document.activeElement;
  els.gitWorkspaceDetailsTitle.textContent = repository.name;
  els.gitWorkspaceDetailsBody.replaceChildren(node('p', 'git-workspace-muted', 'Loading repository details…'));
  els.gitWorkspaceDetailsModal.classList.remove('hidden');
  els.gitWorkspaceDetailsModal.setAttribute('aria-hidden', 'false');
  els.gitWorkspaceDetailsClose.focus();
  try {
    const [details, history] = await Promise.all([getGitRepositoryDetails(repositoryID), getGitRepositoryHistory(repositoryID, 20)]);
    renderDetails(value(details, 'repository') || repository, value(details, 'cached_status', 'cachedStatus'), Array.isArray(history) ? history : []);
  } catch (error) {
    els.gitWorkspaceDetailsBody.replaceChildren(node('p', 'error-box', error?.message || error));
  }
}

function renderDetails(repositoryRaw, statusRaw, history) {
  const repository = normalizeRepository(repositoryRaw);
  const status = normalizeStatus(statusRaw);
  const body = els.gitWorkspaceDetailsBody;
  body.replaceChildren();
  const grid = node('div', 'git-workspace-details-grid');
  const items = [
    ['Workspace', workspaceNames(state.repositories.find((item) => item.id === repository.id) || repository)],
    ['Repository', repository.name],
    ['Branch', status?.branch || 'Not checked'],
    ['Working tree', repository.missing ? 'Missing' : status ? (status.dirty ? 'Dirty' : 'Clean') : 'Not checked'],
    ['Sync', status?.sync_state || 'Not checked'],
    ['Upstream', status?.upstream || 'None'],
    ['Remote', status?.remote_display || 'None'],
    ['Last checked', formatDate(status?.checked_at)],
  ];
  items.forEach(([label, content]) => {
    const item = node('div', 'git-workspace-detail-item');
    item.append(node('span', '', label), node('strong', '', content));
    grid.append(item);
  });
  body.append(grid);
  const historySection = node('section', 'git-workspace-history');
  historySection.append(node('h3', '', 'Recent commits'));
  if (history.length === 0) {
    historySection.append(node('p', 'git-workspace-muted', 'No recent commits are available.'));
  } else {
    history.forEach((commit) => {
      const row = node('div', 'git-workspace-commit');
      row.append(node('strong', '', clean(value(commit, 'message')) || 'Untitled commit'));
      row.append(node('span', '', `${clean(value(commit, 'hash')).slice(0, 10) || '—'} · ${clean(value(commit, 'author')) || 'Unknown author'} · ${formatDate(value(commit, 'timestamp'))}`));
      historySection.append(row);
    });
  }
  body.append(historySection);
}

function closeDetails() {
  if (!els.gitWorkspaceDetailsModal || els.gitWorkspaceDetailsModal.classList.contains('hidden')) return;
  els.gitWorkspaceDetailsModal.classList.add('hidden');
  els.gitWorkspaceDetailsModal.setAttribute('aria-hidden', 'true');
  state.detailPreviousFocus?.focus?.();
  state.detailPreviousFocus = null;
}

async function handleRepositoryAction(action, repositoryID) {
  const repository = state.repositories.find((item) => item.id === repositoryID);
  if (!repository) return;
  if (action === 'details') return openDetails(repositoryID);
  if (repository.missing) return;
  if (action === 'folder' || action === 'editor' || action === 'remote') {
    try {
      const actions = { folder: openGitRepositoryFolder, editor: openGitRepositoryInEditor, remote: openGitRepositoryRemote };
      await actions[action](repositoryID);
    } catch (error) {
      showError(error?.message || error);
    }
    return;
  }
  const confirmations = {
    pull: {
      title: 'Pull repositories?', confirmLabel: 'Pull fast-forward only',
      message: 'Pull uses fast-forward only. Dirty, diverged, and no-upstream repositories are skipped. Repository files are never deleted.',
    },
    sync: {
      title: 'Sync repositories?', confirmLabel: 'Sync fast-forward only',
      message: 'Sync uses fast-forward only. Dirty, diverged, and no-upstream repositories are skipped. Repository files are never deleted.',
    },
  };
  await startJob(action, operationRequest(repositoryID), confirmations[action] || null);
}

async function saveSettings() {
  if (!state.settings) return;
  const workerCount = numberValue(els.gitWorkspaceWorkers.value, 0);
  const staleDays = numberValue(els.gitWorkspaceStaleDays.value, 0);
  try {
    const response = await updateGitWorkspaceSettings({
        editor_application_id: els.gitWorkspaceEditor.value || null,
        worker_count: workerCount,
        stale_days: staleDays,
        expected_revision: state.settings.revision,
      });
    state.settings = { ...state.settings, ...(value(response, 'settings') || response) };
    els.gitWorkspaceSettingsStatus.textContent = 'Git workspace settings saved.';
  } catch (error) {
    els.gitWorkspaceSettingsStatus.textContent = clean(error?.message || error);
  }
}

function bindEvents() {
  if (state.eventsBound) return;
  state.eventsBound = true;
  if (hasWailsBinding() && window.runtime?.EventsOn) {
    window.runtime.EventsOn(EVENTS.inventory, () => {
      void (async () => {
        try {
          const roots = await listGitWorkspaces();
          state.workspaces = (Array.isArray(roots) ? roots : []).map(normalizeWorkspace);
        } catch { /* keep the last known roots while the inventory refreshes */ }
        await loadInventory();
        render();
      })();
    });
    window.runtime.EventsOn(EVENTS.status, (payload) => {
      const repositoryID = numberValue(value(payload, 'repository_id', 'repositoryId'), 0);
      if (!repositoryID) return;
      void getGitRepositoryDetails(repositoryID).then((details) => {
        const repository = state.repositories.find((item) => item.id === repositoryID);
        const cached = value(details, 'cached_status', 'cachedStatus');
        if (repository && cached) repository.cached_status = normalizeStatus(cached);
        render();
      }).catch(() => {});
    });
    [EVENTS.progress, EVENTS.complete].forEach((eventName) => {
      window.runtime.EventsOn(eventName, (payload) => {
        const job = normalizeJob(payload);
        if (!job || !job.id) return;
        state.activeJob = job;
        render();
        if (eventName === EVENTS.complete) {
          void loadInventory().then(() => render());
        }
      });
    });
  }
  els.gitWorkspaceAdd?.addEventListener('click', () => void chooseWorkspace());
  els.gitWorkspaceRescan?.addEventListener('click', () => {
    const workspaceID = selectedWorkspaceID() || (state.workspaces.length === 1 ? state.workspaces[0].id : 0);
    if (!workspaceID) {
      showStatus(state.workspaces.length === 0 ? 'Add a workspace before rescanning.' : 'Choose a workspace before rescanning.');
      return;
    }
    void startJob('scan', { workspace_id: workspaceID, repository_ids: [] });
  });
  els.gitWorkspaceRefreshStatus?.addEventListener('click', () => void startJob('status_refresh', operationRequest()));
  els.gitWorkspaceFetch?.addEventListener('click', () => void startJob('fetch', operationRequest()));
  els.gitWorkspaceSync?.addEventListener('click', () => void startJob('sync', operationRequest(), {
    title: 'Sync repositories?', confirmLabel: 'Sync fast-forward only',
    message: 'Sync uses fast-forward only. Dirty, diverged, and no-upstream repositories are skipped. Repository files are never deleted.',
  }));
  els.gitWorkspacePrune?.addEventListener('click', () => void pruneMissing());
  [els.gitWorkspaceWorkspaceFilter, els.gitWorkspaceSearch, els.gitWorkspaceBranchFilter, els.gitWorkspaceStatusFilter, els.gitWorkspaceSyncFilter].forEach((control) => {
    control?.addEventListener('input', () => renderRepositories());
    control?.addEventListener('change', () => renderRepositories());
  });
  els.gitWorkspaceList?.addEventListener('click', (event) => {
    const trigger = event.target.closest('[data-git-action]');
    if (!trigger || trigger.disabled) return;
    void handleRepositoryAction(trigger.dataset.gitAction, numberValue(trigger.dataset.repositoryId));
  });
  els.gitWorkspaceRoots?.addEventListener('click', (event) => {
    const trigger = event.target.closest('[data-git-action]');
    if (!trigger) return;
    const workspaceID = numberValue(trigger.dataset.repositoryId);
    if (trigger.dataset.gitAction === 'rescan-root') void startJob('scan', { workspace_id: workspaceID, repository_ids: [] });
    if (trigger.dataset.gitAction === 'remove-root') void removeWorkspace(workspaceID);
  });
  els.gitWorkspaceCancel?.addEventListener('click', async () => {
    if (!isJobActive()) return;
    try { state.activeJob = normalizeJob(await cancelGitWorkspaceJob(state.activeJob.id)); render(); } catch (error) { showError(error?.message || error); }
  });
  els.gitWorkspaceSettingsSave?.addEventListener('click', () => void saveSettings());
  els.gitWorkspaceDetailsClose?.addEventListener('click', closeDetails);
  els.gitWorkspaceDetailsBackdrop?.addEventListener('click', closeDetails);
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !els.gitWorkspaceDetailsModal?.classList.contains('hidden')) {
      event.preventDefault();
      closeDetails();
    }
  });
}

export function initGitWorkspaces() {
  bindEvents();
  renderCapability();
  renderJob();
}
