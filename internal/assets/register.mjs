import { registerHooks } from 'node:module';
import { readFileSync, existsSync, appendFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const disabled = String(process.env.CURSOR_MODE_MODEL || '1').trim() === '0';
const locked = Boolean(process.env.CURSOR_MODE_MODEL_LOCK);
const debugLog =
  String(process.env.CURSOR_MODE_MODEL_DEBUG || '').trim() === '1'
    ? (obj) => {
        try {
          appendFileSync(
            join(here, 'sync.log'),
            JSON.stringify({ t: Date.now(), ...obj }) + '\n',
          );
        } catch {
          /* ignore */
        }
      }
    : () => {};

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

function stashManager(mgr, configProvider) {
  if (!mgr || typeof mgr !== 'object') return;
  globalThis.__cursorModeModelManager = mgr;
  const cfg =
    (configProvider && typeof configProvider.get === 'function' && configProvider) ||
    (mgr.configProvider && typeof mgr.configProvider.get === 'function' && mgr.configProvider) ||
    null;
  if (cfg) {
    globalThis.__cursorModeModelConfig = cfg;
  }
  const pending = globalThis.__cursorModeModelPending;
  if (pending != null) {
    globalThis.__cursorModeModelPending = undefined;
    queueMicrotask(() => syncMode('mode', pending));
  }
}

function scheduleFlush() {
  if (globalThis.__cursorModeModelFlushTimer) return;
  let n = 0;
  globalThis.__cursorModeModelFlushTimer = setInterval(() => {
    n += 1;
    const pending = globalThis.__cursorModeModelPending;
    const mgr = globalThis.__cursorModeModelManager;
    if (pending != null && mgr) {
      clearInterval(globalThis.__cursorModeModelFlushTimer);
      globalThis.__cursorModeModelFlushTimer = undefined;
      globalThis.__cursorModeModelPending = undefined;
      void applyModel(resolveModel(pending));
      return;
    }
    if (n >= 200) {
      clearInterval(globalThis.__cursorModeModelFlushTimer);
      globalThis.__cursorModeModelFlushTimer = undefined;
    }
  }, 50);
}

async function applyModel(modelId) {
  const mgr = globalThis.__cursorModeModelManager;
  const cfg =
    globalThis.__cursorModeModelConfig ||
    (mgr && mgr.configProvider && typeof mgr.configProvider.get === 'function'
      ? mgr.configProvider
      : null);
  debugLog({ ev: 'apply', modelId, hasMgr: !!mgr, hasCfg: !!cfg });
  if (!mgr || !modelId) return;
  try {
    // 官方签名：setModelFromStoredId(modelId, configProvider)
    if (typeof mgr.setModelFromStoredId === 'function' && cfg) {
      const ok = await mgr.setModelFromStoredId(modelId, cfg);
      debugLog({ ev: 'apply_result', via: 'setModelFromStoredId', ok });
      return;
    }
    if (typeof mgr.setCurrentModelWithParameters === 'function' && cfg) {
      await mgr.setCurrentModelWithParameters(modelId, [], cfg);
      debugLog({ ev: 'apply_result', via: 'setCurrentModelWithParameters' });
      return;
    }
    if (typeof mgr.setCurrentModel === 'function' && cfg) {
      await mgr.setCurrentModel(
        {
          modelId,
          displayModelId: modelId,
          displayName: modelId,
          displayNameShort: modelId,
          aliases: [],
        },
        cfg,
      );
      debugLog({ ev: 'apply_result', via: 'setCurrentModel' });
    }
  } catch (err) {
    debugLog({ ev: 'apply_error', message: err && err.message ? err.message : String(err) });
    console.error(
      '[cursor-mode-model] 切换模型失败:',
      err && err.message ? err.message : err,
    );
  }
}

function syncMode(key, value) {
  if (disabled || locked) return;
  if (key !== 'mode') return;
  globalThis.__cursorModeModelLastMode = value;
  const modelId = resolveModel(value);
  debugLog({ ev: 'mode', value, modelId, hasMgr: !!globalThis.__cursorModeModelManager });
  if (!modelId) return;
  if (!globalThis.__cursorModeModelManager) {
    globalThis.__cursorModeModelPending = value;
    scheduleFlush();
    return;
  }
  void applyModel(modelId);
}

globalThis.__cursorModeModelSync = syncMode;
globalThis.__cursorModeModelStash = stashManager;

const stashThis = 'globalThis.__cursorModeModelStash&&globalThis.__cursorModeModelStash(this)';
const stashThisCfg =
  'globalThis.__cursorModeModelStash&&globalThis.__cursorModeModelStash(this,t)';
const stashThisCfgR =
  'globalThis.__cursorModeModelStash&&globalThis.__cursorModeModelStash(this,r)';

const PATCH_SET_CURRENT =
  'setCurrentModel(e,t){return ' + stashThisCfg + ',p(this,void 0,void 0,(function*(){';
const PATCH_SET_PARAMS =
  'setCurrentModelWithParameters(e,t,r){return ' +
  stashThisCfgR +
  ',p(this,void 0,void 0,(function*(){';
const PATCH_SET_FROM_STORED =
  'setModelFromStoredId(e,t){return ' + stashThisCfg + ',p(this,void 0,void 0,(function*(){';
const PATCH_GET_CURRENT =
  'getCurrentModel(){return ' + stashThis + ',this.deriveCurrentModelDetails()';
const syncCall =
  'typeof globalThis.__cursorModeModelSync==="function"&&' +
  'globalThis.__cursorModeModelSync(e,t)';
const PATCH_SET_METADATA =
  'setMetadata(e,t){this.metadataStore.set(e,t);' + syncCall + '}';

const SRC_SET_CURRENT = 'setCurrentModel(e,t){return p(this,void 0,void 0,(function*(){';
const SRC_SET_PARAMS =
  'setCurrentModelWithParameters(e,t,r){return p(this,void 0,void 0,(function*(){';
const SRC_SET_FROM_STORED =
  'setModelFromStoredId(e,t){return p(this,void 0,void 0,(function*(){';
const SRC_GET_CURRENT = 'getCurrentModel(){return this.deriveCurrentModelDetails()';
const SRC_SET_METADATA = 'setMetadata(e,t){this.metadataStore.set(e,t)}';

function patchSource(source) {
  if (typeof source !== 'string') return { source, hits: 0 };
  if (source.includes('__cursorModeModelSync') || source.includes('__cursorModeModelStash')) {
    return { source, hits: 0 };
  }
  let s = source;
  let hits = 0;
  const pairs = [
    [SRC_SET_CURRENT, PATCH_SET_CURRENT],
    [SRC_SET_PARAMS, PATCH_SET_PARAMS],
    [SRC_SET_FROM_STORED, PATCH_SET_FROM_STORED],
    [SRC_GET_CURRENT, PATCH_GET_CURRENT],
    [SRC_SET_METADATA, PATCH_SET_METADATA],
  ];
  for (const [src, patch] of pairs) {
    if (s.includes(src)) {
      s = s.split(src).join(patch);
      hits += 1;
    }
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
      debugLog({ ev: 'patch', url, hits });
      return {
        format: result.format || 'commonjs',
        source,
        shortCircuit: true,
      };
    },
  });
}
