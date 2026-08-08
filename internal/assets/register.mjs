import { registerHooks } from 'node:module';
import { readFileSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const disabled = String(process.env.CURSOR_MODE_MODEL || '1').trim() === '0';
const locked = Boolean(process.env.CURSOR_MODE_MODEL_LOCK);

function loadConfig() {
  const fromEnv = process.env.CURSOR_MODE_MODEL_CONFIG;
  const candidates = [];
  if (fromEnv) candidates.push(fromEnv);
  candidates.push(join(here, 'config.json'));
  for (const p of candidates) {
    try {
      if (p && existsSync(p)) {
        return JSON.parse(readFileSync(p, 'utf8'));
      }
    } catch {
      /* 故障开放 */
    }
  }
  return {
    enabled: true,
    models: {
      plan: 'claude-opus-5-thinking-high',
      default: 'cursor-grok-4.5-high-fast',
      search: 'cursor-grok-4.5-high-fast',
      debug: 'cursor-grok-4.5-high-fast',
    },
  };
}

const config = loadConfig();
const models = (config && config.models) || {};

function resolveModel(mode) {
  const key = String(mode || 'default');
  return models[key] || models.default || null;
}

function stashManager(mgr) {
  if (!mgr || typeof mgr !== 'object') return;
  globalThis.__cursorModeModelManager = mgr;
  const pending = globalThis.__cursorModeModelPending;
  if (pending) {
    globalThis.__cursorModeModelPending = undefined;
    queueMicrotask(() => syncMode('mode', pending));
  }
}

async function applyModel(modelId) {
  const mgr = globalThis.__cursorModeModelManager;
  if (!mgr || !modelId) return;
  try {
    if (typeof mgr.setCurrentModelWithParameters === 'function') {
      await mgr.setCurrentModelWithParameters(modelId, []);
      return;
    }
    if (typeof mgr.setCurrentModel === 'function') {
      await mgr.setCurrentModel({
        modelId,
        displayModelId: modelId,
        displayName: modelId,
        displayNameShort: modelId,
        aliases: [],
      });
    }
  } catch (err) {
    console.error('[cursor-mode-model] 切换模型失败:', err && err.message ? err.message : err);
  }
}

function syncMode(key, value) {
  if (disabled || locked) return;
  if (key !== 'mode') return;
  globalThis.__cursorModeModelLastMode = value;
  const modelId = resolveModel(value);
  if (!modelId) return;
  if (!globalThis.__cursorModeModelManager) {
    globalThis.__cursorModeModelPending = value;
    return;
  }
  void applyModel(modelId);
}

globalThis.__cursorModeModelSync = syncMode;
globalThis.__cursorModeModelStash = stashManager;

const stashCall = 'globalThis.__cursorModeModelStash&&globalThis.__cursorModeModelStash(this),';
const PATCH_SET_CURRENT =
  'setCurrentModel(e,t){return ' + stashCall + 'p(this,void 0,void 0,(function*(){';
const PATCH_SET_PARAMS =
  'setCurrentModelWithParameters(e,t,r){return ' + stashCall + 'p(this,void 0,void 0,(function*(){';
const syncCall =
  'typeof globalThis.__cursorModeModelSync==="function"&&' +
  'globalThis.__cursorModeModelSync(e,t)';
const PATCH_SET_METADATA =
  'setMetadata(e,t){this.metadataStore.set(e,t);' + syncCall + '}';

const SRC_SET_CURRENT = 'setCurrentModel(e,t){return p(this,void 0,void 0,(function*(){';
const SRC_SET_PARAMS =
  'setCurrentModelWithParameters(e,t,r){return p(this,void 0,void 0,(function*(){';
const SRC_SET_METADATA = 'setMetadata(e,t){this.metadataStore.set(e,t)}';

function patchSource(source) {
  if (typeof source !== 'string') return { source, hits: 0 };
  if (source.includes('__cursorModeModelSync')) {
    return { source, hits: 0 };
  }
  let s = source;
  let hits = 0;
  if (s.includes(SRC_SET_CURRENT)) {
    s = s.split(SRC_SET_CURRENT).join(PATCH_SET_CURRENT);
    hits += 1;
  }
  if (s.includes(SRC_SET_PARAMS)) {
    s = s.split(SRC_SET_PARAMS).join(PATCH_SET_PARAMS);
    hits += 1;
  }
  if (s.includes(SRC_SET_METADATA)) {
    s = s.split(SRC_SET_METADATA).join(PATCH_SET_METADATA);
    hits += 1;
  }
  return { source: s, hits };
}

function shouldPatchUrl(url) {
  if (!url || url.startsWith('node:')) return false;
  if (url.includes('cursor-mode-model') && url.includes('register.mjs')) return false;
  return url.includes('.js');
}

if (!disabled && config.enabled !== false) {
  registerHooks({
    load(url, context, nextLoad) {
      const result = nextLoad(url, context);
      if (!shouldPatchUrl(url)) return result;
      let text;
      if (result.source != null) {
        text =
          typeof result.source === 'string'
            ? result.source
            : Buffer.from(result.source).toString('utf8');
      } else {
        try {
          text = readFileSync(fileURLToPath(url), 'utf8');
        } catch {
          return result;
        }
      }
      const { source, hits } = patchSource(text);
      if (!hits) return result;
      return {
        format: result.format || 'commonjs',
        source,
        shortCircuit: true,
      };
    },
  });
}
