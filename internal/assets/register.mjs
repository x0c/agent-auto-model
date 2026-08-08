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

function currentMode() {
  const m = globalThis.__cursorModeModelLastMode;
  return m == null || m === '' ? 'default' : m;
}

function managerModelId(mgr) {
  if (!mgr) return null;
  const selected = mgr.currentSelectedModel && mgr.currentSelectedModel.modelId;
  if (selected) return selected;
  try {
    if (typeof mgr.getCurrentModel === 'function') {
      const cur = mgr.getCurrentModel();
      if (cur && cur.modelId) return cur.modelId;
    }
  } catch {
    /* ignore */
  }
  return (mgr.currentModel && mgr.currentModel.modelId) || null;
}

function bumpGen() {
  const n = (globalThis.__cursorModeModelGen || 0) + 1;
  globalThis.__cursorModeModelGen = n;
  return n;
}

function stashStore(store) {
  if (store && typeof store === 'object') {
    globalThis.__cursorModeModelStore = store;
  }
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
  scheduleRestoreGuard('stash');
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
      void applyModel(resolveModel(pending), { reason: 'flush', gen: bumpGen() });
      return;
    }
    if (n >= 200) {
      clearInterval(globalThis.__cursorModeModelFlushTimer);
      globalThis.__cursorModeModelFlushTimer = undefined;
    }
  }, 50);
}

function writeLastUsedModel(modelId) {
  const store = globalThis.__cursorModeModelStore;
  if (!store || !modelId) return;
  try {
    if (typeof store.setMetadata === 'function') {
      store.setMetadata('lastUsedModel', modelId);
      debugLog({ ev: 'lastUsedModel_write', modelId, via: 'setMetadata' });
      return;
    }
    if (store.metadataStore && typeof store.metadataStore.set === 'function') {
      store.metadataStore.set('lastUsedModel', modelId);
      debugLog({ ev: 'lastUsedModel_write', modelId, via: 'metadataStore' });
    }
  } catch (err) {
    debugLog({
      ev: 'lastUsedModel_error',
      message: err && err.message ? err.message : String(err),
    });
  }
}

function forceSelectedModel(mgr, modelId) {
  if (!mgr || !modelId) return;
  let parameters = [];
  const cfg = globalThis.__cursorModeModelConfig;
  try {
    if (typeof mgr.getParametersForModel === 'function' && cfg) {
      parameters = mgr.getParametersForModel(modelId, cfg) || [];
    } else if (
      mgr.currentSelectedModel &&
      mgr.currentSelectedModel.modelId === modelId &&
      Array.isArray(mgr.currentSelectedModel.parameters)
    ) {
      parameters = mgr.currentSelectedModel.parameters;
    }
  } catch {
    parameters = [];
  }
  mgr.currentSelectedModel = { modelId, parameters };
  const prev = mgr.currentModel && typeof mgr.currentModel === 'object' ? mgr.currentModel : {};
  mgr.currentModel = Object.assign({}, prev, {
    modelId,
    displayModelId: modelId,
    displayName: prev.displayName || modelId,
    displayNameShort: prev.displayNameShort || modelId,
  });
}

function beforeBuildRequested(mgr) {
  if (disabled || locked) return;
  if (mgr) stashManager(mgr);
  const mode = currentMode();
  const forced = resolveModel(mode);
  const current = managerModelId(mgr);
  debugLog({
    ev: 'before_build',
    mode,
    forcedModel: forced,
    managerModel: current,
  });
  if (!forced || !mgr) return;
  if (current !== forced) {
    forceSelectedModel(mgr, forced);
    debugLog({
      ev: 'before_build_forced',
      mode,
      forcedModel: forced,
      previousModel: current,
    });
    void applyModel(forced, { reason: 'before_build', gen: bumpGen() });
  }
}

const restoreGuardTimers = [];

function scheduleRestoreGuard(reason) {
  if (disabled || locked) return;
  if (globalThis.__cursorModeModelRestoreArmed) return;
  globalThis.__cursorModeModelRestoreArmed = true;
  const delays = [0, 300, 1000, 2500];
  for (const delay of delays) {
    const timer = setTimeout(() => {
      if (disabled || locked) return;
      const mode = currentMode();
      const target = resolveModel(mode);
      const mgr = globalThis.__cursorModeModelManager;
      const current = managerModelId(mgr);
      debugLog({
        ev: 'restore_guard',
        reason,
        delay,
        mode,
        target,
        managerModel: current,
      });
      if (!target || !mgr) return;
      if (current !== target) {
        void applyModel(target, { reason: 'restore_guard', delay, gen: bumpGen() });
      }
    }, delay);
    restoreGuardTimers.push(timer);
  }
}

function noteStoredRestore(storedId) {
  debugLog({
    ev: 'stored_restore_call',
    storedId,
    mode: currentMode(),
    want: resolveModel(currentMode()),
    selfApply: !!globalThis.__cursorModeModelApplying,
  });
  // 忽略我们自己发起的切换，只盯会话恢复等外部写入
  if (globalThis.__cursorModeModelApplying) return;
  // 允许再次武装：清掉旧标记后重挂延迟对齐
  globalThis.__cursorModeModelRestoreArmed = false;
  scheduleRestoreGuard('setModelFromStoredId');
}

async function applyModel(modelId, opts) {
  const options = opts || {};
  const gen = options.gen != null ? options.gen : bumpGen();
  const mgr = globalThis.__cursorModeModelManager;
  const cfg =
    globalThis.__cursorModeModelConfig ||
    (mgr && mgr.configProvider && typeof mgr.configProvider.get === 'function'
      ? mgr.configProvider
      : null);
  debugLog({
    ev: 'apply',
    modelId,
    reason: options.reason || 'unspecified',
    gen,
    hasMgr: !!mgr,
    hasCfg: !!cfg,
  });
  if (!mgr || !modelId) return false;
  globalThis.__cursorModeModelApplying = true;
  try {
    let applied = false;
    let via = '';
    if (typeof mgr.setModelFromStoredId === 'function' && cfg) {
      const ok = await mgr.setModelFromStoredId(modelId, cfg);
      via = 'setModelFromStoredId';
      debugLog({ ev: 'apply_result', via, ok, gen, modelId });
      if (ok) {
        applied = true;
      } else {
        debugLog({ ev: 'apply_rejected', via, modelId, gen });
      }
    }
    if (!applied && typeof mgr.setCurrentModelWithParameters === 'function' && cfg) {
      await mgr.setCurrentModelWithParameters(modelId, [], cfg);
      via = 'setCurrentModelWithParameters';
      applied = true;
      debugLog({ ev: 'apply_result', via, ok: true, gen, modelId });
    }
    if (!applied && typeof mgr.setCurrentModel === 'function' && cfg) {
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
      via = 'setCurrentModel';
      applied = true;
      debugLog({ ev: 'apply_result', via, ok: true, gen, modelId });
    }
    if (gen !== globalThis.__cursorModeModelGen) {
      debugLog({ ev: 'apply_stale', gen, currentGen: globalThis.__cursorModeModelGen, modelId });
      return false;
    }
    if (!applied) {
      console.error('[cursor-mode-model] 切换模型失败: 无可用 API');
      return false;
    }
    const verified = managerModelId(mgr);
    debugLog({
      ev: 'apply_verify',
      modelId,
      verified,
      match: verified === modelId,
      via,
      gen,
    });
    if (verified !== modelId) {
      // 发送路径仍可靠 before_build 强制；这里尽量把内存态掰正
      forceSelectedModel(mgr, modelId);
      console.error(
        '[cursor-mode-model] 切换后校验不一致，已强制内存模型:',
        verified,
        '->',
        modelId,
      );
    }
    writeLastUsedModel(modelId);
    return true;
  } catch (err) {
    debugLog({
      ev: 'apply_error',
      message: err && err.message ? err.message : String(err),
      gen,
      modelId,
    });
    console.error(
      '[cursor-mode-model] 切换模型失败:',
      err && err.message ? err.message : err,
    );
    return false;
  } finally {
    globalThis.__cursorModeModelApplying = false;
  }
}

function syncMode(key, value) {
  if (disabled || locked) return;
  if (key === 'lastUsedModel') return;
  if (key !== 'mode') return;
  globalThis.__cursorModeModelLastMode = value;
  const modelId = resolveModel(value);
  const gen = bumpGen();
  debugLog({
    ev: 'mode',
    value,
    modelId,
    gen,
    hasMgr: !!globalThis.__cursorModeModelManager,
  });
  if (!modelId) return;
  if (!globalThis.__cursorModeModelManager) {
    globalThis.__cursorModeModelPending = value;
    scheduleFlush();
    return;
  }
  void applyModel(modelId, { reason: 'mode', gen });
}

globalThis.__cursorModeModelSync = syncMode;
globalThis.__cursorModeModelStash = stashManager;
globalThis.__cursorModeModelStashStore = stashStore;
globalThis.__cursorModeModelBeforeBuild = beforeBuildRequested;
globalThis.__cursorModeModelNoteStored = noteStoredRestore;

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
  'setModelFromStoredId(e,t){return ' +
  stashThisCfg +
  ',globalThis.__cursorModeModelNoteStored&&globalThis.__cursorModeModelNoteStored(e),p(this,void 0,void 0,(function*(){';
const PATCH_GET_CURRENT =
  'getCurrentModel(){return ' + stashThis + ',this.deriveCurrentModelDetails()';
const syncCall =
  'typeof globalThis.__cursorModeModelSync==="function"&&' +
  'globalThis.__cursorModeModelSync(e,t)';
const stashStoreCall =
  'globalThis.__cursorModeModelStashStore&&globalThis.__cursorModeModelStashStore(this);';
const PATCH_SET_METADATA =
  'setMetadata(e,t){' + stashStoreCall + 'this.metadataStore.set(e,t);' + syncCall + '}';
const PATCH_BUILD_REQUESTED =
  'buildRequestedModel(){typeof globalThis.__cursorModeModelBeforeBuild==="function"&&globalThis.__cursorModeModelBeforeBuild(this);var e,t,r,n;const o=this.currentSelectedModel;';

const SRC_SET_CURRENT = 'setCurrentModel(e,t){return p(this,void 0,void 0,(function*(){';
const SRC_SET_PARAMS =
  'setCurrentModelWithParameters(e,t,r){return p(this,void 0,void 0,(function*(){';
const SRC_SET_FROM_STORED =
  'setModelFromStoredId(e,t){return p(this,void 0,void 0,(function*(){';
const SRC_GET_CURRENT = 'getCurrentModel(){return this.deriveCurrentModelDetails()';
const SRC_SET_METADATA = 'setMetadata(e,t){this.metadataStore.set(e,t)}';
const SRC_BUILD_REQUESTED =
  'buildRequestedModel(){var e,t,r,n;const o=this.currentSelectedModel;';

function patchSource(source) {
  if (typeof source !== 'string') return { source, hits: 0 };
  if (
    source.includes('__cursorModeModelSync') ||
    source.includes('__cursorModeModelStash') ||
    source.includes('__cursorModeModelBeforeBuild')
  ) {
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
    [SRC_BUILD_REQUESTED, PATCH_BUILD_REQUESTED],
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
  // 只改 Cursor Agent 安装树，避免误伤无关 .js
  if (!url.includes('cursor-agent') && !url.includes('/.local/share/cursor-agent/')) {
    return false;
  }
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
