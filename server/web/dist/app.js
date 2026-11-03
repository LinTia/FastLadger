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
// toast(msg, type)：type ∈ "ok" | "err" | "info"（sonner 风格变体，带前缀图标）
function toast(msg, type) {
  if (type !== "ok" && type !== "err" && type !== "info") type = "info";
  const t = $("#toast");
  t.className = type;                       // 重置 + 设定变体（触发重播动画）
  t.textContent = (type === "ok" ? "✓ " : type === "err" ? "✕ " : "ℹ ") + msg;
  clearTimeout(t._h);
  t._h = setTimeout(() => {
    t.classList.add("out");                 // 先播滑出动画再隐藏
    setTimeout(() => t.classList.add("hidden"), 200);
  }, 2600);
}

// Promise 化确认对话框（替代原生 confirm()，对齐原型 AlertDialog：标题+描述+取消/确认）
function confirmDialog(opts) {
  const dlg = $("#confirmDlg");
  $("#cTitle").textContent = opts.title || "确认操作";
  $("#cDesc").textContent = opts.desc || "";
  const ok = $("#cOk");
  ok.textContent = opts.confirmText || "确认";
  ok.className = opts.danger ? "danger" : "";
  dlg.classList.remove("hidden");
  if (typeof dlg.showModal === "function") dlg.showModal(); else dlg.setAttribute("open", "");
  return new Promise((resolve) => {
    $("#cCancel").onclick = () => { close(); resolve(false); };
    $("#confirmForm").onsubmit = (e) => { e.preventDefault(); close(); resolve(true); };
    function close() {
      dlg.close ? dlg.close() : dlg.removeAttribute("open");
      dlg.classList.add("hidden");
    }
  });
}

// 按钮 pending 态：禁用 + 文案切换，返回恢复函数
function btnPending(btn, loadingText) {
  const old = btn.textContent;
  btn.disabled = true;
  if (loadingText) btn.textContent = loadingText;
  return () => { btn.disabled = false; btn.textContent = old; };
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
  const bn = $("#bottomNav"); if (bn) bn.classList.toggle("hidden", !book);
  const fab = $("#fab"); if (fab) fab.classList.toggle("hidden", !book);
  $("#topbar").classList.remove("hidden");
  $("#logoutBtn").classList.remove("hidden");
  $("#whoami").textContent = state.whoami || "";
  if (book) {
    $("#navTxns").classList.remove("hidden");
    $("#navReport").classList.remove("hidden");
    $("#navBudget").classList.remove("hidden");
    $("#navInvest").classList.remove("hidden");
    $("#navExport").classList.remove("hidden");
    $("#navAccounts").classList.remove("hidden");
    $("#navMembers").classList.remove("hidden");
    $("#navRecycle").classList.remove("hidden");
    $("#navTxns").href = "#b/" + book.id;
    $("#navReport").href = "#b/" + book.id + "/report";
    $("#navBudget").href = "#b/" + book.id + "/budget";
    $("#navInvest").href = "#b/" + book.id + "/invest";
    $("#navExport").href = "#b/" + book.id + "/export";
    $("#navAccounts").href = "#b/" + book.id + "/accounts";
    $("#navMembers").href = "#b/" + book.id + "/members";
    $("#navRecycle").href = "#b/" + book.id + "/recycle";
  } else {
    $("#navTxns").classList.add("hidden");
    $("#navReport").classList.add("hidden");
    $("#navBudget").classList.add("hidden");
    $("#navInvest").classList.add("hidden");
    $("#navExport").classList.add("hidden");
    $("#navAccounts").classList.add("hidden");
    $("#navMembers").classList.add("hidden");
    $("#navRecycle").classList.add("hidden");
  }
  $("#navSettings").classList.toggle("hidden", !state.token);
}
$("#logoutBtn").addEventListener("click", () => { clearSession(); route(); });
$("#brand").addEventListener("click", () => location.hash = "#books");

/* ---------- 登录/注册（对齐原型：logo 块 + 副标题 + loading + 成功 toast） ----------
   注册模式为本机试用/自托管新增（PR：登录页无注册入口）；服务端契约：
   POST /auth/register {username,password} → 仅建号（不返回 token），成功后自动登录。 */
function renderLogin(mode) {
  const isRegister = mode === "register";
  showNav(null);
  $("#topbar").classList.add("hidden");
  view.replaceChildren();
  const card = document.createElement("div");
  card.className = "card";
  card.style.maxWidth = "380px";
  card.style.margin = "48px auto";
  const logo = document.createElement("div");
  logo.style.cssText = "font-size:36px;text-align:center;";
  logo.textContent = "📒";
  const h = document.createElement("h2"); h.textContent = isRegister ? "注册 FastLedger" : "登录 FastLedger"; h.style.textAlign = "center";
  const sub = document.createElement("p"); sub.className = "muted"; sub.style.textAlign = "center";
  sub.textContent = "家庭智能记账 · 截图秒记";
  const f = document.createElement("form");
  f.style.cssText = "display:flex;flex-direction:column;gap:10px;margin-top:10px;";
  const u = document.createElement("input"); u.placeholder = "用户名"; u.autocomplete = "username"; u.required = true;
  const p = document.createElement("input"); p.placeholder = isRegister ? "密码（至少 8 位）" : "密码"; p.type = "password";
  p.autocomplete = isRegister ? "new-password" : "current-password"; p.required = true;
  if (isRegister) p.minLength = 8;
  const b = document.createElement("button"); b.textContent = isRegister ? "注 册" : "登 录";
  if (isRegister) {
    const p2 = document.createElement("input"); p2.placeholder = "确认密码"; p2.type = "password";
    p2.autocomplete = "new-password"; p2.required = true;
    f.append(u, p, p2, b);   // 字段顺序：用户名 → 密码 → 确认密码 → 提交按钮
    f.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (p.value !== p2.value) { toast("两次输入的密码不一致", "err"); return; }
      const done = btnPending(b, "注册中…");
      try {
        await api("POST", "/auth/register", { username: u.value, password: p.value });
        const data = await api("POST", "/auth/login", { username: u.value, password: p.value });
        state.token = data.accessToken;
        state.whoami = u.value;
        localStorage.setItem("fl_token", state.token);
        localStorage.setItem("fl_whoami", u.value);
        toast("注册成功，已自动登录！", "ok");
        location.hash = "#books";
      } catch (err) { toast(err.message, "err"); }
      finally { done(); }
    });
  } else {
    f.append(u, p, b);
    f.addEventListener("submit", async (e) => {
      e.preventDefault();
      const done = btnPending(b, "登录中…");          // 对齐原型 loading 态
      try {
        const data = await api("POST", "/auth/login", { username: u.value, password: p.value });
        state.token = data.accessToken;
        state.whoami = u.value;
        localStorage.setItem("fl_token", state.token);
        localStorage.setItem("fl_whoami", u.value);
        toast("登录成功，欢迎回来！", "ok");
        location.hash = "#books";
      } catch (err) { toast(err.message, "err"); }
      finally { done(); }
    });
  }
  const toggle = document.createElement("a");
  toggle.href = "#";
  toggle.className = "muted";
  toggle.style.cssText = "display:block;text-align:center;margin-top:12px;font-size:13px;";
  toggle.textContent = isRegister ? "已有账号？返回登录" : "没有账号？注册新账号";
  toggle.addEventListener("click", (e) => {
    e.preventDefault();
    renderLogin(isRegister ? "login" : "register");
  });
  card.append(logo, h, sub, f, toggle);
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
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "📖";
    const p = document.createElement("p"); p.textContent = "还没有账本——在下面创建第一个吧。";
    empty.append(ic, p);
    card.append(empty);
  }
  for (const bk of books) {
    const a = document.createElement("a");
    a.href = "#b/" + bk.id;
    const name = document.createElement("span");
    name.textContent = bk.name + (bk.bookType === 2 ? "（家庭）" : "");
    const arrow = document.createElement("span"); arrow.className = "arrow"; arrow.textContent = "›";
    a.append(name, arrow);
    // 删除账本：仅 owner 可删（服务端 403 兜底），软删 + 审计；对齐原型确认对话框
    const row = document.createElement("div");
    row.style.cssText = "display:flex;gap:8px;align-items:center;";
    a.style.flex = "1";
    const del = document.createElement("button");
    del.textContent = "删除"; del.className = "danger";
    del.addEventListener("click", async () => {
      const okD = await confirmDialog({
        title: "删除账本",
        desc: "删除「" + bk.name + "」后，该账本及其中全部交易、账户、成员关系将不再可见（仅账本所有者可删除）。",
        confirmText: "删除", danger: true
      });
      if (!okD) return;
      const done = btnPending(del, "删除中…");
      try {
        await api("DELETE", "/books/" + bk.id);
        toast("账本已删除", "ok");
        await renderBooks();
      } catch (err) { toast(err.message, "err"); }
      finally { done(); }
    });
    row.append(a, del);
    card.append(row);
  }
  // 创建账本表单（POST /books {"name","bookType"}；1=个人 2=家庭）
  const form = document.createElement("form");
  form.style.cssText = "display:flex;gap:8px;margin-top:14px;flex-wrap:wrap;";
  const name = document.createElement("input");
  name.placeholder = "新账本名称"; name.required = true; name.maxLength = 40; name.style.flex = "1"; name.style.minWidth = "160px";
  const typeSel = document.createElement("select");
  [["1", "个人账本"], ["2", "家庭账本"]].forEach(([v, t]) => {
    const o = document.createElement("option"); o.value = v; o.textContent = t; typeSel.append(o);
  });
  const btn = document.createElement("button"); btn.textContent = "创建账本";
  form.append(name, typeSel, btn);
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const done = btnPending(btn, "创建中…");
    try {
      const bk = await api("POST", "/books", { name: name.value, bookType: Number(typeSel.value) });
      toast("账本已创建", "ok");
      location.hash = "#b/" + bk.id;
    } catch (err) { toast(err.message, "err"); }
    finally { done(); }
  });
  card.append(form);
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
    renderSkeleton(tableWrap);                      // 对齐原型：加载中骨架屏
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
    window.scrollTo({ top: 0, behavior: "smooth" }); // 翻页后回顶（原型分页行为）
  }

  // 骨架屏：数据未返回前占位，避免白屏跳变
  function renderSkeleton(wrap) {
    wrap.replaceChildren();
    const sk = document.createElement("div");
    sk.setAttribute("aria-busy", "true");
    for (let i = 0; i < 5; i++) {
      const bar = document.createElement("div"); bar.className = "skel";
      bar.style.width = (88 - i * 9) + "%";
      sk.append(bar);
    }
    wrap.append(sk);
  }

  function renderTable(data) {
    // 防御：老后端对空列表可能返回 null，统一兜底为空数组
    const items = data.items || [];
    tableWrap.replaceChildren();
    const table = document.createElement("table");
    const thead = document.createElement("thead");
    const trh = document.createElement("tr");
    ["", "日期", "类型", "账户", "分类", "备注/商户", "金额", ""].forEach((t, i) => {
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
    const tdTy = document.createElement("td");
    const badge = document.createElement("span");        // 对齐原型：类型胶囊徽标
    badge.className = "badge " + (tx.txType === 1 ? "income" : tx.txType === 2 ? "expense" : "");
    badge.textContent = TX_TYPE_NAME[tx.txType] || tx.txType;
    tdTy.append(badge);
    const tdA = document.createElement("td");
    tdA.textContent = tx.txType === 3 ? accountName(tx.accountId) + " → " + accountName(tx.toAccountId) : accountName(tx.accountId);
    const tdCat = document.createElement("td"); tdCat.textContent = tx.txType === 3 ? "—" : categoryName(tx.categoryId);
    const tdN = document.createElement("td");
    tdN.textContent = (tx.note || "") + (tx.merchant ? (tx.note ? " · " : "") + tx.merchant : "");
    tdN.className = "muted";
    const tdM = document.createElement("td"); tdM.className = "num " + (tx.txType === 1 ? "income" : tx.txType === 2 ? "expense" : "transfer");
    tdM.textContent = (tx.txType === 1 ? "+" : tx.txType === 2 ? "-" : "") + "¥" + cents2str(tx.amount);
    const tdOp = document.createElement("td");
    const editBtn = document.createElement("button"); editBtn.textContent = "编辑"; editBtn.className = "ghost";
    editBtn.addEventListener("click", () => openTxEditForm(bookId, tx, load));
    tdOp.append(editBtn);
    tr.append(tdC, tdT, tdTy, tdA, tdCat, tdN, tdM, tdOp);
    tbody.append(tr);
  }
  if (!items.length) {
    const tr = document.createElement("tr");
    const td = document.createElement("td"); td.colSpan = 8;
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "🧾";
    const p = document.createElement("p"); p.textContent = "没有符合条件的交易——换个筛选条件，或记一笔。";
    empty.append(ic, p); td.append(empty);
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
    // 对齐原型 AlertDialog：自定义确认对话框替代原生 confirm()
    const yes = await confirmDialog({
      title: "删除交易",
      desc: "将删除选中的 " + state.sel.size + " 条交易，删除后可在回收站恢复。",
      confirmText: "删除",
      danger: true,
    });
    if (!yes) return;
    const done = btnPending(delBtn, "删除中…");
    let okN = 0, failN = 0;
    try {
      for (const id of state.sel) {
        try {
          await api("DELETE", "/books/" + bookId + "/transactions/" + id + "?clientUuid=" + encodeURIComponent(uuid()));
          okN++;
        } catch (_) { failN++; }
      }
      toast("删除完成：成功 " + okN + "，失败 " + failN, failN > 0 ? "err" : "ok");
      state.filter.page = 1;
      await load();
    } finally { done(); }
  });
  addBtn.addEventListener("click", () => openTxForm(bookId, load));
  [kw, typeSel, accSel, catSel, dFrom, dTo].forEach((el) => {
    el.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); applyFilters(); } });
  });
  function applyFilters() {
    state.filter = { keyword: kw.value.trim(), type: Number(typeSel.value), accountId: Number(accSel.value), categoryId: Number(catSel.value), minDate: dFrom.value, maxDate: dTo.value, page: 1, count: state.filter.count };
    load().catch((e) => toast(e.message, "err"));
  }
  go.addEventListener("click", applyFilters);
  reset.addEventListener("click", () => { kw.value = ""; typeSel.value = "0"; accSel.value = "0"; catSel.value = "0"; dFrom.value = ""; dTo.value = ""; applyFilters(); });

  try { await load(); } catch (e) { toast(e.message, "err"); }
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
    if (cents === null || cents <= 0) { toast("金额格式不合法（最多两位小数）", "err"); return; }
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
      if (!body.toAccountId || body.toAccountId === body.accountId) { toast("转账需选择不同的两个账户", "err"); return; }
    } else if (!body.accountId) { toast("请选择账户", "err"); return; }
    try {
      const done = btnPending($("#fSubmit"), "保存中…");
      try {
        await api("POST", "/books/" + bookId + "/transactions", body);
        toast("已记账", "ok");
        dlg.close ? dlg.close() : dlg.removeAttribute("open");
        dlg.classList.add("hidden");
        onDone();
      } finally { done(); }
    } catch (err) { toast(err.message, "err"); }
  };
}

/* ---------- 编辑交易（WBS 5.9b：PUT 全量替换，类型不可改） ---------- */
function openTxEditForm(bookId, tx, onDone) {
  const dlg = $("#editDlg");
  $("#eType").textContent = TX_TYPE_NAME[tx.txType] || String(tx.txType);
  $("#eAmount").value = cents2str(tx.amount);
  $("#eDate").value = new Date(tx.txTime * 1000).toISOString().slice(0, 10);
  $("#eNote").value = tx.note || "";
  // 账户下拉：转账需避开"与对手方相同"，这里先全量填充，提交时校验
  fillSelect($("#eAccount"), state.accounts);
  fillSelect($("#eTo"), state.accounts);
  $("#eAccount").value = String(tx.accountId);
  if (tx.txType === 3) $("#eTo").value = String(tx.toAccountId);
  // 分类下拉：只列与交易类型匹配的 catType（服务端会校验一致）
  const catRow = $("#eCatRow"), toRow = $("#eToRow");
  if (tx.txType === 3) {
    catRow.classList.add("hidden");
    toRow.classList.remove("hidden");
  } else {
    toRow.classList.add("hidden");
    catRow.classList.remove("hidden");
    const cats = state.categories.filter((c) => c.catType === tx.txType);
    fillSelect($("#eCategory"), cats);
    $("#eCategory").value = String(tx.categoryId || "");
    if (!cats.find((c) => c.id === tx.categoryId)) $("#eCategory").value = String(cats.length ? cats[0].id : "0");
  }
  dlg.classList.remove("hidden");
  if (typeof dlg.showModal === "function") dlg.showModal(); else dlg.setAttribute("open", "");
  $("#eCancel").onclick = () => { dlg.close ? dlg.close() : dlg.removeAttribute("open"); dlg.classList.add("hidden"); };
  $("#txEditForm").onsubmit = async (e) => {
    e.preventDefault();
    const cents = str2cents($("#eAmount").value);
    if (cents === null || cents <= 0) { toast("金额格式不合法（最多两位小数）", "err"); return; }
    const body = {
      clientUuid: uuid(),
      txType: tx.txType,
      accountId: Number($("#eAccount").value),
      amount: cents,
      categoryId: tx.txType === 3 ? 0 : Number($("#eCategory").value),
      paymentChannel: tx.paymentChannel || 0,
      txTime: dateStartUnix($("#eDate").value, false),
      timezoneOffset: -new Date().getTimezoneOffset(),
      comment: $("#eNote").value,
      baseVersion: tx.syncVersion || 0,
    };
    if (tx.txType === 3) {
      body.toAccountId = Number($("#eTo").value);
      if (!body.toAccountId || body.toAccountId === body.accountId) { toast("转账需选择不同的两个账户", "err"); return; }
    } else if (!body.accountId || !body.categoryId) { toast("请选择账户与分类", "err"); return; }
    const btn = $("#eSubmit");
    const done = btnPending(btn, "保存中…");
    try {
      await api("PUT", "/books/" + bookId + "/transactions/" + tx.id, body);
      toast("已保存修改", "ok");
      dlg.close ? dlg.close() : dlg.removeAttribute("open");
      dlg.classList.add("hidden");
      onDone();
    } catch (err) {
      toast(err.message + (String(err.message).indexOf("并发") >= 0 ? "（请刷新列表后重试）" : ""), "err");
    } finally { done(); }
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
  const h = document.createElement("h2"); h.style.display = "flex"; h.style.alignItems = "center"; h.style.justifyContent = "space-between";
  const ht = document.createElement("span"); ht.textContent = "账户对账";
  const addBtn = document.createElement("button"); addBtn.textContent = "＋ 新增账户";
  addBtn.onclick = () => openAcctForm(bookId, null);
  h.append(ht, addBtn);
  const p = document.createElement("p"); p.className = "muted"; p.textContent = "各账户当前余额（负债账户为欠款）。";
  card.append(h, p);
  const table = document.createElement("table");
  const thead = document.createElement("thead");
  const trh = document.createElement("tr");
  ["账户", "类型", "余额", "操作"].forEach((t, i) => { const th = document.createElement("th"); th.textContent = t; if (i === 2) th.className = "num"; trh.append(th); });
  thead.append(trh); table.append(thead);
  const tbody = document.createElement("tbody");
  for (const a of state.accounts) {
    const tr = document.createElement("tr");
    const tdN = document.createElement("td"); tdN.textContent = a.name;
    const tdC = document.createElement("td"); tdC.textContent = ACC_CAT_NAME[a.category] || ("类别 " + a.category); tdC.className = "muted";
    const tdB = document.createElement("td"); tdB.className = "num " + (a.balance < 0 ? "balance-neg" : "balance-pos");
    tdB.textContent = (a.currency || "CNY") === "CNY" ? "¥" + cents2str(a.balance) : a.currency + " " + cents2str(a.balance);
    const tdOp = document.createElement("td");
    const ra = document.createElement("div"); ra.className = "row-actions";
    const edit = document.createElement("button"); edit.className = "ghost"; edit.textContent = "编辑";
    edit.onclick = () => openAcctForm(bookId, a);
    const del = document.createElement("button"); del.className = "danger"; del.textContent = "删除";
    del.onclick = async () => {
      const ok = await confirmDialog({
        title: "删除账户",
        desc: "删除「" + a.name + "」后，该账户及其余额将不再可见（仅账本所有者可删除）。",
        confirmText: "删除", danger: true
      });
      if (!ok) return;
      const done = btnPending(del, "删除中…");
      try { await api("DELETE", "/books/" + bookId + "/accounts/" + a.id); toast("账户已删除", "ok"); await renderAccounts(bookId); }
      catch (e) { toast(e.message, "err"); } finally { done(); }
    };
    ra.append(edit, del); tdOp.append(ra);
    tr.append(tdN, tdC, tdB, tdOp);
    // 投资账户（类别 5）可点入持仓明细（对齐原型：投资账户详情页）
    if (a.category === 5) {
      tr.style.cursor = "pointer";
      tr.title = "查看持仓明细";
      tr.addEventListener("click", (e) => {
        if (e.target.closest(".row-actions")) return;
        location.hash = "#b/" + bookId + "/account/" + a.id;
      });
    }
    tbody.append(tr);
  }
  table.append(tbody);
  if (!state.accounts.length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "💳";
    const p2 = document.createElement("p"); p2.textContent = "暂无账户——点击右上角「新增账户」添加。";
    empty.append(ic, p2); card.append(empty);
  }
  card.append(table);
  view.append(card);
}

/* ---------- 账户新增/编辑对话框 ---------- */
function openAcctForm(bookId, acc) {
  const dlg = $("#acctDlg");
  const editing = !!acc;
  $("#acctFormTitle").textContent = editing ? "编辑账户" : "新增账户";
  $("#acctName").value = acc ? acc.name : "";
  fillSelect($("#acctCat"), Object.entries(ACC_CAT_NAME).map(([k, v]) => ({ id: Number(k), name: v })), null);
  $("#acctCat").value = acc ? String(acc.category) : "1";
  $("#acctCat").disabled = editing;            // 编辑不支持改类型（后端仅改名称）
  $("#acctCur").value = acc ? (acc.currency || "CNY") : "CNY";
  $("#acctCur").disabled = editing;
  $("#acctErr").textContent = "";
  dlg.classList.remove("hidden");
  if (typeof dlg.showModal === "function") dlg.showModal(); else dlg.setAttribute("open", "");
  $("#acctCancel").onclick = () => { dlg.close ? dlg.close() : dlg.removeAttribute("open"); dlg.classList.add("hidden"); };
  $("#acctForm").onsubmit = async (e) => {
    e.preventDefault();
    const name = $("#acctName").value.trim();
    if (!name) { $("#acctErr").textContent = "请输入账户名称"; return; }
    let body;
    if (editing) {
      body = { name };
    } else {
      body = { name, category: Number($("#acctCat").value), currency: $("#acctCur").value.trim() || "CNY" };
    }
    const done = btnPending($("#acctSubmit"), "保存中…");
    try {
      if (editing) await api("PUT", "/books/" + bookId + "/accounts/" + acc.id, body);
      else await api("POST", "/books/" + bookId + "/accounts", body);
      toast(editing ? "账户已更新" : "账户已创建", "ok");
      dlg.close ? dlg.close() : dlg.removeAttribute("open"); dlg.classList.add("hidden");
      await renderAccounts(bookId);
    } catch (err) { $("#acctErr").textContent = err.message; } finally { done(); }
  };
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
      const done = btnPending(btn, "恢复中…");
      try {
        await api("POST", "/books/" + bookId + "/transactions/" + tx.id + "/restore?clientUuid=" + encodeURIComponent(uuid()));
        toast("已恢复", "ok");
        tr.classList.add("fade-out");                    // 行淡出后再移除（原型过渡细节）
        setTimeout(() => tr.remove(), 190);
      } catch (e) { toast(e.message, "err"); done(); }
    });
    tdA.append(btn); tr.append(tdA);
    tbody.append(tr);
  }
  if (!(data.items || []).length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "🗑️";
    const p = document.createElement("p"); p.textContent = "回收站为空——删除的交易会在这里保留，可随时恢复。";
    empty.append(ic, p); card.append(empty);
  }
  table.append(tbody); card.append(table);
  view.append(card);
}

/* ---------- 收支报表（对齐原型 /report：收支趋势 + 本月支出分类占比 + 本月/上月切换） ---------- */
let reportMonthOffset = 0; // 0=本月，1=上月（原型口径）

async function renderReport(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navReport").classList.add("active");
  view.replaceChildren();

  // 标题 + 月份切换（本月/上月）
  const head = document.createElement("div");
  head.className = "card";
  const h = document.createElement("h2"); h.textContent = "收支报表";
  const nav = document.createElement("div"); nav.style.cssText = "display:flex;gap:8px;align-items:center;";
  const mkMonthBtn = (label, off) => {
    const b = document.createElement("button");
    b.textContent = label;
    b.className = reportMonthOffset === off ? "" : "ghost";
    b.addEventListener("click", () => { reportMonthOffset = off; renderReport(bookId); });
    return b;
  };
  nav.append(mkMonthBtn("本月", 0), mkMonthBtn("上月", 1));
  head.append(h, nav);
  view.append(head);

  const month = monthShift(reportMonthOffset);

  // 卡 1：收支趋势（近 6 月，柱状）
  const trend = await api("GET", "/books/" + bookId + "/reports/monthly?months=6");
  const trendCard = document.createElement("div"); trendCard.className = "card";
  const t2 = document.createElement("h2"); t2.textContent = "收支趋势"; t2.className = "";
  const sub = document.createElement("span"); sub.className = "muted"; sub.textContent = " 近 6 个月（转账不计收支）";
  t2.append(sub);
  trendCard.append(t2);
  const bars = document.createElement("div");
  bars.style.cssText = "display:flex;align-items:flex-end;gap:14px;height:140px;padding:8px 4px 0;";
  const items = trend.items || [];
  let maxV = 1;
  for (const p of items) maxV = Math.max(maxV, p.income, p.expense);
  for (const p of items) {
    const col = document.createElement("div");
    col.style.cssText = "flex:1;display:flex;flex-direction:column;align-items:center;gap:4px;height:100%;justify-content:flex-end;";
    const pair = document.createElement("div");
    pair.style.cssText = "display:flex;gap:3px;align-items:flex-end;height:100%;width:100%;justify-content:center;";
    const mkBar = (v, color, title) => {
      const bar = document.createElement("div");
      bar.style.cssText = "width:16px;border-radius:4px 4px 0 0;background:" + color +
        ";height:" + Math.round(v / maxV * 100) + "%;min-height:2px;";
      bar.title = title;
      return bar;
    };
    pair.append(
      mkBar(p.income, "var(--income)", p.month + " 收入 ¥" + cents2str(p.income)),
      mkBar(p.expense, "var(--expense)", p.month + " 支出 ¥" + cents2str(p.expense))
    );
    const x = document.createElement("span"); x.className = "muted"; x.style.fontSize = "11px";
    x.textContent = p.month.slice(5);
    col.append(pair, x);
    bars.append(col);
  }
  trendCard.append(bars);
  const legend = document.createElement("div"); legend.className = "muted";
  legend.style.cssText = "display:flex;gap:16px;margin-top:8px;font-size:12px;";
  legend.append(mkLegend("var(--income)", "收入"), mkLegend("var(--expense)", "支出"));
  trendCard.append(legend);
  view.append(trendCard);

  // 卡 2：本月支出分类占比（排行）
  const share = await api("GET", "/books/" + bookId + "/reports/categories?type=2&month=" + encodeURIComponent(month));
  const shareCard = document.createElement("div"); shareCard.className = "card";
  const t3 = document.createElement("h2");
  t3.textContent = (reportMonthOffset === 0 ? "本月" : "上月") + "支出分类占比";
  shareCard.append(t3);
  const rows = share.items || [];
  if (!rows.length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "📊";
    const p = document.createElement("p"); p.textContent = "该月暂无支出记录。";
    empty.append(ic, p); shareCard.append(empty);
  } else {
    let total = 0;
    for (const r of rows) total += r.total;
    const table = document.createElement("table");
    const thead = document.createElement("thead"); const trh = document.createElement("tr");
    ["分类排行", "笔数", "金额", "占比"].forEach((t, i) => {
      const th = document.createElement("th"); th.textContent = t;
      if (i >= 1) th.className = "num";
      trh.append(th);
    });
    thead.append(trh); table.append(thead);
    const tbody = document.createElement("tbody");
    for (const r of rows) {
      const tr = document.createElement("tr");
      const tdN = document.createElement("td"); tdN.textContent = r.name;
      const tdC = document.createElement("td"); tdC.className = "num muted"; tdC.textContent = String(r.count);
      const tdA = document.createElement("td"); tdA.className = "num"; tdA.textContent = "¥" + cents2str(r.total);
      const tdP = document.createElement("td"); tdP.className = "num muted";
      tdP.textContent = (total > 0 ? Math.round(r.total / total * 100) : 0) + "%";
      tr.append(tdN, tdC, tdA, tdP);
      tbody.append(tr);
    }
    table.append(tbody); shareCard.append(table);
  }
  view.append(shareCard);
}

function mkLegend(color, label) {
  const s = document.createElement("span");
  const dot = document.createElement("span");
  dot.style.cssText = "display:inline-block;width:10px;height:10px;border-radius:3px;background:" + color + ";margin-right:4px;";
  s.append(dot, document.createTextNode(label));
  return s;
}

// monthShift(0)=本月 "YYYY-MM"，(1)=上月（与服务端 CST 口径对齐；本地时区即用户认知口径）
function monthShift(offset) {
  const d = new Date();
  d.setDate(1);
  d.setMonth(d.getMonth() - offset);
  const p = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + p(d.getMonth() + 1);
}

/* ---------- 数据导出（对齐原型 /export：时间范围 + 收支类型 + 格式卡片 + 导出字段勾选） ---------- */
const EXPORT_FIELDS = [
  ["time", "时间"], ["type", "类型"], ["amount", "金额"], ["category", "分类"],
  ["account", "账户"], ["toAccount", "对方账户"], ["channel", "渠道"],
  ["merchant", "商户"], ["note", "备注"]
];

async function renderExport(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navExport").classList.add("active");
  view.replaceChildren();

  const card = document.createElement("div"); card.className = "card";
  const h = document.createElement("h2"); h.textContent = "数据导出";
  card.append(h);

  // 时间范围（快速预设 + 自定义）
  const sec1 = document.createElement("p"); sec1.className = "muted"; sec1.style.fontWeight = "500";
  sec1.textContent = "时间范围";
  const presetRow = document.createElement("div"); presetRow.className = "export-range";
  const presetBtns = [];
  [["this", "本月"], ["last", "上月"], ["quarter", "本季度"], ["custom", "自定义"]].forEach(([k, t]) => {
    const b = document.createElement("button"); b.textContent = t; b.dataset.kind = k;
    b.className = "active"; presetBtns.push(b); presetRow.append(b);
  });
  const row1 = document.createElement("div"); row1.style.cssText = "display:flex;gap:10px;flex-wrap:wrap;margin:6px 0 12px;";
  const dFrom = document.createElement("input"); dFrom.type = "date"; dFrom.id = "xFrom";
  const dTo = document.createElement("input"); dTo.type = "date"; dTo.id = "xTo";
  row1.append(wrapField("开始日期", dFrom), wrapField("结束日期", dTo));

  const p2 = (n) => String(n).padStart(2, "0");
  const isoOf = (d) => d.getFullYear() + "-" + p2(d.getMonth() + 1) + "-" + p2(d.getDate());
  function setRange(kind) {
    const now = new Date();
    let from, to;
    if (kind === "this") { from = new Date(now.getFullYear(), now.getMonth(), 1); to = now; }
    else if (kind === "last") { from = new Date(now.getFullYear(), now.getMonth() - 1, 1); to = new Date(now.getFullYear(), now.getMonth(), 0); }
    else if (kind === "quarter") { const q = Math.floor(now.getMonth() / 3) * 3; from = new Date(now.getFullYear(), q, 1); to = now; }
    if (kind !== "custom") { dFrom.value = isoOf(from); dTo.value = isoOf(to); }
    presetBtns.forEach((b) => b.classList.toggle("active", b.dataset.kind === kind));
  }
  presetBtns.forEach((b) => b.onclick = () => setRange(b.dataset.kind));
  [dFrom, dTo].forEach((el) => el.addEventListener("change", () => {
    presetBtns.forEach((b) => b.classList.toggle("active", b.dataset.kind === "custom"));
  }));
  setRange("this");

  // 收支类型
  const sec2 = document.createElement("p"); sec2.className = "muted"; sec2.style.fontWeight = "500";
  sec2.textContent = "收支类型";
  const typeSel = document.createElement("select");
  [["0", "全部"], ["1", "收入"], ["2", "支出"], ["3", "转账"]].forEach(([v, t]) => {
    const o = document.createElement("option"); o.value = v; o.textContent = t; typeSel.append(o);
  });
  const row2 = document.createElement("div"); row2.style.margin = "6px 0 12px";
  row2.append(wrapField("类型", typeSel));

  // 导出格式（两卡片，对齐原型 CSV / JSON）
  const sec3 = document.createElement("p"); sec3.className = "muted"; sec3.style.fontWeight = "500";
  sec3.textContent = "导出格式";
  const fmtRow = document.createElement("div"); fmtRow.style.cssText = "display:flex;gap:12px;margin:6px 0 12px;flex-wrap:wrap;";
  const fmtCards = {};
  [["csv", "CSV", "带 UTF-8 BOM，Excel 可直接打开"], ["json", "JSON", "原始结构化数据，适合备份"]].forEach(([v, name, desc]) => {
    const box = document.createElement("label");
    box.style.cssText = "flex:1;min-width:180px;border:1px solid var(--line);border-radius:12px;padding:12px;display:flex;gap:8px;align-items:flex-start;cursor:pointer;";
    const radio = document.createElement("input"); radio.type = "radio"; radio.name = "xFormat"; radio.value = v;
    if (v === "csv") radio.checked = true;
    const txt = document.createElement("span");
    const b1 = document.createElement("b"); b1.textContent = name;
    const p1 = document.createElement("p"); p1.className = "muted"; p1.style.margin = "2px 0 0"; p1.textContent = desc;
    txt.append(b1, p1);
    box.append(radio, txt);
    fmtRow.append(box);
    fmtCards[v] = box;
  });

  // 导出字段（勾选，默认全选）
  const sec4 = document.createElement("p"); sec4.className = "muted"; sec4.style.fontWeight = "500";
  sec4.textContent = "导出字段";
  const fieldRow = document.createElement("div");
  fieldRow.style.cssText = "display:flex;gap:10px 16px;flex-wrap:wrap;margin:6px 0 16px;";
  const boxes = [];
  for (const [key, label] of EXPORT_FIELDS) {
    const lab = document.createElement("label");
    lab.style.cssText = "display:flex;align-items:center;gap:5px;cursor:pointer;";
    const cb = document.createElement("input"); cb.type = "checkbox"; cb.value = key; cb.checked = true;
    lab.append(cb, document.createTextNode(label));
    fieldRow.append(lab);
    boxes.push(cb);
  }

  // 数据预览（共 N 条）
  const previewEl = document.createElement("div"); previewEl.className = "export-preview"; previewEl.textContent = "点击「预览」查看本次导出的记录条数。";
  const btnPreview = document.createElement("button"); btnPreview.className = "ghost"; btnPreview.textContent = "预览";

  async function updatePreview() {
    const q = new URLSearchParams();
    if (typeSel.value !== "0") q.set("type", typeSel.value);
    const min = dateStartUnix(dFrom.value, false), max = dateStartUnix(dTo.value, true);
    if (min) q.set("minTime", String(min));
    if (max) q.set("maxTime", String(max));
    q.set("count", "1"); q.set("page", "1");
    const done = btnPending(btnPreview, "预览中…");
    try {
      const d = await api("GET", "/books/" + bookId + "/transactions?" + q.toString());
      const n = (d && d.total) || 0;
      previewEl.textContent = n > 0 ? "共 " + n + " 条记录将导出。" : "没有可导出的数据（该范围内无交易）。";
    } catch (e) { previewEl.textContent = "预览失败：" + e.message; }
    finally { done(); }
  }

  // 动作按钮（对齐原型："导出 CSV 文件" / "导出 JSON 文件"）
  const actions = document.createElement("div"); actions.style.cssText = "display:flex;gap:10px;flex-wrap:wrap;";
  const btnCsv = document.createElement("button"); btnCsv.textContent = "导出 CSV 文件";
  const btnJson = document.createElement("button"); btnJson.textContent = "导出 JSON 文件"; btnJson.className = "ghost";
  actions.append(btnCsv, btnJson, btnPreview);
  card.append(sec1, presetRow, row1, sec2, row2, sec3, fmtRow, sec4, fieldRow, actions, previewEl);

  const note = document.createElement("p"); note.className = "export-note";
  note.textContent = "CSV 含 UTF-8 BOM，Excel 可直接打开（兼容）；JSON 为原始结构化数据，适合备份或开发 API 读取。";
  card.append(note);
  view.append(card);

  const doExport = async (format, btn) => {
    const fields = boxes.filter((b) => b.checked).map((b) => b.value);
    if (!fields.length) { toast("请至少勾选一个导出字段", "err"); return; }
    const q = new URLSearchParams({ format, fields: fields.join(",") });
    if (typeSel.value !== "0") q.set("type", typeSel.value);
    const min = dateStartUnix(dFrom.value, false), max = dateStartUnix(dTo.value, true);
    if (min) q.set("minTime", String(min));
    if (max) q.set("maxTime", String(max));
    const done = btnPending(btn, "导出中…");
    try {
      const resp = await fetch(API + "/books/" + bookId + "/export?" + q.toString(), {
        headers: { Authorization: "Bearer " + state.token }
      });
      if (!resp.ok) {
        let msg = "导出失败（HTTP " + resp.status + "）";
        try { const j = await resp.json(); if (j && j.msg) msg = j.msg; } catch (_) { /* 文件响应无 JSON */ }
        throw new Error(msg);
      }
      const count = Number(resp.headers.get("X-Export-Count") || "0");
      if (count === 0) { toast("没有可导出的数据", "info"); return; }
      if (resp.headers.get("X-Export-Truncated") === "1") {
        toast("结果超过 10000 条，已截断", "info");
      }
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      const p = (n) => String(n).padStart(2, "0");
      const now = new Date();
      a.href = url;
      a.download = "FastLedger_导出_" + now.getFullYear() + p(now.getMonth() + 1) + p(now.getDate()) + "." + format;
      document.body.append(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      toast("已导出 " + count + " 条记录", "ok");
    } catch (err) {
      toast(err.message, "err");
    } finally { done(); }
  };
  btnCsv.addEventListener("click", () => doExport("csv", btnCsv));
  btnJson.addEventListener("click", () => doExport("json", btnJson));
  btnPreview.addEventListener("click", updatePreview);
  updatePreview();
}

function wrapField(label, control) {
  const lab = document.createElement("label");
  lab.style.cssText = "display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted-fg);";
  lab.append(document.createTextNode(label), control);
  return lab;
}

/* ---------- 预算管理（对齐原型 /budget：本月总预算 + 分类预算信封，三态进度） ---------- */
const BUDGET_STATUS_NAME = { normal: "正常", near: "接近", over: "超支" };
let editingBudgetId = 0; // 0 = 新增

async function renderBudget(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navBudget").classList.add("active");
  view.replaceChildren();

  const head = document.createElement("div"); head.className = "card";
  const h = document.createElement("h2"); h.textContent = "预算管理";
  const tip = document.createElement("p"); tip.className = "muted"; tip.style.margin = "0";
  tip.textContent = "预算周期为自然月，每月 1 日自动重置进度。";
  const addBtn = document.createElement("button");
  addBtn.textContent = "新增预算"; addBtn.style.cssText = "margin-left:auto;";
  addBtn.addEventListener("click", () => openBudgetDialog(bookId, null));
  const headRow = document.createElement("div");
  headRow.style.cssText = "display:flex;align-items:center;gap:10px;flex-wrap:wrap;";
  headRow.append(h, tip, addBtn);
  head.append(headRow);
  view.append(head);

  const data = await api("GET", "/books/" + bookId + "/budgets");
  const items = data.items || [];
  const card = document.createElement("div"); card.className = "card";
  if (!items.length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "✉️";
    const p = document.createElement("p"); p.textContent = "还没有预算——先设一个本月总预算，或按分类设信封。";
    empty.append(ic, p); card.append(empty);
  }
  for (const it of items) {
    const row = document.createElement("div"); row.className = "budget-row";
    const headRow = document.createElement("div"); headRow.className = "budget-head";
    const name = document.createElement("span"); name.style.fontWeight = "500";
    name.textContent = it.budgetType === 2 ? "本月总预算" : (it.categoryName || ("分类 " + it.categoryId));
    const badge = document.createElement("span"); badge.className = "badge " + it.status;
    badge.style.cssText = it.status === "over" ? "background:color-mix(in srgb, var(--danger) 10%, #fff);color:var(--danger)"
      : it.status === "near" ? "background:color-mix(in srgb, var(--warning) 12%, #fff);color:var(--warning)" : "";
    badge.textContent = BUDGET_STATUS_NAME[it.status] || it.status;
    const amt = document.createElement("span"); amt.className = "amt";
    amt.textContent = "已用 ¥" + cents2str(it.spent) + " / ¥" + cents2str(it.amount) + " · 剩余 ¥" + cents2str(Math.max(0, it.amount - it.spent));
    headRow.append(name, badge, amt);
    const bar = document.createElement("div"); bar.className = "progress";
    const fill = document.createElement("i");
    fill.style.width = Math.min(100, it.percent) + "%";
    if (it.status === "over") fill.className = "over";
    else if (it.status === "near") fill.className = "warn";
    bar.append(fill);
    const ops = document.createElement("div"); ops.style.cssText = "display:flex;gap:8px;margin-top:8px;justify-content:flex-end;";
    const edit = document.createElement("button"); edit.textContent = "编辑"; edit.className = "ghost";
    edit.style.padding = "4px 12px";
    edit.addEventListener("click", () => openBudgetDialog(bookId, it));
    const del = document.createElement("button"); del.textContent = "删除"; del.className = "danger";
    del.style.padding = "4px 12px";
    del.addEventListener("click", async () => {
      const okD = await confirmDialog({
        title: "删除预算", desc: "删除「" + (it.budgetType === 2 ? "本月总预算" : (it.categoryName || "分类预算")) + "」？删除后本月进度不再统计。",
        confirmText: "删除", danger: true
      });
      if (!okD) return;
      try {
        await api("DELETE", "/books/" + bookId + "/budgets/" + it.id);
        toast("预算已删除", "ok");
        renderBudget(bookId);
      } catch (err) { toast(err.message, "err"); }
    });
    ops.append(edit, del);
    row.append(headRow, bar, ops);
    card.append(row);
  }
  view.append(card);
}

// 预算对话框：新增（edit=null）或编辑（edit=预算视图）
function openBudgetDialog(bookId, edit) {
  const dlg = $("#budgetDlg");
  editingBudgetId = edit ? edit.id : 0;
  $("#bFormTitle").textContent = edit ? "编辑预算" : "新增预算";
  $("#bErr").textContent = "";
  $("#bAmount").value = edit ? cents2str(edit.amount) : "";
  $("#bRollover").checked = !!(edit && edit.rollover);
  // 类型：编辑时只读（类型/挂靠不可改，与"删了重建"审计语义一致）
  const typeSel = $("#bType");
  const catRow = $("#bCatRow");
  typeSel.value = edit ? String(edit.budgetType) : "1";
  typeSel.disabled = !!edit;
  catRow.classList.toggle("hidden", !!edit && edit.budgetType === 2);
  if (!edit) catRow.classList.remove("hidden");
  // 分类下拉（支出分类；编辑总预算时不需要）
  const sel = $("#bCategory");
  sel.replaceChildren();
  for (const c of state.categories) {
    if (c.catType !== 2) continue; // 预算只挂支出分类（服务端同样校验）
    const o = document.createElement("option");
    o.value = String(c.id);
    o.textContent = c.parentId ? "└ " + c.name : c.name;
    sel.append(o);
  }
  if (edit && edit.categoryId) sel.value = String(edit.categoryId);

  dlg.classList.remove("hidden");
  if (typeof dlg.showModal === "function") dlg.showModal(); else dlg.setAttribute("open", "");
  $("#bCancel").onclick = () => closeBudgetDlg();
  $("#budgetForm").onsubmit = async (ev) => {
    ev.preventDefault();
    const cents = str2cents($("#bAmount").value);
    if (!cents || cents <= 0) { $("#bErr").textContent = "请输入大于 0 的金额"; return; }
    const body = { amount: cents, rollover: $("#bRollover").checked };
    const done = btnPending($("#bSubmit"), "保存中…");
    try {
      if (editingBudgetId) {
        body.name = "";
        await api("PUT", "/books/" + bookId + "/budgets/" + editingBudgetId, body);
        toast("预算已更新", "ok");
      } else {
        body.budgetType = Number(typeSel.value);
        body.categoryId = body.budgetType === 2 ? 0 : Number(sel.value);
        body.name = body.budgetType === 2 ? "" : "分类预算";
        await api("POST", "/books/" + bookId + "/budgets", body);
        toast("预算已创建", "ok");
      }
      closeBudgetDlg();
      renderBudget(bookId);
    } catch (err) {
      $("#bErr").textContent = err.message;
      toast(err.message, "err");
    } finally { done(); }
  };
}
function closeBudgetDlg() {
  const dlg = $("#budgetDlg");
  dlg.close ? dlg.close() : dlg.removeAttribute("open");
  dlg.classList.add("hidden");
}

/* ---------- 投资资产（对齐原型：资产总览/家庭净资产/总资产/总负债 + 持仓） ---------- */
const INSTR_TYPE_NAME = { 1: "股票", 2: "基金", 3: "债券", 4: "贵金属", 5: "外汇", 6: "其他" };

async function renderInvest(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navInvest").classList.add("active");
  view.replaceChildren();

  // 资产总览（原型文案：资产总览/总资产/总负债/家庭净资产）
  const nw = await api("GET", "/books/" + bookId + "/net-worth");
  const ov = document.createElement("div");
  ov.className = "card";
  const oh = document.createElement("h2"); oh.textContent = "资产总览";
  ov.append(oh);
  const grid = document.createElement("div");
  grid.style.cssText = "display:grid;grid-template-columns:repeat(4,1fr);gap:12px;";
  const mkCell = (label, val, color) => {
    const c = document.createElement("div");
    const l = document.createElement("div"); l.className = "muted"; l.style.fontSize = "13px"; l.textContent = label;
    const n = document.createElement("div"); n.style.cssText = "font-size:20px;font-weight:600;" + (color ? "color:" + color : "");
    n.textContent = "¥" + cents2str(val);
    c.append(l, n);
    return c;
  };
  grid.append(
    mkCell("总资产", nw.totalAssets),
    mkCell("总负债", nw.totalLiabilities, "var(--expense)"),
    mkCell("持仓市值", nw.holdingsValue),
    mkCell("家庭净资产", nw.netWorth, nw.netWorth >= 0 ? "var(--primary)" : "var(--expense)")
  );
  ov.append(grid);
  const note = document.createElement("p"); note.className = "muted"; note.style.margin = "10px 0 0";
  note.textContent = "净资产 = 总资产（含持仓市值）− 总负债；负债账户余额为正表示欠款。";
  ov.append(note);
  view.append(ov);

  // 持仓列表
  const hdata = await api("GET", "/books/" + bookId + "/holdings");
  const holdings = hdata.items || [];
  const hCard = document.createElement("div"); hCard.className = "card";
  const hRow = document.createElement("div"); hRow.style.cssText = "display:flex;align-items:center;gap:10px;";
  const hh = document.createElement("h2"); hh.style.margin = "0"; hh.textContent = "持仓";
  const addHolding = document.createElement("button"); addHolding.textContent = "添加持仓"; addHolding.style.marginLeft = "auto";
  addHolding.addEventListener("click", () => openHoldingDialog(bookId));
  hRow.append(hh, addHolding);
  hCard.append(hRow);
  if (!holdings.length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "📈";
    const p = document.createElement("p"); p.textContent = "暂无持仓——先在下方标的库添加标的，再录入持仓。";
    empty.append(ic, p); hCard.append(empty);
  } else {
    const table = document.createElement("table");
    const thead = document.createElement("thead"); const trh = document.createElement("tr");
    ["标的", "数量", "成本", "现价", "市值", "浮动盈亏", ""].forEach((t, i) => {
      const th = document.createElement("th"); th.textContent = t;
      if (i >= 1) th.className = "num";
      trh.append(th);
    });
    thead.append(trh); table.append(thead);
    const tbody = document.createElement("tbody");
    for (const it of holdings) {
      const tr = document.createElement("tr");
      const tdN = document.createElement("td");
      tdN.textContent = (it.instrumentName || "标的 " + it.instrumentId) + " · " + (INSTR_TYPE_NAME[it.instrumentType] || "");
      const mk = (t, cls) => { const td = document.createElement("td"); td.textContent = t; if (cls) td.className = cls; return td; };
      tr.append(
        tdN,
        mk(it.quantity, "num"),
        mk("¥" + cents2str(it.costAmount), "num"),
        mk(it.currentPrice, "num muted"),
        mk("¥" + cents2str(it.marketValue), "num"),
        mk((it.profitLoss >= 0 ? "+" : "") + "¥" + cents2str(Math.abs(it.profitLoss)), "num " + (it.profitLoss >= 0 ? "income" : "expense"))
      );
      const tdOps = document.createElement("td"); tdOps.className = "row-actions";
      const bEdit = document.createElement("button"); bEdit.textContent = "编辑"; bEdit.className = "ghost";
      bEdit.addEventListener("click", () => openHoldingDialog(bookId, it));
      const bPrice = document.createElement("button"); bPrice.textContent = "录价"; bPrice.className = "ghost";
      bPrice.addEventListener("click", () => openPriceDialog(bookId, it.instrumentId, it.instrumentName || "", it.currentPrice));
      const bDel = document.createElement("button"); bDel.textContent = "删除"; bDel.className = "danger";
      bDel.addEventListener("click", async () => {
        const okD = await confirmDialog({ title: "删除持仓", desc: "删除该持仓后其市值不再计入净资产。", confirmText: "删除", danger: true });
        if (!okD) return;
        try {
          await api("DELETE", "/books/" + bookId + "/holdings/" + it.id);
          toast("持仓已删除", "ok");
          renderInvest(bookId);
        } catch (err) { toast(err.message, "err"); }
      });
      tdOps.append(bEdit, bPrice, bDel);
      tr.append(tdOps);
      tbody.append(tr);
    }
    table.append(tbody); hCard.append(table);
  }
  view.append(hCard);

  // 标的库
  const idata = await api("GET", "/books/" + bookId + "/instruments");
  const instruments = idata.items || [];
  const iCard = document.createElement("div"); iCard.className = "card";
  const iRow = document.createElement("div"); iRow.style.cssText = "display:flex;align-items:center;gap:10px;";
  const ih = document.createElement("h2"); ih.style.margin = "0"; ih.textContent = "标的库";
  const addInstr = document.createElement("button"); addInstr.textContent = "新增标的"; addInstr.style.marginLeft = "auto";
  addInstr.addEventListener("click", () => openInstrumentDialog(bookId));
  iRow.append(ih, addInstr);
  iCard.append(iRow);
  if (!instruments.length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "🏷️";
    const p = document.createElement("p"); p.textContent = "标的库为空（MVP 手动录入行情，不做自动行情）。";
    empty.append(ic, p); iCard.append(empty);
  } else {
    const table = document.createElement("table");
    const thead = document.createElement("thead"); const trh = document.createElement("tr");
    ["名称", "类型", "市场", "代码", ""].forEach((t, i) => { const th = document.createElement("th"); th.textContent = t; if (i >= 2) th.className = "num"; trh.append(th); });
    thead.append(trh); table.append(thead);
    const tbody = document.createElement("tbody");
    for (const it of instruments) {
      const tr = document.createElement("tr");
      const mk = (t, cls) => { const td = document.createElement("td"); td.textContent = t; if (cls) td.className = cls; return td; };
      tr.append(
        mk(it.name),
        mk(INSTR_TYPE_NAME[it.type] || it.type, "muted"),
        mk(it.market || "—", "muted"),
        mk(it.code || "—", "num")
      );
      const tdOps = document.createElement("td"); tdOps.className = "row-actions";
      const bEdit = document.createElement("button"); bEdit.textContent = "编辑"; bEdit.className = "ghost";
      bEdit.addEventListener("click", () => openInstrumentDialog(bookId, it));
      const bDel = document.createElement("button"); bDel.textContent = "删除"; bDel.className = "danger";
      bDel.addEventListener("click", async () => {
        const okD = await confirmDialog({ title: "删除标的", desc: "仍有持仓引用的标的无法删除。", confirmText: "删除", danger: true });
        if (!okD) return;
        try {
          await api("DELETE", "/books/" + bookId + "/instruments/" + it.id);
          toast("标的已删除", "ok");
          renderInvest(bookId);
        } catch (err) { toast(err.message, "err"); }
      });
      tdOps.append(bEdit, bDel);
      tr.append(tdOps);
      tbody.append(tr);
    }
    table.append(tbody); iCard.append(table);
  }
  view.append(iCard);
}

// 通用投资对话框字段构造：fields = [{label, control}]
function openInvestDialog(title, fields, onSubmit) {
  const dlg = $("#investDlg");
  $("#iFormTitle").textContent = title;
  $("#iErr").textContent = "";
  const box = $("#investFields");
  box.replaceChildren();
  for (const f of fields) {
    const lab = document.createElement("label");
    lab.style.cssText = "display:flex;flex-direction:column;gap:4px;font-size:13px;color:var(--muted-fg);margin-bottom:10px;";
    lab.append(document.createTextNode(f.label), f.control);
    box.append(lab);
  }
  dlg.classList.remove("hidden");
  if (typeof dlg.showModal === "function") dlg.showModal(); else dlg.setAttribute("open", "");
  $("#iCancel").onclick = () => closeInvestDlg();
  $("#investForm").onsubmit = async (ev) => {
    ev.preventDefault();
    const done = btnPending($("#iSubmit"), "保存中…");
    try {
      await onSubmit();
      closeInvestDlg();
    } catch (err) {
      $("#iErr").textContent = err.message;
      toast(err.message, "err");
    } finally { done(); }
  };
}
function closeInvestDlg() {
  const dlg = $("#investDlg");
  dlg.close ? dlg.close() : dlg.removeAttribute("open");
  dlg.classList.add("hidden");
}
function dlgInput(value, placeholder) {
  const i = document.createElement("input");
  i.type = "text"; i.value = value || ""; if (placeholder) i.placeholder = placeholder;
  return i;
}

function openInstrumentDialog(bookId, edit) {
  const name = dlgInput(edit && edit.name, "如：贵州茅台");
  const typeSel = document.createElement("select");
  Object.entries(INSTR_TYPE_NAME).forEach(([v, t]) => {
    const o = document.createElement("option"); o.value = v; o.textContent = t;
    if (edit && String(edit.type) === v) o.selected = true;
    typeSel.append(o);
  });
  const market = dlgInput(edit && edit.market, "SH / SZ / HK…");
  const code = dlgInput(edit && edit.code, "600519");
  openInvestDialog(edit ? "编辑标的" : "新增标的",
    [{ label: "名称", control: name }, { label: "类型", control: typeSel }, { label: "市场（选填）", control: market }, { label: "代码（选填）", control: code }],
    async () => {
      const body = { name: name.value.trim(), type: Number(typeSel.value), market: market.value.trim(), code: code.value.trim() };
      if (edit) {
        await api("PUT", "/books/" + bookId + "/instruments/" + edit.id, body);
        toast("标的已更新", "ok");
      } else {
        await api("POST", "/books/" + bookId + "/instruments", body);
        toast("标的已创建", "ok");
      }
      renderInvest(bookId);
    });
}

function openHoldingDialog(bookId, edit) {
  const acctSel = document.createElement("select");
  for (const a of state.accounts) {
    const o = document.createElement("option"); o.value = String(a.id); o.textContent = a.name;
    acctSel.append(o);
  }
  if (edit && edit.accountId) acctSel.value = String(edit.accountId);
  acctSel.disabled = !!edit;
  const instrSel = document.createElement("select");
  // 拉取标的库填充（异步页面数据在 renderInvest 已加载，这里直接用 API 再取一次最稳）
  api("GET", "/books/" + bookId + "/instruments").then((d) => {
    instrSel.replaceChildren();
    for (const it of d.items || []) {
      const o = document.createElement("option"); o.value = String(it.id);
      o.textContent = it.name + (it.code ? "（" + it.code + "）" : "");
      instrSel.append(o);
    }
    if (edit && edit.instrumentId) { instrSel.value = String(edit.instrumentId); instrSel.disabled = true; }
  });
  const qty = dlgInput(edit && edit.quantity, "如 1000.50");
  const cost = dlgInput(edit ? cents2str(edit.costAmount) : "", "买入总成本（元）");
  const price = dlgInput(edit && edit.currentPrice !== "0" ? edit.currentPrice : "", "元/份");
  openInvestDialog(edit ? "编辑持仓" : "添加持仓",
    [{ label: "账户", control: acctSel }, { label: "标的", control: instrSel },
     { label: "数量", control: qty }, { label: "买入总成本（元，选填）", control: cost },
     { label: "现价（元/份，选填）", control: price }],
    async () => {
      const costCents = str2cents(cost.value);
      const body = {
        quantity: qty.value.trim(), costAmount: costCents === null ? 0 : costCents,
        currentPrice: price.value.trim() === "" ? "0" : price.value.trim()
      };
      if (edit) {
        await api("PUT", "/books/" + bookId + "/holdings/" + edit.id, body);
        toast("持仓已更新", "ok");
      } else {
        body.accountId = Number(acctSel.value);
        body.instrumentId = Number(instrSel.value);
        await api("POST", "/books/" + bookId + "/holdings", body);
        toast("持仓已添加", "ok");
      }
      renderInvest(bookId);
    });
}

function openPriceDialog(bookId, instrumentId, name, current) {
  const price = dlgInput(current !== "0" ? current : "", "元/份");
  openInvestDialog("录入行情 · " + name,
    [{ label: "最新价（元/份）", control: price }],
    async () => {
      await api("POST", "/books/" + bookId + "/instruments/" + instrumentId + "/price",
        { price: price.value.trim() });
      toast("行情已更新（持仓现价同步刷新）", "ok");
      renderInvest(bookId);
    });
}

/* ---------- 设置页（API Token 管理，对齐原型：安全设置/长期 API Token/退出登录） ---------- */

async function renderSettings() {
  const books = await api("GET", "/books");
  showNav(books.length ? books[0] : null);
  document.querySelectorAll("[data-nav]").forEach((a) => a.classList.remove("active"));
  $("#navSettings").classList.add("active");
  view.replaceChildren();

  const h = document.createElement("h2"); h.textContent = "设置";
  view.append(h);

  // API Token 卡片
  const card = document.createElement("div"); card.className = "card";
  const sh = document.createElement("h3"); sh.textContent = "安全设置 · 长期 API Token";
  card.append(sh);
  const desc = document.createElement("p"); desc.className = "muted";
  desc.textContent = "开发/API 用——此 Token 拥有你的账户完全访问权限，请勿泄露。明文只在生成时显示一次，关闭后无法再查看。";
  card.append(desc);

  // 一次性明文展示区（生成后渲染）
  const freshWrap = document.createElement("div");

  const genBtn = document.createElement("button"); genBtn.textContent = "生成新 Token";
  genBtn.addEventListener("click", () => {
    const name = dlgInput("", "用途备注（如：快捷指令）");
    openInvestDialog("生成长期 API Token", [{ label: "用途备注", control: name }], async () => {
      const data = await api("POST", "/users/me/tokens", { name: name.value.trim() });
      toast("Token 已生成——请立即复制保存", "ok");
      renderSettingsFresh(data.token, data.item);
    });
  });
  // 生成后重渲染列表 + 顶部明文条
  const renderSettingsFresh = (plaintext, item) => {
    freshWrap.replaceChildren();
    const box = document.createElement("div");
    box.style.cssText = "border:1px solid var(--primary);border-radius:12px;padding:12px;margin:10px 0;background:color-mix(in srgb, var(--primary) 6%, #fff);";
    const warn = document.createElement("p"); warn.style.cssText = "margin:0 0 6px;font-weight:600;";
    warn.textContent = "请立即复制保存（仅此一次展示）：";
    const code = document.createElement("code");
    code.style.cssText = "display:block;word-break:break-all;font-size:12px;user-select:all;";
    code.textContent = plaintext;
    const cp = document.createElement("button"); cp.textContent = "复制"; cp.className = "ghost"; cp.style.marginTop = "8px";
    cp.addEventListener("click", async () => {
      try { await navigator.clipboard.writeText(plaintext); toast("Token 已复制", "ok"); }
      catch (_) { toast("复制失败，请手动选中复制", "err"); }
    });
    box.append(warn, code, cp);
    freshWrap.append(box);
    loadTokens();
    void item;
  };
  card.append(genBtn, freshWrap);

  // Token 列表
  const listBox = document.createElement("div");
  card.append(listBox);
  async function loadTokens() {
    const items = await api("GET", "/users/me/tokens");
    listBox.replaceChildren();
    if (!items.length) {
      const empty = document.createElement("p"); empty.className = "muted"; empty.textContent = "还没有长期 Token。";
      listBox.append(empty);
      return;
    }
    const table = document.createElement("table");
    const thead = document.createElement("thead"); const trh = document.createElement("tr");
    ["备注", "创建时间", "最近使用", "状态", ""].forEach((t, i) => { const th = document.createElement("th"); th.textContent = t; if (i >= 1) th.className = "num"; trh.append(th); });
    thead.append(trh); table.append(thead);
    const tbody = document.createElement("tbody");
    for (const it of items) {
      const tr = document.createElement("tr");
      const mk = (t, cls) => { const td = document.createElement("td"); td.textContent = t; if (cls) td.className = cls; return td; };
      tr.append(
        mk(it.name || "—"),
        mk(fmtTime(it.createdAt), "num muted"),
        mk(it.lastUsedAt ? fmtTime(it.lastUsedAt) : "从未使用", "num muted"),
        mk(it.revokedAt ? "已撤销" : "有效", it.revokedAt ? "muted" : "income")
      );
      const tdOps = document.createElement("td"); tdOps.className = "row-actions";
      if (!it.revokedAt) {
        const b = document.createElement("button"); b.textContent = "撤销"; b.className = "danger";
        b.addEventListener("click", async () => {
          const okD = await confirmDialog({
            title: "撤销 Token", desc: "撤销立即生效，使用该 Token 的客户端/快捷指令将无法再访问。此操作不可恢复。",
            confirmText: "撤销", danger: true,
          });
          if (!okD) return;
          try {
            await api("DELETE", "/users/me/tokens/" + it.id);
            toast("Token 已撤销", "ok");
            loadTokens();
          } catch (err) { toast(err.message, "err"); }
        });
        tdOps.append(b);
      }
      tr.append(tdOps);
      tbody.append(tr);
    }
    table.append(tbody); listBox.append(table);
  }
  await loadTokens();
  view.append(card);

  // 退出登录卡片（对齐原型文案）
  const out = document.createElement("div"); out.className = "card";
  const oh = document.createElement("h3"); oh.textContent = "退出登录";
  const ob = document.createElement("button"); ob.textContent = "退出登录"; ob.className = "danger";
  ob.addEventListener("click", async () => {
    const okD = await confirmDialog({ title: "退出登录", desc: "退出后需要重新登录。本地数据会保留。", confirmText: "退出", danger: true });
    if (!okD) return;
    clearSession();
    toast("已退出登录", "info");
    route();
  });
  out.append(oh, ob);
  view.append(out);
}

/* ---------- 家庭成员页（对齐原型：家庭成员/邀请成员/角色/移除成员） ---------- */
async function renderMembers(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navMembers").classList.add("active");
  view.replaceChildren();

  const card = document.createElement("div"); card.className = "card";
  const row = document.createElement("div"); row.style.cssText = "display:flex;align-items:center;gap:10px;";
  const h = document.createElement("h2"); h.style.margin = "0"; h.textContent = "家庭成员";
  const invBtn = document.createElement("button"); invBtn.textContent = "邀请成员"; invBtn.style.marginLeft = "auto";
  invBtn.addEventListener("click", () => {
    const roleSel = document.createElement("select");
    [[3, "普通成员"], [2, "管理员"]].forEach(([v, t]) => {
      const o = document.createElement("option"); o.value = v; o.textContent = t; roleSel.append(o);
    });
    const bound = dlgInput("", "可选：限定用户名（空 = 任何人凭码加入）");
    openInvestDialog("邀请成员", [
      { label: "角色", control: roleSel },
      { label: "绑定用户名", control: bound },
    ], async () => {
      const data = await api("POST", "/books/" + bookId + "/members/invite",
        { role: Number(roleSel.value), boundUsername: bound.value.trim() });
      openInvestDialog("邀请码已生成（请复制给对方）", [], async () => { /* 只展示 */ });
      const box = $("#investFields");
      box.replaceChildren();
      const code = document.createElement("code");
      code.style.cssText = "display:block;word-break:break-all;font-size:14px;font-weight:600;user-select:all;margin:6px 0;";
      code.textContent = data.code;
      const note = document.createElement("p"); note.className = "muted"; note.style.margin = "0";
      note.textContent = "对方在登录页选择「加入账本」并输入邀请码即可。有效期见邀请设置（默认 7 天）。";
      const cp = document.createElement("button"); cp.textContent = "复制邀请码"; cp.className = "ghost";
      cp.addEventListener("click", async () => {
        try { await navigator.clipboard.writeText(data.code); toast("邀请码已复制", "ok"); }
        catch (_) { toast("复制失败，请手动选中复制", "err"); }
      });
      box.append(code, note, cp);
      $("#iSubmit").classList.add("hidden");
      toast("已生成邀请码", "ok");
    });
  });
  row.append(h, invBtn);
  card.append(row);
  const tip = document.createElement("p"); tip.className = "muted"; tip.textContent = "角色决定写权限：管理员可管理账户/分类与他人的交易；普通成员只能记录和查看自己权限范围内的账目。";
  card.append(tip);

  const members = await api("GET", "/books/" + bookId + "/members");
  const me = await api("GET", "/users/me");

  // 家庭信息块（P1-4：家庭名称 / 成员数量 / 创建时间）
  const book = books.find((b) => b.id === bookId) || {};
  const fblock = document.createElement("div"); fblock.className = "family-block";
  const fname = document.createElement("div"); fname.className = "fname"; fname.textContent = book.name || "家庭账本";
  const fmeta = document.createElement("div"); fmeta.className = "fmeta";
  const mkMeta = (val, label) => {
    const d = document.createElement("div");
    const b = document.createElement("b"); b.textContent = val;
    const s = document.createElement("span"); s.textContent = " " + label;
    d.append(b, s); return d;
  };
  fmeta.append(
    mkMeta(String(members.length), "名成员"),
    mkMeta(book.bookType === 2 ? "家庭账本" : "个人账本", "类型"),
    mkMeta(book.createdAt ? fmtTime(book.createdAt).slice(0, 10) : "—", "创建于")
  );
  fblock.append(fname, fmeta);
  card.append(fblock);

  const table = document.createElement("table");
  const thead = document.createElement("thead"); const trh = document.createElement("tr");
  ["成员姓名", "角色", "可见性", ""].forEach((t) => { const th = document.createElement("th"); th.textContent = t; trh.append(th); });
  thead.append(trh); table.append(thead);
  const tbody = document.createElement("tbody");
  const myUid = me.id;
  for (const mb of members) {
    const tr = document.createElement("tr");
    const tdN = document.createElement("td"); tdN.textContent = mb.username + (mb.uid === myUid ? "（我）" : "");
    const tdR = document.createElement("td");
    const badge = document.createElement("span"); badge.className = "badge";
    badge.textContent = mb.role === 1 ? "所有者" : (mb.role === 2 ? "管理员" : "普通成员");
    tdR.append(badge);
    const tdV = document.createElement("td");
    const vbadge = document.createElement("span"); vbadge.className = "badge " + (mb.canViewOthers ? "income" : "");
    vbadge.textContent = mb.canViewOthers ? "已开启账本可见" : "已关闭账本可见";
    tdV.append(vbadge);
    const tdOps = document.createElement("td"); tdOps.className = "row-actions";
    if (mb.role === 3) {
      const bUp = document.createElement("button"); bUp.textContent = "设为管理员"; bUp.className = "ghost";
      bUp.addEventListener("click", async () => {
        try {
          await api("PUT", "/books/" + bookId + "/members/" + mb.uid, { role: 2 });
          toast("已设为管理员", "ok");
          renderMembers(bookId);
        } catch (err) { toast(err.message, "err"); }
      });
      tdOps.append(bUp);
    } else if (mb.role === 2) {
      const bDown = document.createElement("button"); bDown.textContent = "降级为成员"; bDown.className = "ghost";
      bDown.addEventListener("click", async () => {
        try {
          await api("PUT", "/books/" + bookId + "/members/" + mb.uid, { role: 3 });
          toast("已降级为成员", "ok");
          renderMembers(bookId);
        } catch (err) { toast(err.message, "err"); }
      });
      tdOps.append(bDown);
    }
    if (mb.role !== 1) {
      const bDel = document.createElement("button"); bDel.textContent = "移除成员"; bDel.className = "danger";
      bDel.addEventListener("click", async () => {
        const okD = await confirmDialog({
          title: "移除成员", desc: "移除后该成员将无法再访问家庭账本。", confirmText: "确认移除", danger: true,
        });
        if (!okD) return;
        try {
          await api("DELETE", "/books/" + bookId + "/members/" + mb.uid);
          toast("成员已移除", "ok");
          renderMembers(bookId);
        } catch (err) { toast(err.message, "err"); }
      });
      tdOps.append(bDel);
    }
    tr.append(tdN, tdR, tdV, tdOps);
    tbody.append(tr);
  }
  table.append(tbody);
  card.append(table);
  view.append(card);
}

/* ---------- 投资账户详情（对账页投资账户点入：账户信息 + 该账户持仓） ---------- */
async function renderAccountDetail(bookId, accId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  showNav(books.find((b) => b.id === bookId) || { id: bookId });
  $("#navAccounts").classList.add("active");
  view.replaceChildren();

  // 账户信息（含余额）——列表里已有，直接取；找不到就单独拉
  let acc = state.accounts.find((a) => a.id === accId);
  if (!acc) {
    acc = await api("GET", "/books/" + bookId + "/accounts/" + accId);
  }

  const card = document.createElement("div"); card.className = "card";
  const back = document.createElement("button"); back.textContent = "← 返回对账"; back.className = "ghost";
  back.style.marginBottom = "10px";
  back.addEventListener("click", () => location.hash = "#b/" + bookId + "/accounts");
  const h = document.createElement("h2");
  h.textContent = (acc ? acc.name : "账户") + " · " + (ACC_CAT_NAME[acc && acc.category] || "投资账户");
  const bal = document.createElement("p");
  bal.style.cssText = "font-size:20px;font-weight:600;";
  bal.textContent = "账户余额 ¥" + cents2str(acc ? acc.balance : 0);
  const note = document.createElement("p"); note.className = "muted";
  note.textContent = "投资账户余额 = 现金部分；下方持仓市值另计入总资产（不在账户余额内）。";
  card.append(back, h, bal, note);
  view.append(card);

  // 该账户的持仓
  const hdata = await api("GET", "/books/" + bookId + "/holdings");
  const holdings = (hdata.items || []).filter((it) => it.accountId === accId);
  const hCard = document.createElement("div"); hCard.className = "card";
  const hh = document.createElement("h2"); hh.textContent = "持仓明细";
  hCard.append(hh);
  if (!holdings.length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "📈";
    const p = document.createElement("p"); p.textContent = "该账户暂无持仓——到「资产」页添加。";
    empty.append(ic, p); hCard.append(empty);
  } else {
    const table = document.createElement("table");
    const thead = document.createElement("thead"); const trh = document.createElement("tr");
    ["标的", "数量", "成本", "现价", "市值", "浮动盈亏"].forEach((t, i) => {
      const th = document.createElement("th"); th.textContent = t; if (i >= 1) th.className = "num"; trh.append(th);
    });
    thead.append(trh); table.append(thead);
    const tbody = document.createElement("tbody");
    let totalMV = 0;
    for (const it of holdings) {
      totalMV += it.marketValue;
      const tr = document.createElement("tr");
      const mk = (t, cls) => { const td = document.createElement("td"); td.textContent = t; if (cls) td.className = cls; return td; };
      tr.append(
        mk(it.instrumentName || "标的 " + it.instrumentId),
        mk(it.quantity, "num"),
        mk("¥" + cents2str(it.costAmount), "num"),
        mk(it.currentPrice, "num muted"),
        mk("¥" + cents2str(it.marketValue), "num"),
        mk((it.profitLoss >= 0 ? "+" : "") + "¥" + cents2str(Math.abs(it.profitLoss)), "num " + (it.profitLoss >= 0 ? "income" : "expense"))
      );
      tbody.append(tr);
    }
    table.append(tbody); hCard.append(table);
    const total = document.createElement("p");
    total.style.cssText = "text-align:right;font-weight:600;margin:10px 0 0;";
    total.textContent = "市值合计 ¥" + cents2str(totalMV);
    hCard.append(total);
  }
  view.append(hCard);
}

/* ---------- 路由 ---------- */
/* ---------- 日期分组（今天/昨天/前天/N天前） ---------- */
function dateGroupKey(unixSec) {
  const d = new Date(unixSec * 1000);
  return Math.floor(new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime() / 1000);
}
function dateGroupLabel(unixSec) {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const d = new Date(unixSec * 1000);
  const that = new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const days = Math.round((today - that) / 86400000);
  if (days === 0) return "今天";
  if (days === 1) return "昨天";
  if (days === 2) return "前天";
  if (days < 30) return days + " 天前";
  return (d.getMonth() + 1) + "月" + d.getDate() + "日";
}

/* ---------- 底部导航 + 中央 FAB（移动端；桌面由顶部导航承载） ---------- */
function wireBottomNav(bookId, activePath) {
  const nav = $("#bottomNav");
  if (!nav) return;
  nav.querySelectorAll(".tab").forEach((t) => {
    const p = t.dataset.path || "";
    t.classList.toggle("active", p === activePath);
    t.onclick = () => { location.hash = "#b/" + bookId + p; };
  });
  const fab = $("#fab");
  if (fab) fab.onclick = () => openTxForm(bookId, () => route());
}

/* ---------- 账单流（按日期分组渲染，点击进入详情） ---------- */
async function renderBillStream(bookId, container) {
  container.replaceChildren();
  const sk = document.createElement("div"); sk.setAttribute("aria-busy", "true");
  for (let i = 0; i < 4; i++) { const b = document.createElement("div"); b.className = "skel"; b.style.width = (90 - i * 12) + "%"; sk.append(b); }
  container.append(sk);
  let data;
  try { data = await api("GET", "/books/" + bookId + "/transactions?page=1&count=60"); }
  catch (e) { toast(e.message, "err"); return; }
  const items = data.items || [];
  container.replaceChildren();
  if (!items.length) {
    const empty = document.createElement("div"); empty.className = "empty";
    const ic = document.createElement("div"); ic.className = "icon"; ic.textContent = "🧾";
    const p = document.createElement("p"); p.textContent = "还没有账单——点下方「＋ 记一笔」开始记录。";
    empty.append(ic, p); container.append(empty);
    return;
  }
  const groups = new Map();
  for (const tx of items) {
    state.txMap = state.txMap || {}; state.txMap[tx.id] = tx;
    const key = dateGroupKey(tx.txTime);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(tx);
  }
  const keys = [...groups.keys()].sort((a, b) => b - a);
  for (const key of keys) {
    const arr = groups.get(key);
    let daySum = 0;
    for (const t of arr) { if (t.txType === 1) daySum += t.amount; else if (t.txType === 2) daySum -= t.amount; }
    const gt = document.createElement("div"); gt.className = "bill-group-title";
    const lt = document.createElement("span"); lt.textContent = dateGroupLabel(key);
    const rt = document.createElement("span"); rt.className = "sum";
    rt.textContent = (daySum >= 0 ? "+" : "-") + "¥" + cents2str(Math.abs(daySum));
    gt.append(lt, rt); container.append(gt);
    for (const tx of arr) {
      const row = document.createElement("div"); row.className = "bill-row";
      const ico = document.createElement("div"); ico.className = "bill-ico";
      ico.textContent = tx.txType === 1 ? "💰" : tx.txType === 2 ? "💸" : "🔁";
      const main = document.createElement("div"); main.className = "bill-main";
      const t1 = document.createElement("div"); t1.className = "t1";
      t1.textContent = tx.txType === 3
        ? (accountName(tx.accountId) + " → " + accountName(tx.toAccountId))
        : (categoryName(tx.categoryId) || (tx.note || tx.merchant || TX_TYPE_NAME[tx.txType] || "交易"));
      const t2 = document.createElement("div"); t2.className = "t2";
      t2.textContent = (tx.note || "") + (tx.merchant ? " · " + tx.merchant : "") + " · " + fmtTime(tx.txTime).slice(11);
      main.append(t1, t2);
      const amt = document.createElement("div");
      amt.className = "bill-amt " + (tx.txType === 1 ? "income" : tx.txType === 2 ? "expense" : "transfer");
      amt.textContent = (tx.txType === 1 ? "+" : tx.txType === 2 ? "-" : "") + "¥" + cents2str(tx.amount);
      row.append(ico, main, amt);
      row.addEventListener("click", () => { location.hash = "#b/" + bookId + "/tx/" + tx.id; });
      container.append(row);
    }
  }
}

/* ---------- P0-1 首页账单页（总收入/总支出/结余 + 账单流） ---------- */
async function renderHome(bookId) {
  state.bookId = bookId;
  const { accs, cats } = await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  const book = books.find((b) => b.id === bookId) || { id: bookId };
  showNav(book); wireBottomNav(bookId, "");
  $("#navTxns").classList.remove("active");
  view.replaceChildren();

  const card = document.createElement("div"); card.className = "card";
  const h = document.createElement("h2");
  h.style.display = "flex"; h.style.alignItems = "center"; h.style.justifyContent = "space-between";
  const ht = document.createElement("span"); ht.textContent = book.name + " · 概览";
  const add = document.createElement("button"); add.className = "only-desktop"; add.textContent = "＋ 记一笔";
  add.style.padding = "5px 12px"; add.onclick = () => openTxForm(bookId, () => route());
  h.append(ht, add);
  card.append(h);

  const sumWrap = document.createElement("div"); sumWrap.className = "summary";
  let monthly = [];
  try { const m = await api("GET", "/books/" + bookId + "/reports/monthly?months=2"); monthly = (m && m.items) || []; } catch (_) {}
  const cur = monthly[monthly.length - 1] || { income: 0, expense: 0 };
  let balance = 0; for (const a of accs) balance += a.balance || 0;
  const mkCard = (label, value, cls, sub) => {
    const c = document.createElement("div"); c.className = "summary-card " + (cls || "");
    const l = document.createElement("div"); l.className = "label"; l.textContent = label;
    const v = document.createElement("div"); v.className = "value"; v.textContent = value;
    c.append(l, v);
    if (sub) { const s = document.createElement("div"); s.className = "sub"; s.textContent = sub; c.append(s); }
    return c;
  };
  sumWrap.append(
    mkCard("本月收入", "+¥" + cents2str(cur.income), "", "家庭总收入"),
    mkCard("本月支出", "-¥" + cents2str(cur.expense), "", "家庭总支出"),
    mkCard("结余", "¥" + cents2str(balance), "accent", "净资产")
  );
  card.append(sumWrap);
  view.append(card);

  const stream = document.createElement("div");
  view.append(stream);
  await renderBillStream(bookId, stream);
}

/* ---------- P0-2 交易详情页 ---------- */
async function renderTxDetail(bookId, txId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  const book = books.find((b) => b.id === bookId) || { id: bookId };
  showNav(book); wireBottomNav(bookId, "/txns");
  view.replaceChildren();
  let tx;
  try { tx = await api("GET", "/books/" + bookId + "/transactions/" + txId); }
  catch (e) { toast(e.message, "err"); location.hash = "#b/" + bookId + "/txns"; return; }
  state.txMap = state.txMap || {}; state.txMap[tx.id] = tx;

  const card = document.createElement("div"); card.className = "card tx-detail";
  const hero = document.createElement("div"); hero.className = "hero";
  const amt = document.createElement("div");
  amt.className = "amt " + (tx.txType === 1 ? "income" : tx.txType === 2 ? "expense" : "transfer");
  amt.textContent = (tx.txType === 1 ? "+" : tx.txType === 2 ? "-" : "") + "¥" + cents2str(tx.amount);
  const typ = document.createElement("div"); typ.className = "typ"; typ.textContent = TX_TYPE_NAME[tx.txType] || "交易";
  hero.append(amt, typ); card.append(hero);

  const rows = document.createElement("div"); rows.className = "rows";
  const addRow = (k, v) => {
    const r = document.createElement("div"); r.className = "row";
    const kk = document.createElement("span"); kk.className = "k"; kk.textContent = k;
    const vv = document.createElement("span"); vv.className = "v"; vv.textContent = v;
    r.append(kk, vv); rows.append(r);
  };
  addRow("账户", tx.txType === 3 ? (accountName(tx.accountId) + " → " + accountName(tx.toAccountId)) : accountName(tx.accountId));
  if (tx.txType !== 3) addRow("分类", categoryName(tx.categoryId));
  addRow("日期", fmtTime(tx.txTime));
  addRow("备注", tx.note || (tx.merchant ? "商户：" + tx.merchant : "—"));
  card.append(rows);

  const actions = document.createElement("div"); actions.className = "actions";
  const editBtn = document.createElement("button"); editBtn.className = "ghost"; editBtn.textContent = "编辑";
  editBtn.onclick = () => openTxEditForm(bookId, tx, () => renderTxDetail(bookId, txId));
  const delBtn = document.createElement("button"); delBtn.className = "danger"; delBtn.textContent = "删除";
  delBtn.onclick = async () => {
    const ok = await confirmDialog({
      title: "删除交易",
      desc: "删除后将无法恢复，账户余额会相应调整。",
      confirmText: "删除", danger: true
    });
    if (!ok) return;
    const done = btnPending(delBtn, "删除中…");
    try {
      await api("DELETE", "/books/" + bookId + "/transactions/" + tx.id + "?clientUuid=" + encodeURIComponent(uuid()));
      toast("已删除", "ok"); location.hash = "#b/" + bookId + "/txns";
    } catch (e) { toast(e.message, "err"); } finally { done(); }
  };
  actions.append(editBtn, delBtn); card.append(actions);
  view.append(card);
}

/* ---------- P0-3 两级分类管理 ---------- */
function openCatForm(bookId, cat) {
  const dlg = $("#catDlg");
  $("#catFormTitle").textContent = cat ? "编辑分类" : "新增分类";
  $("#catName").value = cat ? cat.name : "";
  $("#catType").value = String(cat ? cat.catType : 2);
  const tops = state.categories.filter((c) => !c.parentId && (!cat || c.id !== cat.id));
  fillSelect($("#catParent"), tops, "无（作为大类）");
  $("#catParent").value = cat && cat.parentId ? String(cat.parentId) : "0";
  $("#catErr").textContent = "";
  dlg.classList.remove("hidden");
  if (typeof dlg.showModal === "function") dlg.showModal(); else dlg.setAttribute("open", "");
  $("#catCancel").onclick = () => { dlg.close ? dlg.close() : dlg.removeAttribute("open"); dlg.classList.add("hidden"); };
  $("#catForm").onsubmit = async (e) => {
    e.preventDefault();
    const name = $("#catName").value.trim();
    if (!name) { $("#catErr").textContent = "请输入分类名称"; return; }
    const body = { name, catType: Number($("#catType").value), parentId: Number($("#catParent").value) };
    const done = btnPending($("#catSubmit"), "保存中…");
    try {
      if (cat) await api("PUT", "/books/" + bookId + "/categories/" + cat.id, body);
      else await api("POST", "/books/" + bookId + "/categories", body);
      toast(cat ? "分类已更新" : "分类已添加", "ok");
      dlg.close ? dlg.close() : dlg.removeAttribute("open"); dlg.classList.add("hidden");
      await renderCategories(bookId);
    } catch (err) { $("#catErr").textContent = err.message; } finally { done(); }
  };
}
async function deleteCategory(bookId, cat) {
  let n = 0;
  try { const d = await api("GET", "/books/" + bookId + "/transactions?categoryId=" + cat.id + "&count=1"); n = (d && d.total) || 0; } catch (_) {}
  const ok = await confirmDialog({
    title: "删除分类",
    desc: "删除「" + cat.name + "」后，" + n + " 笔账单将归入「其他」分类，且不可恢复。",
    confirmText: "删除", danger: true
  });
  if (!ok) return;
  const done = btnPending(document.activeElement, "删除中…");
  try { await api("DELETE", "/books/" + bookId + "/categories/" + cat.id); toast("分类已删除", "ok"); await renderCategories(bookId); }
  catch (e) { toast(e.message, "err"); } finally { done(); }
}
async function renderCategories(bookId) {
  state.bookId = bookId;
  const { cats } = await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  const book = books.find((b) => b.id === bookId) || { id: bookId };
  showNav(book); wireBottomNav(bookId, "");
  view.replaceChildren();

  const card = document.createElement("div"); card.className = "card";
  const h = document.createElement("h2"); h.textContent = "分类管理"; card.append(h);
  const sub = document.createElement("p"); sub.className = "muted";
  sub.textContent = "管理支出 / 收入的大类与小类；删除分类后，相关账单将归入「其他」。";
  card.append(sub);

  const addBar = document.createElement("div"); addBar.className = "cat-add";
  const addTop = document.createElement("button"); addTop.textContent = "＋ 新增大类";
  addTop.onclick = () => openCatForm(bookId, null);
  addBar.append(addTop); card.append(addBar);

  let tree = [];
  try { const t = await api("GET", "/books/" + bookId + "/categories?tree=1"); tree = t || []; } catch (_) {}

  const renderGroup = (catType, title) => {
    const items = (tree || []).filter((c) => c.catType === catType);
    const grp = document.createElement("div"); grp.className = "cat-group";
    const head = document.createElement("div"); head.className = "cat-group-head";
    const nm = document.createElement("span"); nm.className = "name"; nm.textContent = title;
    head.append(nm); grp.append(head);
    if (!items.length) {
      const e = document.createElement("div"); e.className = "cat-empty";
      e.textContent = "暂无，点击上方「新增大类」添加。"; grp.append(e);
    }
    for (const top of items) {
      const item = document.createElement("div"); item.className = "cat-item";
      const ico = document.createElement("div"); ico.className = "cat-ico"; ico.textContent = "📂";
      const main = document.createElement("div"); main.style.flex = "1";
      const n = document.createElement("div"); n.className = "cat-name"; n.textContent = top.name;
      main.append(n);
      const ra = document.createElement("div"); ra.className = "row-actions";
      const addSub = document.createElement("button"); addSub.className = "ghost"; addSub.textContent = "＋小类";
      addSub.onclick = () => openCatForm(bookId, { parentId: top.id, catType: top.catType });
      const edit = document.createElement("button"); edit.className = "ghost"; edit.textContent = "编辑";
      edit.onclick = () => openCatForm(bookId, top);
      const del = document.createElement("button"); del.className = "danger"; del.textContent = "删除";
      del.onclick = async () => { await deleteCategory(bookId, top); };
      ra.append(addSub, edit, del);
      item.append(ico, main, ra); grp.append(item);
      for (const kid of (top.children || [])) {
        const ki = document.createElement("div"); ki.className = "cat-item";
        const kiIco = document.createElement("div"); kiIco.className = "cat-ico"; kiIco.textContent = "🔖";
        const kiMain = document.createElement("div"); kiMain.style.flex = "1";
        const kn = document.createElement("div"); kn.className = "cat-name"; kn.textContent = kid.name;
        kiMain.append(kn);
        const kra = document.createElement("div"); kra.className = "row-actions";
        const ke = document.createElement("button"); ke.className = "ghost"; ke.textContent = "编辑";
        ke.onclick = () => openCatForm(bookId, kid);
        const kd = document.createElement("button"); kd.className = "danger"; kd.textContent = "删除";
        kd.onclick = async () => { await deleteCategory(bookId, kid); };
        kra.append(ke, kd); ki.append(kiIco, kiMain, kra); grp.append(ki);
      }
    }
    card.append(grp);
  };
  renderGroup(2, "支出分类");
  renderGroup(1, "收入分类");
  view.append(card);
}

/* ---------- P1-3 我的（聚合入口 + 资料编辑） ---------- */
async function renderMine(bookId) {
  state.bookId = bookId;
  await loadBookMeta(bookId);
  const books = await api("GET", "/books");
  const book = books.find((b) => b.id === bookId) || { id: bookId };
  showNav(book); wireBottomNav(bookId, "/mine");
  view.replaceChildren();

  const card = document.createElement("div"); card.className = "card";
  const head = document.createElement("div"); head.className = "mine-head";
  const av = document.createElement("div"); av.className = "avatar";
  av.textContent = (state.whoami || "我").slice(0, 1).toUpperCase();
  const info = document.createElement("div");
  const w = document.createElement("div"); w.className = "who"; w.textContent = state.whoami || "我";
  const s = document.createElement("div"); s.className = "sub"; s.textContent = "家庭记账成员";
  info.append(w, s); head.append(av, info); card.append(head);

  // 资料编辑（邮箱可改；用户名按后端设计为只读）
  let me = null;
  try { me = await api("GET", "/users/me"); } catch (_) {}
  const sec1 = document.createElement("div"); sec1.className = "mine-section-title"; sec1.textContent = "资料";
  card.append(sec1);
  const pf = document.createElement("div"); pf.className = "rows";
  const rowU = document.createElement("div"); rowU.className = "row";
  const ku = document.createElement("span"); ku.className = "k"; ku.textContent = "用户名";
  const vu = document.createElement("span"); vu.className = "v"; vu.textContent = (me && me.username) || state.whoami || "—";
  rowU.append(ku, vu);
  const rowE = document.createElement("div"); rowE.className = "row"; rowE.style.flexDirection = "column"; rowE.style.alignItems = "stretch"; rowE.style.gap = "6px";
  const ke = document.createElement("span"); ke.className = "k"; ke.textContent = "邮箱";
  const eWrap = document.createElement("div"); eWrap.style.display = "flex"; eWrap.style.gap = "8px";
  const eInput = document.createElement("input"); eInput.type = "email"; eInput.value = (me && me.email) || ""; eInput.placeholder = "未设置"; eInput.style.flex = "1";
  const eSave = document.createElement("button"); eSave.textContent = "保存"; eSave.className = "ghost";
  eSave.onclick = async () => {
    const done = btnPending(eSave, "保存中…");
    try { await api("PUT", "/users/me", { email: eInput.value.trim() }); toast("资料已更新", "ok"); }
    catch (err) { toast(err.message, "err"); } finally { done(); }
  };
  eWrap.append(eInput, eSave); rowE.append(ke, eWrap);
  pf.append(rowU, rowE); card.append(pf);

  const sec2 = document.createElement("div"); sec2.className = "mine-section-title"; sec2.textContent = "账本与数据";
  card.append(sec2);
  const links = [
    ["📒", "我的账本", "#books"],
    ["💳", "账户", "#b/" + bookId + "/accounts"],
    ["👥", "家庭成员", "#b/" + bookId + "/members"],
    ["📤", "导出数据", "#b/" + bookId + "/export"],
    ["📊", "报表", "#b/" + bookId + "/report"],
    ["🎯", "预算", "#b/" + bookId + "/budget"],
    ["📈", "资产", "#b/" + bookId + "/invest"],
    ["♻️", "回收站", "#b/" + bookId + "/recycle"],
  ];
  for (const [ico, name, href] of links) {
    const a = document.createElement("a"); a.className = "mine-link"; a.href = href;
    const i = document.createElement("div"); i.className = "ico"; i.textContent = ico;
    const t = document.createElement("span"); t.textContent = name;
    const ar = document.createElement("span"); ar.className = "arrow"; ar.textContent = "›";
    a.append(i, t, ar); card.append(a);
  }
  view.append(card);
}

async function route() {
  const hash = location.hash || "#";
  // 交易详情：#b/:id/tx/:txId（先于一般 #b/:id/... 匹配）
  const mt = hash.match(/^#b\/(\d+)\/tx\/(\d+)/);
  // 投资账户详情：#b/:id/account/:accId
  const md = hash.match(/^#b\/(\d+)\/account\/(\d+)/);
  // 账本子路由：txns/categories/mine/accounts/recycle/report/export/budget/invest/members
  const m = hash.match(/^#b\/(\d+)(?:\/(txns|categories|mine|accounts|recycle|report|export|budget|invest|members))?/);
  try {
    if (!state.token) { renderLogin(); return; }
    if (mt) {
      await renderTxDetail(Number(mt[1]), Number(mt[2]));
    } else if (md) {
      await renderAccountDetail(Number(md[1]), Number(md[2]));
    } else if (hash === "#settings") {
      await renderSettings();
    } else if (m) {
      const bookId = Number(m[1]);
      const sub = m[2];
      if (sub === "txns") await renderTxns(bookId);
      else if (sub === "categories") await renderCategories(bookId);
      else if (sub === "mine") await renderMine(bookId);
      else if (sub === "accounts") await renderAccounts(bookId);
      else if (sub === "recycle") await renderRecycle(bookId);
      else if (sub === "report") await renderReport(bookId);
      else if (sub === "export") await renderExport(bookId);
      else if (sub === "budget") await renderBudget(bookId);
      else if (sub === "invest") await renderInvest(bookId);
      else if (sub === "members") await renderMembers(bookId);
      else await renderHome(bookId);                 // 默认落地首页账单页（P0-1）
    } else if (hash === "#books" || hash === "#") {
      await renderBooks();
    } else {
      await renderBooks();
    }
  } catch (e) {
    toast(e.message, "err");
    if (!state.token) renderLogin();
  }
}
window.addEventListener("hashchange", route);
route();
