'use strict';
/**
 * R0 包 B · 分类解析（纯函数，可单测）
 *
 * 背景：原型数据集里存在"分类 ID 悬空"与"展示名 ≠ 分类实名"两类数据瑕疵，
 * 早期实现把"补 cat_ 前缀"排在名称匹配之前，导致 4 笔交易被打到父类。
 * 本模块把解析规则固化，并把"猜"全部改成"失败关闭"。
 *
 * 解析优先级（与 seed-from-dataset.js 顶部注释必须一致）：
 *   1. 精确 ID 命中（id）；
 *   2. 规范化 ID：补 `cat_` 前缀后命中（id-normalized）；
 *   3. 展示名精确命中子类路径「父-子」（name-path，仅在名称唯一时）；
 *   4. 展示名精确命中大类名（name-path-top，仅在名称唯一时）；
 *   5. 同父类下"唯一前缀"匹配（name-prefix，多于 1 个候选即失败关闭）；
 *   6. 大类兜底（parent-fallback）：**仅当展示名本身就是大类名**（不含「-」）时允许，
 *      不是猜测；其余情况一律失败关闭。
 *
 * 冲突规则（关键）：
 *   - 若 ID 命中的是**父类**，而展示名指向该父类下的**子类** → 子类胜出（expand-child），
 *     并记录冲突；绝不允许父类 ID 静默覆盖明确的子类展示路径。
 *   - 若 ID 与展示名指向**两个不同的叶子** → 失败关闭（CATEGORY_CONFLICT），不择优。
 *   - 若展示名含「-」（明确到子类）但只解析到父类 → 失败关闭（CATEGORY_CHILD_UNRESOLVED）。
 *
 * 歧义规则：
 *   - 同级同名 / 大类同名 → NAME_AMBIGUOUS；
 *   - 前缀候选 > 1 → PREFIX_AMBIGUOUS。
 *
 * 显式覆盖（override）：
 *   原型演示数据存在"ID 与展示名互相矛盾且都不是父类"的情况（例：t021 的
 *   categoryId=meal「三餐」而展示名是「餐饮-聚餐」dinner）。规则要求此时失败关闭，
 *   不允许脚本择优。若人工已判定应当采用哪一个，可写入
 *   `tools/r0/category-overrides.json`（含 reason / decidedBy / decidedAt），
 *   解析器只在"确实冲突"时才采用，并把 via 记为 `manual-override` 供审计追溯。
 *   覆盖不是"猜"：它是可复核的显式决定，且只在冲突时生效。
 */

class CategoryResolutionError extends Error {
  constructor(code, message, detail) {
    super(message);
    this.name = 'CategoryResolutionError';
    this.code = code;
    this.detail = detail || {};
  }
}

/** 构造索引；重复名称会被记为 ambiguous，而不是后写覆盖先写。 */
function buildCategoryIndex(categories) {
  const byId = new Map();
  for (const c of categories) byId.set(c.id, c);

  const childrenOf = new Map(); // parentId -> [child]
  for (const c of categories) {
    if (c.parentId === null || c.parentId === undefined) continue;
    if (!childrenOf.has(c.parentId)) childrenOf.set(c.parentId, []);
    childrenOf.get(c.parentId).push(c);
  }

  /** 名称索引：value 为 { id } 或 { ambiguous: string[] } */
  const put = (map, key, id) => {
    const cur = map.get(key);
    if (!cur) {
      map.set(key, { id, ids: [id] });
      return;
    }
    cur.ids.push(id);
  };

  const topByName = new Map();
  const childByPath = new Map();
  const childrenByParentName = new Map();

  for (const c of categories) {
    if (c.parentId === null || c.parentId === undefined) {
      put(topByName, c.name, c.id);
    }
  }
  for (const c of categories) {
    if (c.parentId === null || c.parentId === undefined) continue;
    const parent = byId.get(c.parentId);
    if (!parent) continue;
    put(childByPath, `${parent.name}-${c.name}`, c.id);
    if (!childrenByParentName.has(parent.name)) childrenByParentName.set(parent.name, []);
    childrenByParentName.get(parent.name).push(c);
  }

  const lookupUnique = (map, key, whatIndex) => {
    const hit = map.get(key);
    if (!hit) return null;
    if (hit.ids.length > 1) {
      throw new CategoryResolutionError('NAME_AMBIGUOUS', `分类名「${key}」在${whatIndex}中重复（${hit.ids.join(', ')}），拒绝猜测`, {
        key,
        candidates: hit.ids,
      });
    }
    return hit.id;
  };

  return { byId, childrenOf, topByName, childByPath, childrenByParentName, lookupUnique };
}

const isLeaf = (index, id) => {
  const kids = index.childrenOf.get(id);
  return !kids || kids.length === 0;
};
const isParent = (index, id) => !isLeaf(index, id);

/**
 * 解析单笔交易的分类。
 * @param {{id:string, categoryId:string, categoryName?:string}} tx
 * @param {ReturnType<typeof buildCategoryIndex>} index
 * @returns {{protoId:string, via:string, notes:Array<object>}}
 * @throws {CategoryResolutionError} 无法唯一定位时
 */
function resolveCategory(tx, index, overrides) {
  const display = String(tx.categoryName || '').trim();
  const rawId = String(tx.categoryId || '');
  const notes = [];

  // —— 候选收集（互不覆盖，最后统一裁决）——
  const idHit = index.byId.has(rawId) ? rawId : null;
  const normId = `cat_${rawId}`;
  const normHit = !idHit && index.byId.has(normId) ? normId : null;

  // 展示名 → 子类路径（仅在唯一时返回，重复名直接抛 NAME_AMBIGUOUS）
  let pathHit = null;
  let topHit = null;
  if (display) {
    pathHit = index.lookupUnique(index.childByPath, display, '子类路径');
    if (!pathHit && !display.includes('-')) {
      // 展示名本身就是大类名：记为大类兜底（不是猜测），重复名同样抛错
      topHit = index.lookupUnique(index.topByName, display, '大类');
    }
  }

  // 同父类唯一前缀（展示名形如「父-子」，但子类实名不完全一致，例如 t017「购物-数码」→「数码家电」）
  let prefixHit = null;
  let prefixCandidates = [];
  if (display && !pathHit && display.includes('-')) {
    const [parentName, childPart] = display.split('-');
    const kids = index.childrenByParentName.get(parentName) || [];
    prefixCandidates = kids
      .filter((c) => c.name.startsWith(childPart) || childPart.startsWith(c.name))
      .map((c) => c.id);
    if (prefixCandidates.length === 1) prefixHit = prefixCandidates[0];
  }

  // nameTarget 只表示"明确到子类"的命中；大类命中走 parent-fallback，语义不混
  const nameTarget = pathHit || prefixHit || null;
  const nameVia = pathHit ? 'name-path' : prefixHit ? 'name-prefix' : null;

  const override = overrides && overrides.get ? overrides.get(tx.id) : null;

  // —— 裁决 ——
  if (idHit) {
    if (!nameTarget) {
      if (isParent(index, idHit) && display.includes('-')) {
        // 明确到子类的展示名，却只解析到父类 → 不猜
        throw new CategoryResolutionError(
          'CATEGORY_CHILD_UNRESOLVED',
          `交易 ${tx.id}：分类 ID「${rawId}」是父类，展示名「${display}」指向子类但无法定位，拒绝降级到父类`,
          { txId: tx.id, rawId, display },
        );
      }
      return { protoId: idHit, via: 'id', notes };
    }
    if (idHit === nameTarget) return { protoId: idHit, via: 'id', notes };

    // 冲突：ID 与展示名不一致
    if (isParent(index, idHit) && index.byId.get(nameTarget)?.parentId === idHit) {
      notes.push({ kind: 'id-vs-name', resolution: 'expand-child', parentId: idHit, childId: nameTarget, display });
      return { protoId: nameTarget, via: 'expand-child', notes };
    }
    if (override && index.byId.has(override.categoryId)) {
      notes.push({ kind: 'override', applied: override.categoryId, idCandidate: idHit, nameCandidate: nameTarget, reason: override.reason });
      return { protoId: override.categoryId, via: 'manual-override', notes };
    }
    throw new CategoryResolutionError(
      'CATEGORY_CONFLICT',
      `交易 ${tx.id}：分类 ID「${rawId}」与展示名「${display}」（${nameTarget}）指向不同分类，拒绝择优` +
        '（如已人工判定，请写入 tools/r0/category-overrides.json）',
      { txId: tx.id, rawId, display, idHit, nameTarget, nameVia },
    );
  }

  if (normHit) {
    if (nameTarget && nameTarget !== normHit) {
      if (isParent(index, normHit) && index.byId.get(nameTarget)?.parentId === normHit) {
        notes.push({ kind: 'id-vs-name', resolution: 'expand-child', parentId: normHit, childId: nameTarget, display });
        return { protoId: nameTarget, via: 'expand-child', notes };
      }
      if (override && index.byId.has(override.categoryId)) {
        notes.push({ kind: 'override', applied: override.categoryId, idCandidate: normHit, nameCandidate: nameTarget, reason: override.reason });
        return { protoId: override.categoryId, via: 'manual-override', notes };
      }
      throw new CategoryResolutionError(
        'CATEGORY_CONFLICT',
        `交易 ${tx.id}：规范化 ID「${normId}」与展示名「${display}」（${nameTarget}）冲突（如已人工判定，请写入 tools/r0/category-overrides.json）`,
        { txId: tx.id, rawId, normId, display, nameTarget },
      );
    }
    return { protoId: normHit, via: 'id-normalized', notes };
  }

  if (nameTarget) {
    return { protoId: nameTarget, via: nameVia, notes };
  }

  if (prefixCandidates.length > 1) {
    throw new CategoryResolutionError(
      'PREFIX_AMBIGUOUS',
      `交易 ${tx.id}：名称「${display}」在父类下有 ${prefixCandidates.length} 个候选（${prefixCandidates.join(', ')}），拒绝猜测`,
      { txId: tx.id, display, candidates: prefixCandidates },
    );
  }

  // 大类兜底：只允许"展示名本身就是大类名"，不做 split('-') 猜测
  if (topHit) {
    notes.push({ kind: 'parent-fallback', used: topHit, display });
    return { protoId: topHit, via: 'parent-fallback', notes };
  }

  throw new CategoryResolutionError(
    'CATEGORY_UNRESOLVED',
    `交易 ${tx.id}：ID「${rawId}」与展示名「${display}」都无法唯一定位分类，失败关闭`,
    { txId: tx.id, rawId, display },
  );
}

module.exports = { buildCategoryIndex, resolveCategory, CategoryResolutionError };
