# وضعیت پروژه NexusAI

> این فایل را هر سه ایجنت به‌روز می‌کنند. کوتاه نگهش دارید.

**آخرین به‌روزرسانی:** ۲۰۲۶-۱۰-۰۵ — چک‌پوینت Git روی ref جدا (`refs/nexusai/checkpoints`)، پذیرش واقعی تغییرات (`POST /api/projects/:id/accept`)، و قانون Grounding/Evidence برای حافظه پروژه (بند ۱۰ و ۲۲) کامل شد. ۱۲ پکیج Go بک‌اند ۱۰۰٪ سبز.

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
| **Change Ledger تغییرناپذیر (`.project/ledger.jsonl` — بند ۳۰ سند ویژن)** | کامل — Antigravity |
| **اندپوینت REST لجر تغییرات (`GET /api/projects/{id}/ledger`)** | کامل — Antigravity |
| **اتصال زنده Change Ledger و تاریخچه کامیت‌ها به UI (سایدبار Source Control)** | کامل — Antigravity |
| **تأییدیه تست‌ها (۱۲ پکیج Go ۱۰۰٪ سبز + بیلد فرانت‌اند تمیز)** | کامل — Antigravity |
| **اتصال Plan-Driven Loop به Custom Teams (`RunTeam`)** | کامل — Claude & Antigravity |
| **تزریق تسک فعال به کانتکست کدنویس‌ها و بازبین‌ها (`activeTaskBrief`)** | کامل — Claude & Antigravity |
| **سقف سخت Rework = ۳ و تشخیص تکرار خطا + ارتقا به `WAITING_FOR_HUMAN` (بند ۱۹)** | کامل — Claude & Antigravity |
| **حفاظت و سانسور اسرار و توکن‌ها (`RedactSecrets` + `SafeReadPath` — بند ۲۵)** | کامل — Antigravity & Claude |
| **تفکیک توکن‌ها و دفعات فراخوانی به‌ازای Agent و Model (§۳۷ Observability)** | کامل — Antigravity |
| **قفل فایل با اجاره زمانی (File Lease Lock — جلوگیری از تصادم در فایل‌های مشترک)** | کامل — Claude & Antigravity |
| **کارت زنده استریم تسک‌ها (Task = One Live Message و کامپوننت `TaskStreamCard`)** | کامل — Claude & Antigravity |
| **موازی‌سازی واقعی تسک‌ها (True Task Parallelism با تفکیک فایل‌ها و قفل‌های اتمیک)** | کامل — Claude & Antigravity |
| **ایزوله‌سازی کانتکست ایجنت‌ها (`Fork` و `MergeResult` با ۵ تست هم‌زمانی و race detector)** | کامل — Claude & Antigravity |
| **رشته گفتگوی مستقل هر ایجنت (`AgentConversationPanel` و رویدادهای `taskStepAs`)** | کامل — Claude & Antigravity |
| **اسکریپت تست خودکار end-to-end با وب‌سوکت و تلمتری (`scripts/e2e-test.ps1`)** | کامل — Claude & Antigravity |
| **الزام شواهد برای Factها و انقضای خودکار حافظه (بند ۱۰ سند ویژن)** | تازه — Antigravity |
| **چک‌پوینت Git روی ref جدا (`refs/nexusai/checkpoints`) و Accept Changes واقعی (بند ۲۲ سند ویژن)** | تازه — Antigravity |
| **Context Manifest & Budget و تمرکز کانتکست بر Active Task (بند ۱۷ و ۳۷ سند ویژن)** | تازه — Antigravity |
| **حل ریشه‌ای ضبط فایل‌های ابزارها و اجرای موازی چندمدلی تیم‌ها** | تازه — Claude & Antigravity |
| **حل ریشه‌ای دروازه اعتبارسنجی تسک‌ها (TaskVerifier و رانر تست فلاتر)** | تازه — Antigravity |
| **پشتیبانی کامل فلاتر در LoadFiles و لود دیسک در RunTeam** | تازه — Antigravity |
| **تایید کامل E2E موازی‌سازی با ۳ کارگر هم‌زمان و تولید کدهای واقعی Dart** | تازه — Antigravity |

---

## ⚠️ کارهای در دست و اولویت‌ها

### ۱. استریم لحظه‌ای توکن‌ها (Token Streaming over WebSocket)
نمایش زنده جریان کلمات تولیدی هر مدل روی Composer Panel در زمان واقعی.

### ۲. مدیریت هزینه‌ها و تفکیک هزینه/توکن به‌ازای هر Agent (بند ۳۷ سند ویژن)
ثبت و گزارش توکن‌های ورودی و خروجی مصرفی توسط هر ایجنت برای مشاهده‌ی هزینه و کارایی مدل‌ها.

---

## 🎯 کار بعدی
همکاری با Claude: شروع Priority 1 (استریم لحظه‌ای توکن‌ها) یا تفکیک توکن/هزینه ایجنت‌ها (بند ۳۷) با هماهنگی در `LIVE_BOARD.md` و رزرو در `IN_PROGRESS.md`.

---

## 📌 یادآوری‌ها

- `AGENTS.md` را قبل از هر کار بخوانید
- `IN_PROGRESS.md` را چک کنید تا فایل رزروشده نباشد
- کلید API هرگز در کد hardcode نشود