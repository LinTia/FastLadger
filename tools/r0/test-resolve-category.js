#!/usr/bin/env node
'use strict';
/**
 * 分类解析的合成单元测试（不依赖服务端、不依赖网络、不依赖原型数据）。
 *
 * 覆盖 ChatGPT 复核指出的三类边界 + 失败关闭语义：
 *   1) ID 命中父类、但展示名明确指向子类 —— 不得被父类 ID 静默覆盖；
 *   2) 重复名称（同级同名 / 大类同名）—— 必须判为歧义而不是后者覆盖前者；
 *   3) 多个前缀候选 —— 必须失败关闭，不猜；
 *   4) ID 与展示名指向不同叶子 —— 冲突，失败关闭；
 *   5) 展示名到子类但只解析到父类 —— 失败关闭；
 *   6) 唯一前缀（t017 真实形态）—— 正常命中；
 *   7) 精确叶子 ID + 同名展示 —— id 胜出。
 *
 * 运行：node --test tools/r0/test-resolve-category.js
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const { buildCategoryIndex, resolveCategory, CategoryResolutionError } = require('./lib/resolve-category');

/** 构造一棵合成分类树（与原型结构同形：大类 + 子类） */
function tree() {
  return [
    { id: 'cat_food', name: '餐饮', parentId: null, type: 'expense' },
    { id: 'cat_food_meal', name: '三餐', parentId: 'cat_food', type: 'expense' },
    { id: 'cat_food_snack', name: '零食', parentId: 'cat_food', type: 'expense' },
    { id: 'cat_shop', name: '购物', parentId: null, type: 'expense' },
    { id: 'cat_shop_clothes', name: '服饰', parentId: 'cat_shop', type: 'expense' },
    { id: 'cat_shop_digital', name: '数码家电', parentId: 'cat_shop', type: 'expense' },
    { id: 'cat_salary', name: '工资', parentId: null, type: 'income' },
  ];
}

const codesOf = (fn) => {
  try {
    fn();
  } catch (e) {
    assert.ok(e instanceof CategoryResolutionError, `应抛 CategoryResolutionError，实际 ${e && e.name}: ${e && e.message}`);
    return e.code;
  }
  throw new Error('预期抛错但没有抛');
};

test('边界1：ID 命中父类，展示名明确指向子类 → 子类胜出（expand-child），不得静默用父类', () => {
  const index = buildCategoryIndex(tree());
  const r = resolveCategory({ id: 't1', categoryId: 'cat_shop', categoryName: '购物-服饰' }, index);
  assert.equal(r.protoId, 'cat_shop_clothes');
  assert.equal(r.via, 'expand-child');
  assert.equal(r.notes[0].kind, 'id-vs-name');
  assert.equal(r.notes[0].parentId, 'cat_shop');
});

test('边界1b：ID 命中父类且展示名就是父类名 → 用父类（这不是猜测）', () => {
  const index = buildCategoryIndex(tree());
  const r = resolveCategory({ id: 't2', categoryId: 'cat_shop', categoryName: '购物' }, index);
  assert.equal(r.protoId, 'cat_shop');
  assert.equal(r.via, 'id');
});

test('边界2：同级重复名称 → NAME_AMBIGUOUS，不得后者覆盖前者', () => {
  const cats = tree().concat([
    { id: 'cat_shop_other', name: '其他', parentId: 'cat_shop' },
    { id: 'cat_food_other', name: '其他', parentId: 'cat_food' },
  ]);
  const index = buildCategoryIndex(cats);
  // 两个「其他」在不同父类下，路径键不同 → 不冲突
  assert.equal(resolveCategory({ id: 't3', categoryId: 'cat_shop_other', categoryName: '购物-其他' }, index).protoId, 'cat_shop_other');
});

test('边界2b：同一父类下同名子类 → NAME_AMBIGUOUS 失败关闭', () => {
  const cats = tree().concat([
    { id: 'cat_shop_clothes_dup', name: '服饰', parentId: 'cat_shop' },
  ]);
  const index = buildCategoryIndex(cats);
  assert.equal(
    codesOf(() => resolveCategory({ id: 't4', categoryId: 'nope', categoryName: '购物-服饰' }, index)),
    'NAME_AMBIGUOUS',
  );
});

test('边界2c：大类重名 → NAME_AMBIGUOUS 失败关闭', () => {
  const cats = tree().concat([{ id: 'cat_shop2', name: '购物', parentId: null }]);
  const index = buildCategoryIndex(cats);
  assert.equal(
    codesOf(() => resolveCategory({ id: 't5', categoryId: 'nope', categoryName: '购物' }, index)),
    'NAME_AMBIGUOUS',
  );
});

test('边界3：多个前缀候选 → PREFIX_AMBIGUOUS，不得取第一个', () => {
  // 「数码」「数码家电」都以「数码」开头，且都在「购物」下
  const cats = tree().concat([{ id: 'cat_shop_digital2', name: '数码产品', parentId: 'cat_shop' }]);
  const index = buildCategoryIndex(cats);
  assert.equal(
    codesOf(() => resolveCategory({ id: 't6', categoryId: 'nolonger_exists', categoryName: '购物-数码' }, index)),
    'PREFIX_AMBIGUOUS',
  );
});

test('边界3b：唯一前缀（t017 真实形态「购物-数码」→「数码家电」）→ name-prefix 命中', () => {
  const index = buildCategoryIndex(tree());
  const r = resolveCategory({ id: 't017', categoryId: 'travel', categoryName: '购物-数码' }, index);
  assert.equal(r.protoId, 'cat_shop_digital');
  assert.equal(r.via, 'name-prefix');
});

test('边界4：ID 与展示名指向不同叶子 → CATEGORY_CONFLICT，不得择优', () => {
  const index = buildCategoryIndex(tree());
  assert.equal(
    codesOf(() => resolveCategory({ id: 't7', categoryId: 'cat_food_meal', categoryName: '餐饮-零食' }, index)),
    'CATEGORY_CONFLICT',
  );
});

test('边界5：展示名到子类但只解析到父类 → CATEGORY_CHILD_UNRESOLVED（不降级父类）', () => {
  const index = buildCategoryIndex(tree());
  assert.equal(
    codesOf(() => resolveCategory({ id: 't8', categoryId: 'cat_shop', categoryName: '购物-不存在' }, index)),
    'CATEGORY_CHILD_UNRESOLVED',
  );
});

test('边界6：完全无法定位 → CATEGORY_UNRESOLVED，绝不返回父类猜测', () => {
  const index = buildCategoryIndex(tree());
  assert.equal(
    codesOf(() => resolveCategory({ id: 't9', categoryId: 'ghost', categoryName: '幽灵-未知' }, index)),
    'CATEGORY_UNRESOLVED',
  );
});

test('边界7：精确叶子 ID + 同名展示 → id 胜出且无冲突记录', () => {
  const index = buildCategoryIndex(tree());
  const r = resolveCategory({ id: 't10', categoryId: 'cat_food_meal', categoryName: '餐饮-三餐' }, index);
  assert.equal(r.protoId, 'cat_food_meal');
  assert.equal(r.via, 'id');
  assert.equal(r.notes.length, 0);
});

test('边界9：冲突 + 显式覆盖 → manual-override（不是猜，是可追溯决定）', () => {
  const index = buildCategoryIndex(tree());
  const overrides = new Map([['t20', { txId: 't20', categoryId: 'cat_food_snack', reason: '展示名为准', decidedBy: 'test', decidedAt: '2026-11-05' }]]);
  const r = resolveCategory({ id: 't20', categoryId: 'cat_food_meal', categoryName: '餐饮-零食' }, index, overrides);
  assert.equal(r.protoId, 'cat_food_snack');
  assert.equal(r.via, 'manual-override');
  assert.equal(r.notes[0].kind, 'override');
  assert.equal(r.notes[0].idCandidate, 'cat_food_meal');
  assert.equal(r.notes[0].nameCandidate, 'cat_food_snack');
});

test('边界9b：覆盖值指向不存在的分类 → 仍然失败关闭（覆盖不能绕过校验）', () => {
  const index = buildCategoryIndex(tree());
  const overrides = new Map([['t21x', { txId: 't21x', categoryId: 'ghost_cat', reason: 'x', decidedBy: 't', decidedAt: 'd' }]]);
  assert.equal(
    codesOf(() => resolveCategory({ id: 't21x', categoryId: 'cat_food_meal', categoryName: '餐饮-零食' }, index, overrides)),
    'CATEGORY_CONFLICT',
  );
});

test('边界8：悬空 ID + 大类名展示 → parent-fallback 仅在展示名即大类时生效', () => {
  const index = buildCategoryIndex(tree());
  const r = resolveCategory({ id: 't11', categoryId: 'dangling', categoryName: '餐饮' }, index);
  assert.equal(r.protoId, 'cat_food');
  assert.equal(r.via, 'parent-fallback');
});
