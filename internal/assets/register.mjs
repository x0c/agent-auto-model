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
const unitTest = String(process.env.CURSOR_MODE_MODEL_UNIT_TEST || process.env.AGENT_AUTO_MODEL_UNIT_TEST || '').trim() === '1';
const disabled =
  String(process.env.AGENT_AUTO_MODEL || process.env.CURSOR_MODE_MODEL || '1').trim() === '0';
const locked = Boolean(process.env.AGENT_AUTO_MODEL_LOCK || process.env.CURSOR_MODE_MODEL_LOCK);
const DECISIONS_LOG = join(here, 'decisions.log');
const DECISIONS_MAX_BYTES = 1024 * 1024;
const ALERT_COOLDOWN_MS = 8000;

function debugLog(obj) {
  if (
    String(process.env.AGENT_AUTO_MODEL_DEBUG || process.env.CURSOR_MODE_MODEL_DEBUG || '').trim() !==
    '1'
  )
    return;
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
  const fromEnv =
    process.env.AGENT_AUTO_MODEL_CONFIG || process.env.CURSOR_MODE_MODEL_CONFIG;
  const candidates = [];
  if (fromEnv) candidates.push(fromEnv);
  candidates.push(join(here, 'config.json'));
  for (const p of candidates) {
    const raw = loadJSON(p);
    if (raw) return normalizeConfig(raw);
  }
  return {
    enabled: true,
    strict: false,
    models: {
      plan: 'claude-opus-5-thinking-high',
      default: 'cursor-grok-*-high',
      search: 'cursor-grok-*-high',
      debug: 'cursor-grok-*-high',
    },
  };
}

function normalizeConfig(raw) {
  if (!raw || typeof raw !== 'object') return raw;
  const cursor = raw.runtimes && raw.runtimes.cursor;
  if (cursor && cursor.models && typeof cursor.models === 'object') {
    return {
      ...raw,
      enabled: raw.enabled !== false && cursor.enabled !== false,
      models: { ...raw.models, ...cursor.models },
    };
  }
  return raw;
}

function loadAnchors() {
  const raw = loadJSON(join(here, 'anchors.json'));
  if (raw && typeof raw === 'object') return raw;
  // 内置兜底（与 anchors.json 保持一致）
  return {
    setCurrentModel: 'setCurrentModel(e,t){return p(this,void 0,void 0,(function*(){',
    setCurrentModelWithParameters:
      'setCurrentModelWithParameters(e,t,n){return p(this,void 0,void 0,(function*(){',
    setModelFromStoredId: 'setModelFromStoredId(e,t){return p(this,void 0,void 0,(function*(){',
    getCurrentModel: 'getCurrentModel(){return this.deriveCurrentModelDetails()',
    setMetadata: 'setMetadata(e,t){this.metadataStore.set(e,t)}',
    buildRequestedModel:
      'buildRequestedModel(){var e,t,n,r;const o=this.currentSelectedModel;',
  };
}

const config = loadConfig();
const models = (config && config.models) || {};
const strict = Boolean(config && config.strict);
const anchors = loadAnchors();

function isGlobSpec(spec) {
  return typeof spec === 'string' && (spec.includes('*') || spec.includes('?'));
}

function globToRegExp(spec) {
  let out = '^';
  for (const ch of String(spec)) {
    if (ch === '*') out += '.*';
    else if (ch === '?') out += '.';
    else if (/[.+^${}()|[\]\\]/.test(ch)) out += '\\' + ch;
    else out += ch;
  }
  out += '$';
  return new RegExp(out);
}

/** 从模型管理器收集可匹配的模型 id。 */
function listAvailableModelIds(mgr) {
  const ids = new Set();
  const add = (v) => {
    if (v == null || v === '') return;
    ids.add(String(v));
  };
  if (!mgr) return [];
  try {
    const map = mgr.parameterizedModelMap;
    if (map && typeof map.values === 'function') {
      for (const entry of map.values()) {
        if (!entry) continue;
        add(entry.name);
        add(entry.modelId);
      }
    }
    if (map && typeof map.keys === 'function') {
      for (const k of map.keys()) add(k);
    }
  } catch {
    /* ignore */
  }
  try {
    if (Array.isArray(mgr.availableModels)) {
      for (const m of mgr.availableModels) {
        if (typeof m === 'string') add(m);
        else if (m) add(m.modelId || m.name || m.id);
      }
    }
  } catch {
    /* ignore */
  }
  add(managerModelId(mgr));
  return [...ids];
}

function versionKey(id) {
  const nums = [];
  const re = /(\d+)(?:\.(\d+))?/g;
  let m;
  while ((m = re.exec(String(id))) !== null) {
    nums.push(Number(m[1]));
    nums.push(m[2] != null ? Number(m[2]) : 0);
  }
  return nums;
}

/** 版本更高者更大；同版本优先非 -fast。 */
function compareModelCandidates(a, b) {
  const va = versionKey(a);
  const vb = versionKey(b);
  const n = Math.max(va.length, vb.length);
  for (let i = 0; i < n; i++) {
    const x = va[i] || 0;
    const y = vb[i] || 0;
    if (x !== y) return x - y;
  }
  const af = String(a).endsWith('-fast') ? 1 : 0;
  const bf = String(b).endsWith('-fast') ? 1 : 0;
  if (af !== bf) return bf - af;
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

function pickLatestMatching(spec, candidates) {
  if (!isGlobSpec(spec)) return spec;
  const re = globToRegExp(spec);
  const matched = (candidates || []).filter((id) => re.test(String(id)));
  if (!matched.length) return null;
  matched.sort(compareModelCandidates);
  return matched[matched.length - 1];
}

/** 把配置里的通配符解析成当前可用最新模型 id。 */
function expandModelSpec(spec, mgr) {
  if (!spec) return null;
  const raw = String(spec);
  if (!isGlobSpec(raw)) return raw;
  const candidates = listAvailableModelIds(mgr);
  const picked = pickLatestMatching(raw, candidates);
  if (picked) {
    debugLog({
      ev: 'glob_expand',
      pattern: raw,
      picked,
      candidateCount: candidates.length,
    });
    return picked;
  }
  debugLog({
    ev: 'glob_expand_miss',
    pattern: raw,
    candidateCount: candidates.length,
  });
  return null;
}

function resolveModel(mode) {
  const key = String(mode || 'default');
  const spec = models[key] || models.default || null;
  return expandModelSpec(spec, globalThis.__cursorModeModelManager);
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
  // cursor-grok-4.6-high / -high-fast → grok-4.6（无 mgr 时的兜底）
  const grok = raw.match(/^cursor-(grok-\d+(?:\.\d+)?)(?:-high)?(?:-fast)?$/i);
  if (grok) return grok[1].toLowerCase();
  return raw;
}

/** 配置/别名是否要求 Fast。`cursor-grok-*-high` 不含 fast 段 → false。 */
function specWantsFast(spec) {
  const s = String(spec || '').toLowerCase();
  if (!s) return false;
  if (s.includes('high-fast')) return true;
  return /(^|[-_*])fast($|[-_*])/.test(s);
}

function copyParams(params) {
  return (Array.isArray(params) ? params : [])
    .filter((p) => p && p.id != null)
    .map((p) => ({ id: String(p.id), value: String(p.value) }));
}

function paramValue(params, id) {
  const hit = (params || []).find((p) => p && String(p.id) === id);
  return hit == null ? undefined : String(hit.value);
}

function upsertParam(params, id, value) {
  const out = copyParams(params);
  const idx = out.findIndex((p) => p.id === id);
  const entry = { id, value: String(value) };
  if (idx >= 0) out[idx] = entry;
  else out.push(entry);
  return out;
}

function isTruthyParam(v) {
  return v === 'true' || v === '1' || v === 'yes';
}

/**
 * 按通配符/别名意图覆盖参数。Grok 4.6 起 high 与 high-fast 同为 modelId=grok-4.6，
 * 只靠 fast 参数区分；官方 getParametersForModel 会沿用上次选择/已存配置里的 fast=true。
 */
function applySpecParameterIntent(spec, params) {
  let out = copyParams(params);
  const s = String(spec || '');
  if (/high/i.test(s) && !/thinking-high/i.test(s)) {
    // cursor-grok-*-high* → effort=high（若已有 effort 或规格含 high）
    if (out.some((p) => p.id === 'effort') || /grok/i.test(s)) {
      out = upsertParam(out, 'effort', 'high');
    }
  }
  if (/grok/i.test(s) || out.some((p) => p.id === 'fast') || /fast/i.test(s)) {
    out = upsertParam(out, 'fast', specWantsFast(s) ? 'true' : 'false');
  }
  return out;
}

function mapSelection(mgr, spec) {
  if (!mgr || !spec) return null;
  const cfg = globalThis.__cursorModeModelConfig;
  try {
    if (typeof mgr.mapModelToParameterizedSelection === 'function' && cfg) {
      const mapped = mgr.mapModelToParameterizedSelection(String(spec), cfg);
      if (mapped && mapped.modelId) {
        return {
          modelId: String(mapped.modelId),
          parameters: applySpecParameterIntent(spec, mapped.parameters),
        };
      }
    }
  } catch {
    /* ignore */
  }
  const modelId = normalizeModelId(mgr, spec);
  if (!modelId) return null;
  let parameters = [];
  try {
    if (typeof mgr.getParametersForModel === 'function' && cfg) {
      // 不要在「当前已是同 base id」时直接信任 getParametersForModel：
      // 它会原样返回 currentSelectedModel.parameters（常含 fast=true）。
      const curId =
        mgr.currentSelectedModel && mgr.currentSelectedModel.modelId
          ? String(mgr.currentSelectedModel.modelId)
          : null;
      if (curId !== modelId) {
        parameters = mgr.getParametersForModel(modelId, cfg) || [];
      } else if (typeof mgr.getSavedModelParameters === 'function') {
        parameters = mgr.getSavedModelParameters(modelId, cfg) || [];
      } else {
        const map = mgr.parameterizedModelMap;
        const entry = map && typeof map.get === 'function' ? map.get(modelId) : null;
        const variants = entry && Array.isArray(entry.variants) ? entry.variants : [];
        const wantsFast = specWantsFast(spec);
        const variant =
          variants.find((v) => {
            const vals = copyParams(v && v.parameterValues);
            return isTruthyParam(paramValue(vals, 'fast')) === wantsFast;
          }) ||
          variants.find((v) => v && v.isDefaultNonMaxConfig) ||
          variants[0];
        parameters = variant ? variant.parameterValues || [] : [];
      }
    }
  } catch {
    parameters = [];
  }
  return {
    modelId,
    parameters: applySpecParameterIntent(spec, parameters),
  };
}

function modelsEquivalent(mgr, a, b) {
  if (!a || !b) return a === b;
  if (a === b) return true;
  return normalizeModelId(mgr, a) === normalizeModelId(mgr, b);
}

/** 当前选择是否满足配置规格（含 fast 参数意图）。 */
function selectionMatchesSpec(mgr, spec) {
  if (!mgr || !spec) return false;
  const want = mapSelection(mgr, spec);
  const curId = managerModelId(mgr);
  if (!want || !want.modelId || !curId) return false;
  if (!modelsEquivalent(mgr, curId, want.modelId)) return false;
  const curParams =
    (mgr.currentSelectedModel && mgr.currentSelectedModel.parameters) || [];
  const wantFast = paramValue(want.parameters, 'fast');
  if (wantFast != null) {
    const curFast = paramValue(curParams, 'fast');
    if (isTruthyParam(wantFast) !== isTruthyParam(curFast)) return false;
  }
  const wantEffort = paramValue(want.parameters, 'effort');
  if (wantEffort != null) {
    const curEffort = paramValue(curParams, 'effort');
    if (curEffort != null && curEffort !== wantEffort) return false;
  }
  return true;
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
  const want = mapSelection(mgr, modelId) || {
    modelId: normalizeModelId(mgr, modelId) || modelId,
    parameters: applySpecParameterIntent(modelId, []),
  };
  const targetId = want.modelId;
  const parameters = want.parameters || [];
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
  const pattern = models[String(mode)] || models.default || null;
  const current = managerModelId(mgr);
  const curParams =
    (mgr && mgr.currentSelectedModel && mgr.currentSelectedModel.parameters) || [];
  debugLog({
    ev: 'before_build',
    mode,
    forcedModel: forced,
    pattern,
    managerModel: current,
    managerFast: paramValue(curParams, 'fast'),
    normalizedForced: normalizeModelId(mgr, forced),
    normalizedCurrent: normalizeModelId(mgr, current),
    matches: selectionMatchesSpec(mgr, forced),
  });
  if (!forced || !mgr) return undefined;
  if (selectionMatchesSpec(mgr, forced)) {
    decisionLog({
      ev: 'ok',
      mode,
      expected: forced,
      actual: current,
      fast: paramValue(curParams, 'fast'),
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
    fastAfter: paramValue(
      (mgr.currentSelectedModel && mgr.currentSelectedModel.parameters) || [],
      'fast',
    ),
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
  if (strict && !selectionMatchesSpec(mgr, forced)) {
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
  if (!selectionMatchesSpec(mgr, forced)) {
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
      if (!selectionMatchesSpec(mgr, target)) {
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
  const want = mapSelection(mgr, modelId);
  debugLog({
    ev: 'apply',
    modelId,
    want,
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
    // 优先带参设置：Grok high 与 high-fast 同 modelId，必须显式传 fast。
    if (want && typeof mgr.setCurrentModelWithParameters === 'function' && cfg) {
      await mgr.setCurrentModelWithParameters(want.modelId, want.parameters, cfg);
      via = 'setCurrentModelWithParameters';
      applied = true;
      debugLog({
        ev: 'apply_result',
        via,
        ok: true,
        gen,
        modelId,
        parameters: want.parameters,
      });
    }
    if (!applied && typeof mgr.setModelFromStoredId === 'function' && cfg) {
      const ok = await mgr.setModelFromStoredId(modelId, cfg);
      via = 'setModelFromStoredId';
      debugLog({ ev: 'apply_result', via, ok, gen, modelId });
      if (ok) {
        applied = true;
      } else {
        debugLog({ ev: 'apply_rejected', via, modelId, gen });
      }
    }
    if (!applied && typeof mgr.setCurrentModel === 'function' && cfg) {
      const nid = (want && want.modelId) || normalizeModelId(mgr, modelId);
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
    // 官方 API 可能仍沿用已存 fast=true；始终按规格意图再写一次内存选择。
    forceSelectedModel(mgr, modelId);
    const verified = managerModelId(mgr);
    const match = selectionMatchesSpec(mgr, modelId);
    debugLog({
      ev: 'apply_verify',
      modelId,
      verified,
      match,
      fast: paramValue(
        (mgr.currentSelectedModel && mgr.currentSelectedModel.parameters) || [],
        'fast',
      ),
      via,
      gen,
    });
    if (!match) {
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
      fast: paramValue(
        (mgr.currentSelectedModel && mgr.currentSelectedModel.parameters) || [],
        'fast',
      ),
      match: selectionMatchesSpec(mgr, modelId),
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
// Agent 2026.08.11+：第三参由 r 变为 n（与 anchors.json 同步）
const stashThisCfgN =
  'globalThis.__cursorModeModelStash&&globalThis.__cursorModeModelStash(this,n)';

const PATCH_SET_CURRENT =
  'setCurrentModel(e,t){return ' + stashThisCfg + ',p(this,void 0,void 0,(function*(){';
const PATCH_SET_PARAMS =
  'setCurrentModelWithParameters(e,t,n){return ' +
  stashThisCfgN +
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
  'buildRequestedModel(){typeof globalThis.__cursorModeModelBeforeBuild==="function"&&globalThis.__cursorModeModelBeforeBuild(this);var e,t,n,r;const o=this.currentSelectedModel;';

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
  if (url.includes('register.mjs') && (url.includes('agent-auto-model') || url.includes('cursor-mode-model'))) {
    return false;
  }
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
  expandModelSpec,
  pickLatestMatching,
  isGlobSpec,
  listAvailableModelIds,
  normalizeModelId,
  modelsEquivalent,
  specWantsFast,
  applySpecParameterIntent,
  selectionMatchesSpec,
  mapSelection,
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
