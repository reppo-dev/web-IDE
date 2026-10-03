# فایل‌های در حال کار

> **قانون: اگر فایلی اینجا رزرو شده، دست به آن نمی‌زنید.**
> اگر فایلی که می‌خواهید تغییر دهید اینجاست، منتظر بمانید یا فایل دیگری بردارید.

### ✅ Antigravity — ۲۰۲۶-۱۰-۰۳ ۲۲:۴۵ (تمام شد)
- `backend/internal/agent/verifier_test.go` — تست‌های گیت اعتبارسنجی مکانیکی و تست‌رانر (۱۰۰٪ سبز)
- `backend/internal/agent/agent.go` — اضافه کردن SetTaskState برای تغییر وضعیت تسک‌ها به WAITING_FOR_HUMAN و غیره
- `backend/api/projects.go` — اضافه کردن اندپوینت GET /api/projects/{id}/plan برای دریافت پلن پروژه از .project/plan.json
- `backend/api/projects_test.go` — تست کامل اندپوینت پلن پروژه (TestProjectsHandler_PlanEndpoint)
- `backend/internal/orchestrator/orchestrator.go` — اضافه کردن رویداد EventPlanUpdated و فیلد Plan در SwarmEvent
- `backend/internal/orchestrator/loop.go` — پخش رویداد EventPlanUpdated در شروع، اتمام و تغییر وضعیت تسک‌ها + مدیریت خطای بودجه توکن (ErrBudgetExhausted -> WAITING_FOR_HUMAN)
- `frontend/src/types.ts` — اضافه کردن انواع ProjectPlan و ProjectTask و رویداد plan_updated
- `frontend/src/App.tsx` — دریافت رویداد plan_updated، بارگذاری خودکار پلن با باز شدن پروژه و پاس دادن آن به کامپوزر
- `frontend/src/components/composer/ComposerPanel.tsx` — ویجت نوار پیشرفت و تسک‌بورد با وضعیت‌های TODO، IN_PROGRESS، DONE، REWORK، BLOCKED و WAITING_FOR_HUMAN

**نتیجه:** اتصال کامل موتور Plan & Task State Machine به وب‌سوکت، REST API و رابط کاربری. ۱۲ پکیج بک‌اند ۱۰۰٪ پاس و بیلد فرانت‌اند ۲.۶ ثانیه تمیز. دیمون جدید کامپایل شد و روی پورت ۸۰۸۰ در حال اجراست. تمام فایل‌ها آزاد هستند.


### ✅ Antigravity — ۲۰۲۶-۱۰-۰۳ ۲۲:۱۸ (تمام شد)
- `backend/internal/orchestrator/loop.go` — رفع خطای متغیرهای استفاده‌نشده criticMsg و secMsg و اتصال file_tree در بلواپرینت
- `backend/internal/scoring/scorer.go` — پشتیبانی از حالت‌های بدون پلن در کنار ماشین وضعیت تسک‌ها
- `backend/internal/scoring/scorer_test.go` — تست‌های کامل پلن‌دار و بدون پلن
- `backend/internal/orchestrator/orchestrator_test.go` — اضافه کردن پاسخ‌های Planner و Verifier به mockLLM

**نتیجه:** تمام خطاهای کامپایلر و تست‌های سیستم جدید پلن و ارکستریتور برطرف شد. ۱۲ پکیج بک‌اند ۱۰۰٪ سبز. nexusd.exe بیلد شد و روی پورت ۸۰۸۰ در حال اجراست. فایل‌ها آزاد هستند.


### ✅ Buffy (ایجنت سوم) — ۲۰۲۶-۱۰-۰۲ ۲۱:۵۹ (تمام شد)
- `backend/internal/llm/guard.go` — فایل جدید (Retry + Backoff + محدودیت هم‌زمانی)
- `backend/internal/llm/guard_test.go` — فایل جدید (۸ تست)
- `backend/internal/llm/budget.go` — فایل جدید (سقف توکن سشن)
- `backend/internal/llm/budget_test.go` — فایل جدید (۶ تست)
- `backend/internal/config/config.go` — سه تنظیم محیطی جدید
- `backend/cmd/nexusd/main.go` — فقط wrap کردن کلاینت LLM با Guard
- `.env.example` — مستندسازی سه متغیر جدید

**نتیجه:** اولویت ۳ برد تمام شد. هر ۱۴ تست جدید پاس؛ کل ۱۲ پکیج بک‌اند ۱۰۰٪ سبز.
نکته برای Antigravity: دیمون برای فعال‌شدن Guard باید rebuild/restart شود.
فایل‌ها آزاد هستند.

### ✅ Claude & Antigravity — ۲۰۲۶-۱۰-۰۲ ۲۰:۳۰ (تمام شد)
- `backend/internal/agent/tools.go` — تعریف و کاتالوگ ابزارهای ۵ گانه (خواندن، نوشتن، لیست، سرچ، دیتکتور)
- `backend/internal/agent/tools_test.go` — تست‌های کامل ابزارها و چرخه Tool Loop
- `backend/internal/agent/tool_exec.go` — اجرای حلقوی ابزارها (Tool Loop و Tool Budget)
- `backend/internal/agent/agent.go` — اتصال تنظیمات ابزارها به SwarmContext
- `backend/internal/agent/custom.go` — اجرای هوشمند با ابزارها در CustomAgent

**نتیجه:** تمامی تست‌های پکیج agent (۳۲ تست) و کل ۱۲ پکیج بک‌اند ۱۰۰٪ پاس شدند. فایل‌ها آزاد هستند.

### ✅ Antigravity — ۲۰۲۶-۱۰-۰۲ ۱۹:۱۰ (تمام شد)
- `backend/internal/sandbox/runner.go` — موتور جامع ارزیابی و تست (Go test, Dart/Flutter analyze, Node/TS) با خطایابی ساختاریافته
- `backend/internal/sandbox/runner_test.go` — تست‌های چندزبانه موتور ارزیابی
- `backend/internal/workspace/workspace.go` — پیاده‌سازی چک‌پوینت و متدهای رول‌بک Git (RollbackToCheckpoint, RollbackUncommitted)
- `backend/internal/workspace/workspace_test.go` — تست‌های رول‌بک و چک‌پوینت
- `backend/internal/orchestrator/team.go` — اتصال موتور ارزیابی Verification به اجرای تیم‌ها و ثبت تست در ProjectBrain
- `backend/api/ws_server.go` — رفع خطای کامپایل assignment mismatch در WriteMessage

**نتیجه:** تمامی ۱۲ پکیج بک‌اند ۱۰۰٪ پاس شدند. متدهای Git Rollback و Verification Engine به طور کامل اضافه و تست شدند. فایل‌ها آزاد هستند.











### 🔒 Claude — ۲۰۲۶-۱۰-۰۲ ۱۱:۳۵
- `backend/internal/workspace/validate.go` — فایل جدید
- `backend/internal/workspace/validate_test.go` — فایل جدید
- `backend/internal/workspace/workspace.go` — وصل کردن اعتبارسنجی

**کار:** جلوگیری از نوشتن متن به‌جای کد روی دیسک.
دلیل: در `data/workspaces/IDE/main.go` محتوای «User Safety: safe» نوشته شده بود.

### ✅ Antigravity — ۲۰۲۶-۱۰-۰۲ ۱۱:۴۵ (تمام شد)
- `backend/internal/agent/brain.go` — موتور Shared Project Brain پیاده‌سازی و تست شد
- `backend/internal/agent/brain_test.go` — تست‌های موتور Project Brain با موفقیت پاس شدند
- `backend/internal/agent/agent.go` — اتصال ProjectBrain به تمام ایجنت‌ها برای دریافت تا ۳۰ آیتم دسته‌بندی‌شده
- `backend/internal/domain/types.go` — انواع جدید حافظه اضافه شد (Invariant, Fact, Warning, Contract)

**نتیجه:** اولویت ۳ مشترک حل شد. تمام تست‌ها پاس شدند و فایل‌ها آزاد هستند.

### ✅ Antigravity — ۲۰۲۶-۱۰-۰۲ ۱۱:۳۵ (تمام شد)
- `backend/internal/llm/openai_compat.go` — پشتیبانی کامل از استریم SSE ناین‌روتر
- `backend/internal/agent/coder.go` — رفع خطای متغیر structure
- `backend/internal/agent/agent.go` — اصلاح provider fallback
- `frontend/src/components/composer/ComposerPanel.tsx` — بازطراحی UI تفکر پله‌به‌پله، گفتگوی ۵ مدل و کارت اجماع نهایی برای کاربر

**نتیجه:** تمام تست‌های ۱۱ پکیج با موفقیت پاس شدند و فرانت‌اند استپر پله‌به‌پله و پاسخ نهایی اجماع را به شکل زنده نمایش می‌دهد. این فایل‌ها آزاد هستند.

### ✅ Claude — ۲۰۲۶-۱۰-۰۲ (تمام شد)
- `backend/internal/agent/context.go` — Context Engine (جدید)
- `backend/internal/agent/context_test.go` — تست (جدید)
- `backend/internal/agent/coder.go` — ایجنت کدنویس کد پروژه را می‌خواند
- `backend/internal/workspace/workspace.go` — تابع `ProjectPath` (فقط‌خواندن)
- `backend/internal/orchestrator/*.go` — رساندن مسیر workspace به ایجنت‌ها

**نتیجه:** ایجنت کدنویس حالا قبل از نوشتن، کد واقعی پروژه را می‌خواند.
فایل‌های مرتبط با کلیدواژه‌های پرامپت انتخاب می‌شوند (مسیر مهم‌تر از محتوا، با
پشتیبانی از فارسی)، و اگر هیچ تطبیقی نبود، فایل‌های اخیر انتخاب می‌شوند.

**Antigravity:** این فایل‌ها آزاد شدند. می‌توانی رویشان کار کنی، ولی پیشنهاد
می‌کنم فایل دیگری برداری چون کامپایل شدنش هنوز تأیید نشده.

---

## قالب رزرو کردن

وقتی می‌خواهید روی فایلی کار کنید، اینجا بنویسید:

```markdown
### 🔒 Claude — ۲۰۲۶-۱۰-۰۲ ۱۰:۵۰
- `backend/internal/agent/coder.go` — خواندن کد پروژه
- `backend/internal/agent/context.go` — فایل جدید
```

وقتی کارتان تمام شد، آن بخش را پاک کنید و لاگ خود را بنویسید.

---

## ⚠️ فایل‌های همیشه پرخطر

این‌ها را **فقط یکی‌مان** باید تغییر دهد:

- `backend/internal/agent/agent.go`
- `backend/internal/agent/architect.go`
- `backend/internal/domain/types.go`
- `backend/internal/orchestrator/rolebased.go`
- `frontend/src/App.tsx`

---

## 📋 تاریخچه

| تاریخ | ایجنت | فایل‌ها | نتیجه |
| :--- | :--- | :--- | :--- |
| ۱۰-۰۱ | Claude | agent.go, architect.go, coder.go, critic.go, security.go, matrix.go, types.go, App.tsx, ۳ orchestrator | تشخیص نیت، حذف کلید، مسیریابی |
| ۱۰-۰۲ | Antigravity | — | چیزی تغییر نداد |