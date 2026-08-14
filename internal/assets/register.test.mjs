import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

process.env.AGENT_AUTO_MODEL_UNIT_TEST = '1';
delete process.env.AGENT_AUTO_MODEL_CONFIG;
delete process.env.AGENT_AUTO_MODEL;

const {
  currentMode,
  modeKnown,
  resolveModel,
  expandModelSpec,
  pickLatestMatching,
  isGlobSpec,
  normalizeModelId,
  modelsEquivalent,
  specWantsFast,
  applySpecParameterIntent,
  selectionMatchesSpec,
  forceSelectedModel,
  patchSource,
  shouldPatchUrl,
  anchors,
} = await import('./register.mjs');

const here = dirname(fileURLToPath(import.meta.url));

test('anchors.json 与 loadAnchors 一致且含全部关键锚点', () => {
  const disk = JSON.parse(readFileSync(join(here, 'anchors.json'), 'utf8'));
  for (const key of [
    'setCurrentModel',
    'setCurrentModelWithParameters',
    'setModelFromStoredId',
    'getCurrentModel',
    'setMetadata',
    'buildRequestedModel',
  ]) {
    assert.equal(anchors[key], disk[key]);
    assert.ok(disk[key].length > 10);
  }
});

test('currentMode：权威值优先于事件缓存', () => {
  globalThis.__cursorModeModelLastMode = 'default';
  globalThis.__cursorModeModelStore = {
    getMetadata(key) {
      if (key === 'mode') return 'plan';
      return undefined;
    },
  };
  assert.equal(currentMode(), 'plan');
  assert.equal(modeKnown(), true);
});

test('currentMode：无权威值时用事件缓存', () => {
  globalThis.__cursorModeModelStore = undefined;
  globalThis.__cursorModeModelLastMode = 'search';
  assert.equal(currentMode(), 'search');
});

test('currentMode：都没有时返回 null（禁止瞎猜 default）', () => {
  globalThis.__cursorModeModelStore = undefined;
  globalThis.__cursorModeModelLastMode = undefined;
  assert.equal(currentMode(), null);
  assert.equal(modeKnown(), false);
});

test('resolveModel 按配置映射（无 mgr 时通配符无法展开）', () => {
  globalThis.__cursorModeModelManager = undefined;
  assert.equal(resolveModel('plan'), 'claude-opus-5-thinking-high');
  assert.equal(resolveModel('default'), null);
  assert.equal(resolveModel('search'), null);
});

test('通配符自动选最新模型，且同版本优先非 fast', () => {
  assert.equal(isGlobSpec('cursor-grok-*-high'), true);
  assert.equal(isGlobSpec('cursor-grok-4.5-high'), false);
  const candidates = [
    'cursor-grok-4.5-high',
    'cursor-grok-4.5-high-fast',
    'cursor-grok-4.6-high',
    'cursor-grok-4.6-high-fast',
    'claude-opus-5',
  ];
  assert.equal(pickLatestMatching('cursor-grok-*-high', candidates), 'cursor-grok-4.6-high');
  assert.equal(
    pickLatestMatching('cursor-grok-4.5-*', candidates),
    'cursor-grok-4.5-high',
  );
  const mgr = {
    parameterizedModelMap: new Map([
      ['cursor-grok-4.5-high', { modelId: 'cursor-grok-4.5-high', name: 'cursor-grok-4.5-high' }],
      ['cursor-grok-4.6-high', { modelId: 'cursor-grok-4.6-high', name: 'cursor-grok-4.6-high' }],
      [
        'cursor-grok-4.6-high-fast',
        { modelId: 'cursor-grok-4.6-high-fast', name: 'cursor-grok-4.6-high-fast' },
      ],
    ]),
  };
  assert.equal(expandModelSpec('cursor-grok-*-high', mgr), 'cursor-grok-4.6-high');
  assert.equal(expandModelSpec('cursor-grok-4.6-high', mgr), 'cursor-grok-4.6-high');
  assert.equal(expandModelSpec('cursor-grok-9.*-high', mgr), null);
});

test('normalizeModelId：thinking-high 别名与 mapModelToParameterizedSelection', () => {
  assert.equal(
    normalizeModelId(null, 'claude-opus-5-thinking-high'),
    'claude-opus-5',
  );
  const mgr = {
    mapModelToParameterizedSelection(id) {
      if (id === 'claude-opus-5-thinking-high') {
        return { modelId: 'claude-opus-5', parameters: [{ id: 'thinking', value: 'true' }] };
      }
      return null;
    },
  };
  assert.equal(normalizeModelId(mgr, 'claude-opus-5-thinking-high'), 'claude-opus-5');
  assert.equal(modelsEquivalent(mgr, 'claude-opus-5-thinking-high', 'claude-opus-5'), true);
  assert.equal(modelsEquivalent(mgr, 'cursor-grok-4.5-high-fast', 'claude-opus-5'), false);
});

test('通配符 *-high 不含 fast；*-high-fast 才要 fast', () => {
  assert.equal(specWantsFast('cursor-grok-*-high'), false);
  assert.equal(specWantsFast('cursor-grok-4.6-high'), false);
  assert.equal(specWantsFast('cursor-grok-*-high-fast'), true);
  assert.equal(specWantsFast('cursor-grok-4.6-high-fast'), true);
  const noFast = applySpecParameterIntent('cursor-grok-4.6-high', [
    { id: 'effort', value: 'high' },
    { id: 'fast', value: 'true' },
  ]);
  assert.equal(
    noFast.find((p) => p.id === 'fast').value,
    'false',
  );
  const yesFast = applySpecParameterIntent('cursor-grok-4.6-high-fast', [
    { id: 'effort', value: 'high' },
    { id: 'fast', value: 'false' },
  ]);
  assert.equal(
    yesFast.find((p) => p.id === 'fast').value,
    'true',
  );
});

test('selectionMatchesSpec：同 grok-4.6 但 fast 不同则不匹配', () => {
  const mgr = {
    currentSelectedModel: {
      modelId: 'grok-4.6',
      parameters: [
        { id: 'effort', value: 'high' },
        { id: 'fast', value: 'true' },
      ],
    },
    currentModel: { modelId: 'grok-4.6' },
    mapModelToParameterizedSelection(id) {
      if (String(id).startsWith('cursor-grok-4.6') || id === 'grok-4.6') {
        return {
          modelId: 'grok-4.6',
          parameters: [
            { id: 'effort', value: 'high' },
            { id: 'fast', value: 'true' },
          ],
        };
      }
      return null;
    },
  };
  globalThis.__cursorModeModelConfig = {};
  assert.equal(selectionMatchesSpec(mgr, 'cursor-grok-4.6-high'), false);
  forceSelectedModel(mgr, 'cursor-grok-4.6-high');
  assert.equal(mgr.currentSelectedModel.parameters.find((p) => p.id === 'fast').value, 'false');
  assert.equal(selectionMatchesSpec(mgr, 'cursor-grok-4.6-high'), true);
});

test('patchSource 命中全部锚点', () => {
  const src =
    anchors.setCurrentModel +
    'BODY1}' +
    anchors.setCurrentModelWithParameters +
    'BODY2}' +
    anchors.setModelFromStoredId +
    'BODY3}' +
    anchors.getCurrentModel +
    ';' +
    anchors.setMetadata +
    ';' +
    anchors.buildRequestedModel +
    'return o}';
  const { source, hits } = patchSource(src);
  assert.equal(hits, 6);
  assert.ok(source.includes('__cursorModeModelStash'));
  assert.ok(source.includes('__cursorModeModelBeforeBuild'));
  assert.ok(source.includes('__cursorModeModelSync'));
  // 幂等：已打过补丁不再改
  const again = patchSource(source);
  assert.equal(again.hits, 0);
});

test('shouldPatchUrl 只打 Cursor Agent 树', () => {
  assert.equal(
    shouldPatchUrl('file:///Users/x/.local/share/cursor-agent/versions/1/4378.index.js'),
    true,
  );
  assert.equal(shouldPatchUrl('file:///tmp/other/app.js'), false);
  assert.equal(shouldPatchUrl('node:fs'), false);
});
