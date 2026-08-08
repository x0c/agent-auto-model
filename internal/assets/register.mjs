import { registerHooks } from 'node:module';
import {
  readFileSync,
  existsSync,
  appendFileSync,
  renameSync,
  statSync,
} from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const unitTest = String(process.env.CURSOR_MODE_MODEL_UNIT_TEST || '').trim() === '1';
const disabled = String(process.env.CURSOR_MODE_MODEL || '1').trim() === '0';
const locked = Boolean(process.env.CURSOR_MODE_MODEL_LOCK);
const DECISIONS_LOG = join(here, 'decisions.log');
const DECISIONS_MAX_BYTES = 1024 * 1024;
const ALERT_COOLDOWN_MS = 8000;

function debugLog(obj) {
  if (String(process.env.CURSOR_MODE_MODEL_DEBUG || '').trim() !== '1') return;
  try {
    appendFileSync(join(here, 'sync.log'), JSON.stringify({ t: Date.now(), ...obj }) + '\n');
  } catch {
    /* ignore */
  }
}

function loadJSON(path) {
  try {
    if (path && existsSync(path)) return JSON.parse(readFileSync(path, 'utf8'));
  } catch {
    /* ignore */
  }
  return null;
}

function loadConfig() {
  const fromEnv = process.env.CURSOR_MODE_MODEL_CONFIG;
  const candidates = [];
  if (fromEnv) candidates.push(fromEnv);
  candidates.push(join(here, 'config.json'));
  for (const p of candidates) {
    const raw = loadJSON(p);
    if (raw) return raw;
  }
  return {
    enabled: true,
    strict: false,
    models: {
      plan: 'claude-opus-5-thinking-high',
      default: 'cursor-grok-4.5-high-fast',
      search: 'cursor-grok-4.5-high-fast',
      debug: 'cursor-grok-4.5-high-fast',
    },
  };
}

function loadAnchors() {
  const raw = loadJSON(join(here, 'anchors.json'));
  if (raw && typeof raw === 'object') return raw;
  // 内置兜底（与 anchors.json 保持一致）
  return {
    setCurrentModel: 'setCurrentModel(e,t){return p(this,void 0,void 0,(function*(){',
    setCurrentModelWithParameters:
      'setCurrentModelWithParameters(e,t,r){return p(this,void 0,void 0,(function*(){',
    setModelFromStoredId: 'setModelFromStoredId(e,t){return p(this,void 0,void 0,(function*(){',
    getCurrentModel: 'getCurrentModel(){return this.deriveCurrentModelDetails()',
    setMetadata: 'setMetadata(e,t){this.metadataStore.set(e,t)}',
    buildRequestedModel:
      'buildRequestedModel(){var e,t,r,n;const o=this.currentSelectedModel;',
  };
}

const config = loadConfig();
const models = (config && config.models) || {};
const strict = Boolean(config && config.strict);
const anchors = loadAnchors();

function resolveModel(mode) {
  const key = String(mode || 'default');
  return models[key] || models.default || null;
}

/** 读模式：会话权威值 > 事件缓存；都没有则返回 null（禁止瞎猜 default）。 */
function currentMode() {
  const store = globalThis.__cursorModeModelStore;
  if (store && typeof store.getMetadata === 'function') {
    try {
      const m = store.getMetadata('mode');
      if (m != null && m !== '') return String(m);
    } catch {
      /* ignore */
    }
  }
  const cached = globalThis.__cursorModeModelLastMode;
  if (cached != null && cached !== '') return String(cached);
  return null;
}

function modeKnown() {
  return currentMode() != null;
}

function normalizeModelId(mgr, id) {
  if (!id) return id;
  const raw = String(id);
  if (mgr) {
    try {
      const cfg = globalThis.__cursorModeModelConfig;
      if (typeof mgr.mapModelToParameterizedSelection === 'function' && cfg) {
        const mapped = mgr.mapModelToParameterizedSelection(raw, cfg);
        if (mapped && mapped.modelId) return String(mapped.modelId);
      }
    } catch {
      /* ignore */
    }
    try {
      const map = mgr.parameterizedModelMap;
      if (map && typeof map.has === 'function' && map.has(raw)) return raw;
      if (map && typeof map.values === 'function') {
        for (const entry of map.values()) {
          if (!entry) continue;
          if (entry.name === raw || entry.modelId === raw) {
            return String(entry.name || entry.modelId || raw);
          }
        }
      }
    } catch {
      /* ignore */
    }
  }
  // 常见别名：*-thinking-high → 参数化基名（无 mgr 时也要生效）
  const thinking = raw.match(/^(claude-opus-\d+(?:\.\d+)?)-thinking-high$/);
  if (thinking) return thinking[1];
  return raw;
}

function modelsEquivalent(mgr, a, b) {
  if (!a || !b) return a === b;
  if (a === b) return true;
  return normalizeModelId(mgr, a) === normalizeModelId(mgr, b);
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

function rotateDecisionsIfNeeded() {
  try {
    if (!existsSync(DECISIONS_LOG)) return;
    const st = statSync(DECISIONS_LOG);
    if (st.size < DECISIONS_MAX_BYTES) return;
    renameSync(DECISIONS_LOG, DECISIONS_LOG + '.1');
  } catch {
    /* ignore */
  }
}

function decisionLog(obj) {
  try {
    rotateDecisionsIfNeeded();
    appendFileSync(DECISIONS_LOG, JSON.stringify({ t: Date.now(), ...obj }) + '\n');
  } catch {
    /* ignore */
  }
  debugLog({ ev: 'decision', ...obj });
}

function alertOnce(message) {
  const now = Date.now();
  const last = globalThis.__cursorModeModelLastAlertAt || 0;
  if (now - last < ALERT_COOLDOWN_MS) return;
  globalThis.__cursorModeModelLastAlertAt = now;
  console.error('[cursor-mode-model]', message);
}

function ensureModeSubscription(store) {
  if (!store || typeof store.subscribeToMetadata !== 'function') return;
  if (globalThis.__cursorModeModelModeUnsub) return;
  try {
    const unsub = store.subscribeToMetadata('mode', () => {
      let value;
      try {
        value = store.getMetadata('mode');
      } catch {
        return;
      }
      if (value == null) return;
      syncMode('mode', value, { via: 'subscribe' });
    });
    globalThis.__cursorModeModelModeUnsub = unsub;
    // 只缓存权威值，不在此处 apply：启动时 mode 常先为 default，
    // 随后 CLI 才 setMetadata(plan)，过早 apply 会把 Plan 短暂压成 Grok。
    try {
      const m = store.getMetadata('mode');
      if (m != null && m !== '') {
        globalThis.__cursorModeModelLastMode = String(m);
        debugLog({ ev: 'mode_cached_from_store', value: String(m) });
      }
    } catch {
      /* ignore */
    }
    debugLog({ ev: 'mode_subscribed' });
  } catch (err) {
    debugLog({
      ev: 'mode_subscribe_error',
      message: err && err.message ? err.message : String(err),
    });
  }
}

function stashStore(store) {
  if (!store || typeof store !== 'object') return;
  globalThis.__cursorModeModelStore = store;
  ensureModeSubscription(store);
  try {
    if (typeof store.getMetadata === 'function') {
      const m = store.getMetadata('mode');
      if (m != null && m !== '') {
        globalThis.__cursorModeModelLastMode = String(m);
      }
    }
  } catch {
    /* ignore */
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
    queueMicrotask(() => syncMode('mode', pending, { via: 'pending_flush' }));
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
  const mgr = globalThis.__cursorModeModelManager;
  const id = normalizeModelId(mgr, modelId) || modelId;
  try {
    if (typeof store.setMetadata === 'function') {
      store.setMetadata('lastUsedModel', id);
      debugLog({ ev: 'lastUsedModel_write', modelId: id, via: 'setMetadata' });
      return;
    }
    if (store.metadataStore && typeof store.metadataStore.set === 'function') {
      store.metadataStore.set('lastUsedModel', id);
      debugLog({ ev: 'lastUsedModel_write', modelId: id, via: 'metadataStore' });
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
  const cfg = globalThis.__cursorModeModelConfig;
  let targetId = modelId;
  let parameters = [];
  try {
    if (typeof mgr.mapModelToParameterizedSelection === 'function' && cfg) {
      const mapped = mgr.mapModelToParameterizedSelection(modelId, cfg);
      if (mapped && mapped.modelId) {
        targetId = mapped.modelId;
        if (Array.isArray(mapped.parameters)) parameters = mapped.parameters;
      }
    }
  } catch {
    /* ignore */
  }
  try {
    if (
      parameters.length === 0 &&
      typeof mgr.getParametersForModel === 'function' &&
      cfg
    ) {
      parameters = mgr.getParametersForModel(targetId, cfg) || [];
    } else if (
      parameters.length === 0 &&
      mgr.currentSelectedModel &&
      modelsEquivalent(mgr, mgr.currentSelectedModel.modelId, targetId) &&
      Array.isArray(mgr.currentSelectedModel.parameters)
    ) {
      parameters = mgr.currentSelectedModel.parameters;
    }
  } catch {
    parameters = parameters || [];
  }
  mgr.currentSelectedModel = { modelId: targetId, parameters };
  const prev = mgr.currentModel && typeof mgr.currentModel === 'object' ? mgr.currentModel : {};
  mgr.currentModel = Object.assign({}, prev, {
    modelId: targetId,
    displayModelId: targetId,
    displayName: prev.displayName || targetId,
    displayNameShort: prev.displayNameShort || targetId,
  });
  try {
    if (typeof mgr.notifyListeners === 'function') {
      mgr.notifyListeners();
    }
  } catch {
    /* ignore */
  }
  return targetId;
}

function beforeBuildRequested(mgr) {
  if (disabled || locked) return undefined;
  if (mgr) stashManager(mgr);
  // 尽量补抓 store：部分路径会把 agentStore 挂在 mgr 上
  if (!globalThis.__cursorModeModelStore && mgr) {
    const cand =
      mgr.agentStore ||
      mgr.store ||
      (mgr.sharedServices && mgr.sharedServices.agentStore) ||
      null;
    if (cand) stashStore(cand);
  }
  const mode = currentMode();
  if (mode == null) {
    decisionLog({
      ev: 'skip_force',
      reason: 'mode_unknown',
      managerModel: managerModelId(mgr),
    });
    debugLog({ ev: 'before_build_skip', reason: 'mode_unknown' });
    return undefined;
  }
  const forced = resolveModel(mode);
  const current = managerModelId(mgr);
  debugLog({
    ev: 'before_build',
    mode,
    forcedModel: forced,
    managerModel: current,
    normalizedForced: normalizeModelId(mgr, forced),
    normalizedCurrent: normalizeModelId(mgr, current),
  });
  if (!forced || !mgr) return undefined;
  if (modelsEquivalent(mgr, current, forced)) {
    decisionLog({
      ev: 'ok',
      mode,
      expected: forced,
      actual: current,
      corrected: false,
    });
    return undefined;
  }
  const appliedId = forceSelectedModel(mgr, forced);
  decisionLog({
    ev: 'corrected',
    mode,
    expected: forced,
    actualBefore: current,
    actualAfter: managerModelId(mgr),
    appliedId,
  });
  debugLog({
    ev: 'before_build_forced',
    mode,
    forcedModel: forced,
    previousModel: current,
    appliedId,
  });
  void applyModel(forced, { reason: 'before_build', gen: bumpGen() });
  // strict：若纠正后仍不等价，抛错阻断发送
  if (strict && !modelsEquivalent(mgr, managerModelId(mgr), forced)) {
    const msg =
      '严格模式：无法将请求模型纠正为 Mode 对应值（mode=' +
      mode +
      ', want=' +
      forced +
      ', got=' +
      managerModelId(mgr) +
      '）';
    alertOnce(msg);
    decisionLog({ ev: 'strict_block', mode, expected: forced, actual: managerModelId(mgr) });
    throw new Error('[cursor-mode-model] ' + msg);
  }
  if (!modelsEquivalent(mgr, managerModelId(mgr), forced)) {
    alertOnce(
      '发送前纠正失败：mode=' +
        mode +
        ' 期望 ' +
        forced +
        '，当前 ' +
        managerModelId(mgr),
    );
  }
  return undefined;
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
      if (mode == null) {
        debugLog({ ev: 'restore_guard_skip', reason, delay, why: 'mode_unknown' });
        return;
      }
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
      if (!modelsEquivalent(mgr, current, target)) {
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
  if (globalThis.__cursorModeModelApplying) return;
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
      let params = [];
      try {
        if (typeof mgr.getParametersForModel === 'function') {
          params = mgr.getParametersForModel(normalizeModelId(mgr, modelId), cfg) || [];
        }
      } catch {
        params = [];
      }
      await mgr.setCurrentModelWithParameters(
        normalizeModelId(mgr, modelId),
        params,
        cfg,
      );
      via = 'setCurrentModelWithParameters';
      applied = true;
      debugLog({ ev: 'apply_result', via, ok: true, gen, modelId });
    }
    if (!applied && typeof mgr.setCurrentModel === 'function' && cfg) {
      const nid = normalizeModelId(mgr, modelId);
      await mgr.setCurrentModel(
        {
          modelId: nid,
          displayModelId: nid,
          displayName: nid,
          displayNameShort: nid,
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
      alertOnce('切换模型失败: 无可用 API');
      decisionLog({ ev: 'apply_failed', modelId, reason: 'no_api' });
      return false;
    }
    const verified = managerModelId(mgr);
    const match = modelsEquivalent(mgr, verified, modelId);
    debugLog({
      ev: 'apply_verify',
      modelId,
      verified,
      match,
      via,
      gen,
    });
    if (!match) {
      forceSelectedModel(mgr, modelId);
      alertOnce('切换后校验不一致，已强制内存模型: ' + verified + ' -> ' + modelId);
    } else {
      try {
        if (typeof mgr.notifyListeners === 'function') mgr.notifyListeners();
      } catch {
        /* ignore */
      }
    }
    writeLastUsedModel(verified || modelId);
    decisionLog({
      ev: 'apply_done',
      reason: options.reason || 'unspecified',
      expected: modelId,
      actual: managerModelId(mgr),
      match: modelsEquivalent(mgr, managerModelId(mgr), modelId),
      via,
    });
    return true;
  } catch (err) {
    debugLog({
      ev: 'apply_error',
      message: err && err.message ? err.message : String(err),
      gen,
      modelId,
    });
    alertOnce('切换模型失败: ' + (err && err.message ? err.message : err));
    decisionLog({
      ev: 'apply_error',
      modelId,
      message: err && err.message ? err.message : String(err),
    });
    return false;
  } finally {
    globalThis.__cursorModeModelApplying = false;
  }
}

function syncMode(key, value, opts) {
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
    via: (opts && opts.via) || 'setMetadata',
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
    [anchors.setCurrentModel, PATCH_SET_CURRENT],
    [anchors.setCurrentModelWithParameters, PATCH_SET_PARAMS],
    [anchors.setModelFromStoredId, PATCH_SET_FROM_STORED],
    [anchors.getCurrentModel, PATCH_GET_CURRENT],
    [anchors.setMetadata, PATCH_SET_METADATA],
    [anchors.buildRequestedModel, PATCH_BUILD_REQUESTED],
  ];
  for (const [src, patch] of pairs) {
    if (src && s.includes(src)) {
      s = s.split(src).join(patch);
      hits += 1;
    }
  }
  return { source: s, hits };
}

function shouldPatchUrl(url) {
  if (!url || url.startsWith('node:')) return false;
  if (url.includes('cursor-mode-model') && url.includes('register.mjs')) return false;
  if (!url.includes('cursor-agent') && !url.includes('/.local/share/cursor-agent/')) {
    return false;
  }
  return url.includes('.js');
}

globalThis.__cursorModeModelSync = syncMode;
globalThis.__cursorModeModelStash = stashManager;
globalThis.__cursorModeModelStashStore = stashStore;
globalThis.__cursorModeModelBeforeBuild = beforeBuildRequested;
globalThis.__cursorModeModelNoteStored = noteStoredRestore;

export {
  currentMode,
  modeKnown,
  resolveModel,
  normalizeModelId,
  modelsEquivalent,
  patchSource,
  shouldPatchUrl,
  forceSelectedModel,
  loadAnchors,
  anchors,
};

if (!unitTest && !disabled && config.enabled !== false) {
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
