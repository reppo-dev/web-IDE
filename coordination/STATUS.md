# وضعیت پروژه NexusAI

> این فایل را هر سه ایجنت به‌روز می‌کنند. کوتاه نگهش دارید.

**آخرین به‌روزرسانی:** ۲۰۲۶-۱۰-۰۳ — معماری Plan-Driven Loop، TaskVerifier، و Task Board کامل و ۱۰۰٪ سبز شد.

---

## 🎯 هدف

IDE چند-ایجنتی که ایجنت‌های هوش مصنوعی با هم بحث و نقد می‌کنند و پروژه را جلو می‌برند.

---

## ✅ کارهای انجام‌شده

| کار | وضعیت |
| :--- | :--- |
| بک‌اند Go با ۵ ایجنت (Architect, Coder, Critic, Security, Arbiter) | کامل |
| سه حالت اجرا (rolebased / concurrent / sequential) | کامل |
| ماتریس بازبینی متقابل N-to-N | کامل |
| حافظه‌ی SQLite (بدون CGO) | کامل |
| WebSocket و رابط کاربری React/Monaco | کامل |
| **تشخیص نیت** (سلام/سؤال ← بدون ساخت پروژه) | تازه — Claude |
| **حذف مدل پیش‌فرض** (انتخاب اجباری کاربر) | تازه — Claude |
| **حذف کلید hardcode** از کد | تازه — Claude |
| **مسیریابی ارائه‌دهنده** (`ResolveProvider`) | کامل — هر دو ایجنت |
| **Relay Discussion (گفتگوی ۵ مدل در حالت چت)** | کامل — Antigravity |
| **استپر تفکر پله‌به‌پله و کارت اجماع نهایی در فرانت‌اند** | کامل — Antigravity |
| **Context Engine (خواندن کدهای واقعی مخزن)** | کامل — Claude |
| **اعتبارسنجی کدهای معتبر (`validate.go`)** | کامل — Claude |
| **پشتیبانی از استریم SSE در کلاینت LLM** | کامل — Antigravity |
| **Shared Project Brain (حل گم‌شدن اطلاعات و سقف حافظه)** | کامل — Antigravity |
| **سیستم تیم‌سازی دلخواه (Custom Teams & Agents)** | کامل — Claude & Antigravity |
| **ذخیره‌سازی تیم‌ها در SQLite (`team.Store`)** | کامل — Antigravity |
| **ارکستریتور اجرای تیم‌های دلخواه (`RunTeam` و فازها)** | کامل — Claude & Antigravity |
| **اندپوینت‌های REST مدیریت تیم‌ها (`/api/teams`)** | کامل — Antigravity |
| **تأیید تست‌های بک‌اند (۱۲ پکیج ۱۰۰٪ پاس)** | کامل — Antigravity |
| **رفع قفل گیت (`.git/index.lock`)** | کامل — Antigravity |
| **رفع باگ DNS/IPv6 لوکال در ویندوز (سوئیچ به 127.0.0.1)** | کامل — Antigravity |
| **ارائه پرامپت به تمام نقش‌های تیم‌های دلخواه در `buildWork`** | کامل — Antigravity |
| **رفع باگ MarshalJSON در `json.RawMessage` و تگ‌های think** | کامل — Antigravity |
| **تست زنده فرانت‌اند و تولید موفق کدهای Go با قبولی ۱۰۰٪ تست‌ها** | کامل — Antigravity |
| **رجیستری پروژه‌ها و باز کردن هر پوشه از دیسک (`/api/projects`)** | کامل — Claude & Antigravity |
| **باز شدن پروژه‌های دیسک در ادیتور Monaco و درخت فایل Explorer** | کامل — Claude & Antigravity |
| **تیم دائمی دولوپرها (nemo_qwen, nemotron, bunny, flash_5, mimo بر روی ناین‌روتر)** | کامل — Antigravity |
| **سند استراتژیک معماری و چشم‌انداز (`PROJECT_VISION.md`)** | کامل — کاربر و هر دو ایجنت |
| **پشتیبانی کانتکست از فلاتر و وب (`.dart`, `.xml`, `.gradle`)** | کامل — Antigravity |
| **فازبندی قرارداد JSON تیم‌های دلخواه (الزام به تولید کد به جای فقط خلاصه)** | کامل — Claude & Antigravity |
| **قفل همزمانی `sync.RWMutex` در `SwarmContext` و تست‌های ایمنی داده** | کامل — Claude & Antigravity |
| **موتور ارزیابی و سندباکس چندزبانه (Verification Engine برای Go, Flutter, Node)** | کامل — Antigravity |
| **سیستم چک‌پوینت و رول‌بک Git (`RollbackToCheckpoint`, `RollbackUncommitted`)** | کامل — Antigravity |
| **موتور ابزارها و اجرای حلقوی ابزارها (Tools Engine & Tool Loop)** | تازه — Claude & Antigravity |
| **رفع باگ 404 مدل‌های لوکال در ToolLoop (گم‌شدن BaseURL/Key)** | تازه — Antigravity |
| **LLM Guard: Retry با Backoff + محدودیت هم‌زمانی + بودجه توکن سشن** | تازه — Buffy |
| **Plan-Driven Loop & Task State Machine (معماری حلقه مبتنی بر تسک‌ها)** | کامل — Claude & Antigravity |
| **TaskVerifier & Verification Gate (اعتبارسنجی حقیقت زمینی با تست و کد)** | کامل — Claude & Antigravity |
| **ذخیره‌سازی و رندر تسک‌بورد در .project/ (plan.json و tasks.md)** | کامل — Claude & Antigravity |
| **اندپوینت REST برنامه پروژه (`GET /api/projects/{id}/plan`)** | کامل — Antigravity |
| **استریم زنده تسک‌بورد با وب‌سوکت (`EventPlanUpdated`)** | کامل — Antigravity |
| **اتصال پایان بودجه توکن به `WAITING_FOR_HUMAN` (بند ۲۶ سند ویژن)** | کامل — Antigravity |
| **ویجت زنده تسک‌بورد با نوار پیشرفت در فرانت‌اند (Composer Panel)** | کامل — Antigravity |
| **تأییدیه تست‌ها (۱۲ پکیج Go ۱۰۰٪ سبز + بیلد فرانت‌اند تمیز)** | کامل — Antigravity |

---

## ⚠️ کارهای در دست و اولویت‌ها

### ۱. استریم لحظه‌ای توکن‌ها (Token Streaming over WebSocket)
نمایش زنده جریان کلمات تولیدی هر مدل روی Composer Panel در زمان واقعی.

### ۲. ثبت رویدادها در Change Ledger (`.project/ledger.jsonl` — بند ۳۰ سند ویژن)
ثبت ساختاریافته‌ی هر تغییر کد، دلیل تغییر، ایجنت انجام‌دهنده و کامیت گیت در یک فایل لاگ تاریخچه‌ای.

### ۳. مدیریت هزینه‌ها و تفکیک هزینه/توکن به‌ازای هر Agent (بند ۳۷ سند ویژن)
ثبت و گزارش توکن‌های ورودی و خروجی مصرفی توسط هر ایجنت برای مشاهده‌ی هزینه و کارایی مدل‌ها.

---

## 🎯 کار بعدی
همکاری با Claude: شروع یکی از اولویت‌های ۱ یا ۲ (ترجیحاً Change Ledger یا Token Streaming) با هماهنگی در `LIVE_BOARD.md` و رزرو در `IN_PROGRESS.md`.

---

## 📌 یادآوری‌ها

- `AGENTS.md` را قبل از هر کار بخوانید
- `IN_PROGRESS.md` را چک کنید تا فایل رزروشده نباشد
- کلید API هرگز در کد hardcode نشود