import { getMCPServerStatus, setMCPServerEnabled } from './api.js';

const statusElement = document.getElementById('mcp-status');
const endpointElement = document.getElementById('mcp-endpoint');
const toggleButton = document.getElementById('mcp-toggle');
const errorElement = document.getElementById('mcp-error');

let currentStatus = null;
let changing = false;

function statusLabel(state) {
  return {
    on: 'On',
    off: 'Off',
    starting: 'Starting...',
    stopping: 'Stopping...',
    error: 'Error',
    unavailable: 'Unavailable',
  }[state] || 'Unknown';
}

function renderStatus(status) {
  if (!statusElement || !endpointElement || !toggleButton || !errorElement) return;

  currentStatus = status;

  const state = status?.state || 'unavailable';
  const running = Boolean(status?.running);
  const transitioning = state === 'starting' || state === 'stopping';
  const message = status?.last_error || '';

  statusElement.dataset.state = state;
  statusElement.textContent = statusLabel(state);
  endpointElement.textContent = status?.endpoint || '-';

  toggleButton.disabled = changing || transitioning || state === 'unavailable';
  toggleButton.textContent = running ? 'Turn off MCP server' : 'Turn on MCP server';
  toggleButton.setAttribute('aria-pressed', String(running));
  toggleButton.classList.toggle('is-stop', running);

  errorElement.textContent = message;
  errorElement.classList.toggle('hidden', !message);
}

export async function refreshMCPServerStatus() {
  try {
    renderStatus(await getMCPServerStatus());
  } catch (error) {
    renderStatus({
      running: false,
      state: 'unavailable',
      endpoint: '',
      last_error: error?.message || 'Could not read the MCP server status.',
    });
  }
}

async function toggleMCPServer() {
  if (changing || !currentStatus) return;

  const shouldEnable = !currentStatus.running;
  let actionError = '';
  changing = true;
  renderStatus(currentStatus);
  toggleButton.textContent = shouldEnable ? 'Turning on...' : 'Turning off...';

  try {
    renderStatus(await setMCPServerEnabled(shouldEnable));
  } catch (error) {
    await refreshMCPServerStatus();

    if (!currentStatus?.last_error) {
      actionError = error?.message || 'Could not change the MCP server state.';
    }
  } finally {
    changing = false;
    if (currentStatus) renderStatus(currentStatus);
    if (actionError) {
      errorElement.textContent = actionError;
      errorElement.classList.remove('hidden');
    }
  }
}

export function initMCPServer() {
  if (!toggleButton) return;

  toggleButton.addEventListener('click', toggleMCPServer);
  void refreshMCPServerStatus();
}
