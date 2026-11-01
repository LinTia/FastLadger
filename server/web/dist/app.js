/* FastLedger Web 端（WBS 5.9）——原生 JS，无构建工具链。
   安全口径：所有用户数据一律 textContent 渲染，禁止 innerHTML 拼接（防 XSS）；
   token 仅存 localStorage 并经 Authorization 头携带；401 统一清会话回登录页。
   金额全程整数分，渲染时用字符串拼接转元，禁止浮点参与金额运算。 */
"use strict";

const $ = (sel) => document.querySelector(sel);
const view = $("#view");
const API = "/api/v1";

const state = {
  token: localStorage.getItem("fl_token") || "",
  whoami: localStorage.getItem("fl_whoami") || "",
  bookId: 0,
  accounts: [],
  categories: [],
  sel: new Set(),          // 勾选的交易 id
  filter: { keyword: "", type: 0, accountId: 0, categoryId: 0, minDate: "", maxDate: "", page: 1, count: 50 },
};

/* ---------- 工具 ---------- */
function toast(msg, isErr) {
  const t = $("#toast");
  t.textContent = msg;
  t.classList.toggle("err", !!isErr);
  t.classList.remove("hidden");
  clearTimeout(t._h);
  t._h = setTimeout(() => t.classList.add("hidden"), 2600);
}

// 整数分 → "1,234.56"（字符串运算，无浮点）
function cents2str(cents) {
  const neg = cents < 0 ? "-" : "";
  const abs = Math.abs(cents).toString();
  const yuan = abs.length <= 2 ? "0" : abs.slice(0, abs.length - 2).replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const fen = abs.length === 1 ? "0" + abs : abs.slice(-2);
  return neg + yuan + "." + fen;
}
// "1234.56" → 123456 分（整数解析，两位小数校验；非法返回 null）
function str2cents(s) {
  s = s.trim().replace(/,/g, "").replace(/[¥￥]/g, "");
  const m = /^(\d+)(?:\.(\d{1,2}))?$/.exec(s);
  if (!m) return null;
  const whole = BigInt(m[1]) * 100n;
  const frac = BigInt((m[2] || "0").padEnd(2, "0"));
  const v = whole + frac;
  return v > BigInt(Number.MAX_SAFE_INTEGER) ? null : Number(v);
}
function fmtTime(unixSec) {
  const d = new Date(unixSec * 1000);
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}
function dateStartUnix(dateStr, endOfDay) {
  if (!dateStr) return null;
  const [y, m, d] = dateStr.split("-").map(Number);
  const t = new Date(y, m - 1, d, endOfDay ? 23 : 0, endOfDay ? 59 : 0, endOfDay ? 59 : 0);
  return Math.floor(t.getTime() / 1000);
}
const uuid = () => (crypto.randomUUID ? crypto.randomUUID() : "web-" + Date.now() + "-" + Math.random().toString(36).slice(2));

/* ---------- API ---------- */
async function api(method, path, body, isForm) {
  const opt = { method, headers: {} };
  if (state.token) opt.headers["Authorization"] = "Bearer " + state.token;
  if (body !== undefined) {
    if (isForm) { opt.body = body; }
    else { opt.headers["Content-Type"] = "application/json"; opt.body = JSON.stringify(body); }
  }
  const resp = await fetch(API + path, opt);
  let data = null;
  try { data = await resp.json(); } catch (_) { /* 非 JSON（如空体） */ }
  if (resp.status === 401) {
    clearSession();
    route();
    throw new Error(data && data.msg ? data.msg : "登录已失效，请重新登录");
  }
  if (!resp.ok) throw new Error(data && data.msg ? data.msg : "请求失败（HTTP " + resp.status + "）");
  return data.data;
}
function clearSession() {
  state.token = ""; state.whoami = "";
  localStorage.removeItem("fl_token");
  localStorage.removeItem("fl_whoami");
}

/* ---------- 顶部导航 ---------- */
function showNav(book) {
  $("#topbar").classList.remove("hidden");
  $("#logoutBtn").classList.remove("hidden");
  $("#whoami").textContent = state.whoami || "";
  if (book) {
    $("#navTxns").classList.remove("hidden");
    $("#navAccounts").classList.remove("hidden");
    $("#navRecycle").classList.remove("hidden");
    $("#navTxns").href = "#b/" + book.id;
    $("#navAccounts").href = "#b/" + book.id + "/accounts";
    $("#navRecycle").href = "#b/" + book.id + "/recycle";
  } else {
    $("#navTxns").classList.add("hidden");
    $("#navAccounts").classList.add("hidden");
    $("#navRecycle").classList.add("hidden");
  }
}
$("#logoutBtn").addEventListener("click", () => { clearSession(); route(); });
$("#brand").addEventListener("click", () => location.hash = "#books");

/* ---------- 登录 ---------- */
function renderLogin() {
  showNav(null);
  $("#topbar").classList.add("hidden");
  view.replaceChildren();
  const card = document.createElement("div");
  card.className = "card";
  const h = document.createElement("h2"); h.textContent = "登录 FastLedger";
  const f = document.createElement("form");
  const u = document.createElement("input"); u.placeholder = "用户名"; u.autocomplete = "username"; u.required = true;
  const p = document.createElement("input"); p.placeholder = "密码"; p.type = "password"; p.autocomplete = "current-password"; p.required = true;
  const b = document.createElement("button"); b.textContent = "登录"; b.style.marginTop = "10px";
  f.append(u, p, b);
  f.addEventListener("submit", async (e) => {
    e.preventDefault();
    b.disabled = true;
    try {
      const data = await api("POST", "/auth/login", { username: u.value, password: p.value });
      state.token = data.accessToken;
      state.whoami = u.value;
      localStorage.setItem("fl_token", state.token);
      localStorage.setItem("fl_whoami", u.value);
      location.hash = "#books";
    } catch (err) { toast(err.message, true); }
    finally { b.disabled = false; }
  });
  card.append(h, f);
  view.append(card);
}

/* ---------- 账本列表 ---------- */
async function renderBooks() {
  showNav(null);
  $("#navBooks").classList.add("active");
  view.replaceChildren();
  const card = document.createElement("div"); card.className = "card book-list";
  const h = document.createElement("h2"); h.textContent = "我的账本";
  card.append(h);
  const books = (await api("GET", "/books")) || [];
  if (!books.length) {
    const p = document.createElement("p"); p.className = "muted"; p.textContent = "暂无账本——先在客户端创建一个账本。";
    card.append(p);
  }
  for (const bk of books) {
    const a = document.createElement("a");
    a.href = "#b/" + bk.id;
    a.textContent = bk.name + (bk.bookType === 2 ? "（投资）" : "");
    card.append(a);
  }
  view.append(card);
}

/* ---------- 共享数据加载 ---------- */
async function loadBookMeta(bookId) {
  const [accs, cats] = await Promise.all([
    api("GET", "/books/" + bookId + "/accounts"),
    api("GET", "/books/" + bookId + "/categories"),
  ]);
  state.accounts = accs || [];
  state.categories = cats || [];
  return { accs, cats };
}
function accountName(id) {
  const a = state.accounts.find((x) => x.id === id);
  return a ? a.name : "#" + id;
}
function categoryName(id) {
  if (!id) return "—";
  const c = state.categories.find((x) => x.id === id);
  return c ? c.name : "#" + id;
}
function fillSelect(sel, items, blankLabel) {
  sel.replaceChildren();
  if (blankLabel !== undefined) {
    const o = document.createElement("option"); o.value = "0"; o.textContent = blankLabel; sel.append(o);
  }
  for (const it of items) {
    const o = document.createElement("option"); o.value = String(it.id); o.textContent = it.name; sel.append(o);
  }
}

/* ---------- 交易列表（浏览/检索/批量删除/记一笔） ---------- */
const TX_TYPE_NAME = { 1: "收入", 2: "支出", 3: "转账" };

async function renderTxns(bookId) {
  state.bookId = bookId;
  state.sel.clear();
  const { accs, cats } = await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  const book = books.find((b) => b.id === bookId);
  showNav(book || { id: bookId });
  $("#navTxns").classList.add("active");
  view.replaceChildren();

  const card = document.createElement("div"); card.className = "card";
  const h = document.createElement("h2");
  h.textContent = book ? book.name + " · 交易" : "交易";
  card.append(h);

  /* 筛选行 */
  const filters = document.createElement("div"); filters.className = "filters";
  const mk = (tag, ph) => { const el = document.createElement(tag); if (ph) el.placeholder = ph; return el; };
  const kw = mk("input", "关键词搜索"); kw.value = state.filter.keyword; kw.style.minWidth = "160px";
  const typeSel = mk("select"); fillSelect(typeSel, [], "全部类型");
  [[1, "收入"], [2, "支出"], [3, "转账"]].forEach(([v, n]) => { const o = document.createElement("option"); o.value = v; o.textContent = n; typeSel.append(o); });
  typeSel.value = String(state.filter.type || 0);
  const accSel = mk("select"); fillSelect(accSel, accs, "全部账户"); accSel.value = String(state.filter.accountId || 0);
  const catSel = mk("select"); fillSelect(catSel, cats, "全部分类"); catSel.value = String(state.filter.categoryId || 0);
  const dFrom = mk("input"); dFrom.type = "date"; dFrom.value = state.filter.minDate;
  const dTo = mk("input"); dTo.type = "date"; dTo.value = state.filter.maxDate;
  const go = document.createElement("button"); go.textContent = "查询";
  const reset = document.createElement("button"); reset.textContent = "重置"; reset.className = "ghost";
  filters.append(kw, typeSel, accSel, catSel, dFrom, dTo, go, reset);
  card.append(filters);

  /* 批量操作条 */
  const bulk = document.createElement("div"); bulk.className = "bulkbar";
  const allChk = document.createElement("input"); allChk.type = "checkbox";
  const allLbl = document.createElement("label"); allLbl.textContent = "全选本页"; allLbl.className = "muted";
  const delBtn = document.createElement("button"); delBtn.textContent = "删除选中"; delBtn.className = "danger"; delBtn.disabled = true;
  const addBtn = document.createElement("button"); addBtn.textContent = "＋ 记一笔";
  const selInfo = document.createElement("span"); selInfo.className = "muted";
  bulk.append(allChk, allLbl, selInfo, delBtn, addBtn);
  card.append(bulk);

  const refreshSel = () => {
    delBtn.disabled = state.sel.size === 0;
    selInfo.textContent = state.sel.size ? "已选 " + state.sel.size + " 项" : "";
  };

  /* 表格 */
  const tableWrap = document.createElement("div");
  card.append(tableWrap);

  async function load() {
    const f = state.filter;
    const q = new URLSearchParams();
    q.set("page", String(f.page)); q.set("count", String(f.count));
    if (f.keyword) q.set("keyword", f.keyword);
    if (f.type) q.set("type", String(f.type));
    if (f.accountId) q.set("accountId", String(f.accountId));
    if (f.categoryId) q.set("categoryId", String(f.categoryId));
    const mn = dateStartUnix(f.minDate, false), mx = dateStartUnix(f.maxDate, true);
    if (mn !== null) q.set("minTime", String(mn));
    if (mx !== null) q.set("maxTime", String(mx));
    const data = await api("GET", "/books/" + bookId + "/transactions?" + q.toString());
    state.sel.clear(); refreshSel();
    renderTable(data);
  }

  function renderTable(data) {
    // 防御：老后端对空列表可能返回 null，统一兜底为空数组
    const items = data.items || [];
    tableWrap.replaceChildren();
    const table = document.createElement("table");
    const thead = document.createElement("thead");
    const trh = document.createElement("tr");
    ["", "日期", "类型", "账户", "分类", "备注/商户", "金额"].forEach((t, i) => {
      const th = document.createElement("th"); th.textContent = t; if (i === 6) th.className = "num"; trh.append(th);
    });
    thead.append(trh); table.append(thead);
    const tbody = document.createElement("tbody");
    for (const tx of items) {
      const tr = document.createElement("tr");
      const tdC = document.createElement("td");
      const chk = document.createElement("input"); chk.type = "checkbox"; chk.dataset.id = String(tx.id);
      chk.addEventListener("change", () => {
        if (chk.checked) state.sel.add(tx.id); else state.sel.delete(tx.id);
        refreshSel();
      });
      tdC.append(chk);
      const tdT = document.createElement("td"); tdT.textContent = fmtTime(tx.txTime);
      const tdTy = document.createElement("td"); tdTy.textContent = TX_TYPE_NAME[tx.txType] || tx.txType;
      const tdA = document.createElement("td");
      tdA.textContent = tx.txType === 3 ? accountName(tx.accountId) + " → " + accountName(tx.toAccountId) : accountName(tx.accountId);
      const tdCat = document.createElement("td"); tdCat.textContent = tx.txType === 3 ? "—" : categoryName(tx.categoryId);
      const tdN = document.createElement("td");
      tdN.textContent = (tx.note || "") + (tx.merchant ? (tx.note ? " · " : "") + tx.merchant : "");
      tdN.className = "muted";
      const tdM = document.createElement("td"); tdM.className = "num " + (tx.txType === 1 ? "income" : tx.txType === 2 ? "expense" : "transfer");
      tdM.textContent = (tx.txType === 1 ? "+" : tx.txType === 2 ? "-" : "") + "¥" + cents2str(tx.amount);
      tr.append(tdC, tdT, tdTy, tdA, tdCat, tdN, tdM);
      tbody.append(tr);
    }
    if (!items.length) {
      const tr = document.createElement("tr");
      const td = document.createElement("td"); td.colSpan = 7; td.className = "muted"; td.textContent = "没有符合条件的交易。";
      tr.append(td); tbody.append(tr);
    }
    table.append(tbody); tableWrap.append(table);

    const pager = document.createElement("div"); pager.className = "pager";
    const prev = document.createElement("button"); prev.textContent = "上一页"; prev.className = "ghost"; prev.disabled = data.page <= 1;
    const next = document.createElement("button"); next.textContent = "下一页"; next.className = "ghost";
    next.disabled = data.page * data.count >= data.total;
    const info = document.createElement("span"); info.className = "muted";
    info.textContent = "共 " + data.total + " 条 · 第 " + data.page + " 页";
    prev.addEventListener("click", () => { state.filter.page--; load(); });
    next.addEventListener("click", () => { state.filter.page++; load(); });
    pager.append(prev, info, next);
    tableWrap.append(pager);
  }

  allChk.addEventListener("change", () => {
    document.querySelectorAll("tbody input[type=checkbox]").forEach((chk) => {
      chk.checked = allChk.checked;
      const id = Number(chk.dataset.id);
      if (allChk.checked) state.sel.add(id); else state.sel.delete(id);
    });
    refreshSel();
  });
  delBtn.addEventListener("click", async () => {
    if (!state.sel.size) return;
    if (!confirm("确认删除选中的 " + state.sel.size + " 条交易？删除后可在回收站恢复。")) return;
    delBtn.disabled = true;
    let okN = 0, failN = 0;
    for (const id of state.sel) {
      try {
        await api("DELETE", "/books/" + bookId + "/transactions/" + id + "?clientUuid=" + encodeURIComponent(uuid()));
        okN++;
      } catch (_) { failN++; }
    }
    toast("删除完成：成功 " + okN + "，失败 " + failN, failN > 0);
    state.filter.page = 1;
    await load();
  });
  addBtn.addEventListener("click", () => openTxForm(bookId, load));
  [kw, typeSel, accSel, catSel, dFrom, dTo].forEach((el) => {
    el.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); applyFilters(); } });
  });
  function applyFilters() {
    state.filter = { keyword: kw.value.trim(), type: Number(typeSel.value), accountId: Number(accSel.value), categoryId: Number(catSel.value), minDate: dFrom.value, maxDate: dTo.value, page: 1, count: state.filter.count };
    load().catch((e) => toast(e.message, true));
  }
  go.addEventListener("click", applyFilters);
  reset.addEventListener("click", () => { kw.value = ""; typeSel.value = "0"; accSel.value = "0"; catSel.value = "0"; dFrom.value = ""; dTo.value = ""; applyFilters(); });

  try { await load(); } catch (e) { toast(e.message, true); }
}

/* ---------- 记一笔 ---------- */
function openTxForm(bookId, onDone) {
  const dlg = $("#formDlg");
  $("#fType").value = "2";
  fillSelect($("#fAccount"), state.accounts);
  fillSelect($("#fTo"), state.accounts);
  fillSelect($("#fCategory"), state.categories);
  $("#fDate").value = new Date().toISOString().slice(0, 10);
  $("#fNote").value = "";
  $("#fAmount").value = "";
  syncFormFields();
  dlg.classList.remove("hidden");
  if (typeof dlg.showModal === "function") dlg.showModal(); else dlg.setAttribute("open", "");
  function syncFormFields() {
    const t = Number($("#fType").value);
    $("#fToRow").classList.toggle("hidden", t !== 3);
    $("#fCatRow").classList.toggle("hidden", t === 3);
  }
  $("#fType").onchange = syncFormFields;
  $("#fCancel").onclick = () => { dlg.close ? dlg.close() : dlg.removeAttribute("open"); dlg.classList.add("hidden"); };
  $("#txForm").onsubmit = async (e) => {
    e.preventDefault();
    const cents = str2cents($("#fAmount").value);
    if (cents === null || cents <= 0) { toast("金额格式不合法（最多两位小数）", true); return; }
    const t = Number($("#fType").value);
    const body = {
      clientUuid: uuid(),
      txType: t,
      accountId: Number($("#fAccount").value),
      amount: cents,
      categoryId: t === 3 ? 0 : Number($("#fCategory").value),
      txTime: dateStartUnix($("#fDate").value, false),
      timezoneOffset: -new Date().getTimezoneOffset(),
      comment: $("#fNote").value,
      source: 0,
    };
    if (t === 3) {
      body.toAccountId = Number($("#fTo").value);
      if (!body.toAccountId || body.toAccountId === body.accountId) { toast("转账需选择不同的两个账户", true); return; }
    } else if (!body.accountId) { toast("请选择账户", true); return; }
    try {
      await api("POST", "/books/" + bookId + "/transactions", body);
      toast("已记账（等待你确认无误后生效）");
      dlg.close ? dlg.close() : dlg.removeAttribute("open");
      dlg.classList.add("hidden");
      onDone();
    } catch (err) { toast(err.message, true); }
  };
}

/* ---------- 对账（账户余额） ---------- */
const ACC_CAT_NAME = { 1: "现金", 2: "活期储蓄", 3: "定期存款", 4: "电子钱包", 5: "投资账户", 6: "实物资产", 7: "保险/年金", 8: "其他资产", 9: "应收款", 10: "信用卡", 11: "信用消费贷", 12: "房贷", 13: "车贷", 14: "其他借款", 15: "应付款" };

async function renderAccounts(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navAccounts").classList.add("active");
  view.replaceChildren();
  const card = document.createElement("div"); card.className = "card";
  const h = document.createElement("h2"); h.textContent = "账户对账";
  const p = document.createElement("p"); p.className = "muted"; p.textContent = "各账户当前余额（负债账户为欠款）。";
  card.append(h, p);
  const table = document.createElement("table");
  const thead = document.createElement("thead");
  const trh = document.createElement("tr");
  ["账户", "类型", "余额"].forEach((t, i) => { const th = document.createElement("th"); th.textContent = t; if (i === 2) th.className = "num"; trh.append(th); });
  thead.append(trh); table.append(thead);
  const tbody = document.createElement("tbody");
  for (const a of state.accounts) {
    const tr = document.createElement("tr");
    const tdN = document.createElement("td"); tdN.textContent = a.name;
    const tdC = document.createElement("td"); tdC.textContent = ACC_CAT_NAME[a.category] || ("类别 " + a.category); tdC.className = "muted";
    const tdB = document.createElement("td"); tdB.className = "num " + (a.balance < 0 ? "balance-neg" : "balance-pos");
    tdB.textContent = (a.currency || "CNY") === "CNY" ? "¥" + cents2str(a.balance) : a.currency + " " + cents2str(a.balance);
    tr.append(tdN, tdC, tdB);
    tbody.append(tr);
  }
  table.append(tbody); card.append(table);
  view.append(card);
}

/* ---------- 回收站 ---------- */
async function renderRecycle(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navRecycle").classList.add("active");
  view.replaceChildren();
  const card = document.createElement("div"); card.className = "card";
  card.append(Object.assign(document.createElement("h2"), { textContent: "回收站" }));
  const table = document.createElement("table");
  const thead = document.createElement("thead");
  const trh = document.createElement("tr");
  ["删除时间", "类型", "账户", "备注", "金额", ""].forEach((t, i) => { const th = document.createElement("th"); th.textContent = t; if (i === 4) th.className = "num"; trh.append(th); });
  thead.append(trh); table.append(thead);
  const tbody = document.createElement("tbody");
  const data = await api("GET", "/books/" + bookId + "/recycle?page=1&count=100");
  for (const tx of data.items || []) {
    const tr = document.createElement("tr");
    const mkTd = (t, cls) => { const td = document.createElement("td"); td.textContent = t; if (cls) td.className = cls; return td; };
    tr.append(
      mkTd(tx.deletedAt ? fmtTime(tx.deletedAt) : "—", "muted"),
      mkTd(TX_TYPE_NAME[tx.txType] || tx.txType),
      mkTd(tx.txType === 3 ? accountName(tx.accountId) + " → " + accountName(tx.toAccountId) : accountName(tx.accountId)),
      mkTd(tx.note || "", "muted"),
      mkTd("¥" + cents2str(tx.amount), "num")
    );
    const tdA = document.createElement("td");
    const btn = document.createElement("button"); btn.textContent = "恢复"; btn.className = "ghost";
    btn.addEventListener("click", async () => {
      btn.disabled = true;
      try {
        await api("POST", "/books/" + bookId + "/transactions/" + tx.id + "/restore?clientUuid=" + encodeURIComponent(uuid()));
        toast("已恢复");
        tr.remove();
      } catch (e) { toast(e.message, true); btn.disabled = false; }
    });
    tdA.append(btn); tr.append(tdA);
    tbody.append(tr);
  }
  if (!(data.items || []).length) {
    const tr = document.createElement("tr");
    const td = document.createElement("td"); td.colSpan = 6; td.className = "muted"; td.textContent = "回收站为空。";
    tr.append(td); tbody.append(tr);
  }
  table.append(tbody); card.append(table);
  view.append(card);
}

/* ---------- 路由 ---------- */
async function route() {
  const hash = location.hash || "#";
  const m = hash.match(/^#b\/(\d+)(?:\/(accounts|recycle))?/);
  try {
    if (!state.token) { renderLogin(); return; }
    if (m) {
      const bookId = Number(m[1]);
      if (m[2] === "accounts") await renderAccounts(bookId);
      else if (m[2] === "recycle") await renderRecycle(bookId);
      else await renderTxns(bookId);
    } else if (hash === "#books" || hash === "#") {
      await renderBooks();
    } else {
      await renderBooks();
    }
  } catch (e) {
    toast(e.message, true);
    if (!state.token) renderLogin();
  }
}
window.addEventListener("hashchange", route);
route();
