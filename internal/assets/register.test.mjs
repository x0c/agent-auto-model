import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

process.env.CURSOR_MODE_MODEL_UNIT_TEST = '1';

const {
  currentMode,
  modeKnown,
  resolveModel,
  normalizeModelId,
  modelsEquivalent,
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

test('resolveModel 按配置映射', () => {
  assert.equal(resolveModel('plan'), 'claude-opus-5-thinking-high');
  assert.equal(resolveModel('default'), 'cursor-grok-4.5-high-fast');
  assert.equal(resolveModel('search'), 'cursor-grok-4.5-high-fast');
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
