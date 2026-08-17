// Frontend-only persistence for the Setups UI. Once the Go setup methods are
// implemented, this adapter automatically prefers them and IndexedDB becomes
// only a development fallback.
const DATABASE_NAME = 'something-setups-ui';
const DATABASE_VERSION = 1;
const STORE_NAME = 'setups';

function setupBackend() {
  return window.go?.backend?.App || null;
}

function hasSetupBackend() {
  const backend = setupBackend();
  return Boolean(
    backend &&
    typeof backend.ListSetups === 'function' &&
    typeof backend.CreateSetup === 'function' &&
    typeof backend.UpdateSetup === 'function' &&
    typeof backend.DeleteSetup === 'function'
  );
}

function openDatabase() {
  return new Promise((resolve, reject) => {
    if (!window.indexedDB) {
      reject(new Error('Setups require the desktop app or browser storage support.'));
      return;
    }

    const request = window.indexedDB.open(DATABASE_NAME, DATABASE_VERSION);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains(STORE_NAME)) {
        database.createObjectStore(STORE_NAME, { keyPath: 'id' });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error('Could not open setup storage.'));
  });
}

async function listLocalSetups() {
  const database = await openDatabase();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction(STORE_NAME, 'readonly');
    const request = transaction.objectStore(STORE_NAME).getAll();
    request.onsuccess = () => {
      const records = Array.isArray(request.result) ? request.result : [];
      records.sort((left, right) => String(left.created_at || '').localeCompare(String(right.created_at || '')));
      resolve(records);
    };
    request.onerror = () => reject(request.error || new Error('Could not load setups.'));
    transaction.oncomplete = () => database.close();
    transaction.onerror = () => database.close();
  });
}

async function putLocalSetup(setup) {
  const database = await openDatabase();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction(STORE_NAME, 'readwrite');
    transaction.objectStore(STORE_NAME).put(setup);
    transaction.oncomplete = () => {
      database.close();
      resolve(setup);
    };
    transaction.onerror = () => {
      const error = transaction.error || new Error('Could not save the setup.');
      database.close();
      reject(error);
    };
  });
}

async function deleteLocalSetup(id) {
  const database = await openDatabase();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction(STORE_NAME, 'readwrite');
    transaction.objectStore(STORE_NAME).delete(Number(id));
    transaction.oncomplete = () => {
      database.close();
      resolve();
    };
    transaction.onerror = () => {
      const error = transaction.error || new Error('Could not delete the setup.');
      database.close();
      reject(error);
    };
  });
}

function cleanAppIDs(value) {
  return [...new Set((Array.isArray(value) ? value : [])
    .map(Number)
    .filter((id) => Number.isInteger(id) && id > 0))];
}

export async function listSetupRecords() {
  if (hasSetupBackend()) {
    const result = await setupBackend().ListSetups();
    return Array.isArray(result) ? result : [];
  }
  return listLocalSetups();
}

export async function createSetupRecord(request) {
  if (hasSetupBackend()) {
    return setupBackend().CreateSetup(request);
  }

  const records = await listLocalSetups();
  const id = records.reduce((largest, setup) => Math.max(largest, Number(setup.id) || 0), 0) + 1;
  const timestamp = new Date().toISOString();
  return putLocalSetup({
    id,
    name: String(request.name || '').trim(),
    description: String(request.description || '').trim(),
    app_ids: cleanAppIDs(request.app_ids),
    icon_data_url: String(request.icon_data_url || ''),
    icon_url: '',
    created_at: timestamp,
    updated_at: timestamp,
  });
}

export async function updateSetupRecord(request) {
  if (hasSetupBackend()) {
    return setupBackend().UpdateSetup(request);
  }

  const records = await listLocalSetups();
  const existing = records.find((setup) => Number(setup.id) === Number(request.id));
  if (!existing) throw new Error('The setup no longer exists.');

  let iconDataURL = String(existing.icon_data_url || '');
  if (request.remove_icon) iconDataURL = '';
  if (request.icon_data_url) iconDataURL = String(request.icon_data_url);

  return putLocalSetup({
    ...existing,
    name: String(request.name || '').trim(),
    description: String(request.description || '').trim(),
    app_ids: cleanAppIDs(request.app_ids),
    icon_data_url: iconDataURL,
    icon_url: request.remove_icon || request.icon_data_url ? '' : String(existing.icon_url || ''),
    updated_at: new Date().toISOString(),
  });
}

export async function deleteSetupRecord(id) {
  if (hasSetupBackend()) {
    await setupBackend().DeleteSetup(Number(id));
    return;
  }
  await deleteLocalSetup(id);
}

export function backendStartSetup(id) {
  const backend = setupBackend();
  if (!backend || typeof backend.StartSetup !== 'function') return null;
  return backend.StartSetup(Number(id));
}
