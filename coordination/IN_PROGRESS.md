# فایل‌های در حال کار

> **قانون: اگر فایلی اینجا رزرو شده، دست به آن نمی‌زنید.**
> اگر فایلی که می‌خواهید تغییر دهید اینجاست، منتظر بمانید یا فایل دیگری بردارید.
### ✅ Claude & Antigravity — ۲۰۲۶-۱۰-۰۴ ۰۸:۰۸ (تمام شد)
- `backend/internal/workspace/guard.go` & `guard_test.go` — ممانعت قطعی از خواندن فایل‌های اسرار (.env، کلیدها، توکن‌ها طبق بند ۲۵)
- `backend/internal/workspace/context.go` — رد کردن خودکار فایل‌های محرمانه در پیمایش Context Engine
- `backend/internal/llm/sanitize.go` & `sanitize_test.go` — ماژول جامع سانسور خودکار کلیدهای API و اسرار (RedactSecrets طبق بند ۲۵)
- `backend/internal/domain/team.go` — فیلدهای ReworkCount و LastError و متدهای AllDone / Completed / Settled
- `backend/internal/agent/agent.go` & `agent_test.go` — اعمال سقف سخت ۳ بار Rework + تشخیص تکرار خطا و ارتقا خودکار به WAITING_FOR_HUMAN (بند ۱۹)
- `backend/internal/orchestrator/progress.go` & `escalation_test.go` — گزارش جامع پایان کار و تست‌های کامل جلوگیری از بلاک شدن پلن
- `backend/api/ws_server.go` — سانسور خودکار اسرار قبل از برودکست وب‌سوکت

**نتیجه:** بندهای ۱۹ (سقف Rework و تشخیص تکرار خطا) و ۲۵ (حفاظت و سانسور اسرار) سند ویژن ۲.۰ به طور کامل پیاده‌سازی و تست شدند. تمام ۱۲ پکیج Go بک‌اند ۱۰۰٪ سبز هستند. باینری nexusd.exe کامپایل و روی پورت ۸۰۸۰ لایو است. تمام فایل‌ها آزاد هستند.

### ✅ Antigravity — ۲۰۲۶-۱۰-۰۴ ۰۷:۴۲ (تمام شد)
- `backend/internal/workspace/project_brain.go` — پشتیبانی ReadLedger از تمام ورودی‌های لجر (تغییر فایل و رویدادهای کامیت)
- `frontend/src/types.ts` — تعریف ChangeLedgerEntry و TestLedgerRecord در تایپ‌های فرانت‌اند
- `frontend/src/App.tsx` — ارسال projectId به کامپوننت Sidebar
- `frontend/src/components/layout/Sidebar.tsx` — اتصال زنده نمایش Change Ledger (§30) و تاریخچه کامیت‌ها به API پروژه

**نتیجه:** ویوی Source Control در سایدبار از حالت ماک استاتیک خارج شده و به Change Ledger زنده و تاریخچه کامیت‌های گیت متصل شد. تمام ۱۲ پکیج Go بک‌اند ۱۰۰٪ سبز، بیلد فرانت‌اند در ۲.۹ ثانیه موفق، دیمون nexusd روی پورت ۸۰۸۰ و سرور فرانت روی پورت ۳۰۰۰ لایو و در حال کارند. تمام فایل‌ها آزاد هستند.


### ✅ Antigravity — ۲۰۲۶-۱۰-۰۴ ۰۷:۲۸ (تمام شد)
- `backend/internal/orchestrator/team.go` — اضافه کردن map مدل‌ها (AgentModels) به sCtx در RunTeam
- `backend/internal/orchestrator/team_test.go` — تنظیم دقیق SetActiveTask در TestTeamFilesTouched
- `backend/internal/orchestrator/plan_loop_test.go` — استخراج صحیح TASK TITLE و شبیه‌سازی کدهای استاتیک برای تست استال (۱۰۰٪ سبز)

**نتیجه:** تمام تست‌های جدید ارکستریتور و پلن‌محور در چند ثانیه ۱۰۰٪ پاس شدند. تمام فایل‌ها آزاد هستند.

### ✅ Claude & Antigravity — ۲۰۲۶-۱۰-۰۴ ۰۷:۱۸ (تمام شد)
- `backend/internal/orchestrator/team.go` — اتصال کامل حلقه مبتنی بر پلن (Plan-Driven Loop)، ماشین وضعیت تسک‌ها، TaskVerifier، Change Ledger و تشخیص استال به RunTeam
- `backend/internal/agent/custom.go` — پیاده‌سازی CustomAgent.BuildPlan برای طراحی پلن تیم و تزریق تسک فعال (activeTaskBrief) به تمام اعضای تیم
- `backend/internal/agent/coder.go` — تزریق تسک فعال و لیست کارهای انجام‌شده قبلی به کانتکست ایجنت کدنویس
- `backend/internal/agent/critic.go` — تزریق activeTaskBrief به ایجنت منتقد برای ارزیابی اختصاصی تسک فعال و جلوگیری از خطای تسک‌های آینده
- `backend/internal/agent/security.go` — تزریق activeTaskBrief به ایجنت امنیت
- `backend/internal/agent/planner.go` & `planner_test.go` — اکسپورت فرمت پلن، NormalizePlan و ValidatePlan
- `backend/internal/orchestrator/team_test.go` — تست‌های پلن‌محور تیم‌های دلخواه (pickPlanner, teamFilesTouched, applyPlanToTeamVerdict)
- `backend/internal/orchestrator/plan_loop_test.go` — تست جامع ساخت پلن در تیم‌های دلخواه و راستی‌آزمایی با تسک‌ها

**نتیجه:** اتصال ۱۰۰٪ معماری Plan-Driven Loop به تیم‌های دلخواه (RunTeam) و ایزوله‌سازی کانتکست ایجنت‌ها برای کار اختصاصی روی تسک جاری. تمام ۱۲ پکیج بک‌اند ۱۰۰٪ سبز، بیلد فرانت‌اند تمیز و باینری nexusd.exe کامپایل و روی پورت ۸۰۸۰ در حال اجراست. تمام فایل‌ها آزاد هستند.

### ✅ Antigravity — ۲۰۲۶-۱۰-۰۳ ۲۳:۱۵ (تمام شد)
- `backend/internal/workspace/project_brain.go` — ساختار دقیق ChangeLedgerEntry بر اساس بند ۳۰ سند ویژن و متدهای RecordChange و ReadLedger
- `backend/internal/workspace/project_brain_test.go` — تست‌های ذخیره و خواندن Change Ledger در ledger.jsonl (۱۰۰٪ سبز)
- `backend/internal/orchestrator/loop.go` — اتصال خودکار persistChangeLedger پس از اعتبارسنجی و ثبت کامیت
- `backend/api/projects.go` — اندپوینت GET /api/projects/{id}/ledger برای دریافت تاریخچه تغییرات پروژه
- `backend/api/projects_test.go` — تست اندپوینت لجر پروژه (TestProjectsHandler_LedgerEndpoint)

**نتیجه:** لجر تغییرات تغییرناپذیر پروژه (Change Ledger طبق بند ۳۰ سند ویژن) به طور کامل پیاده‌سازی و تست شد. تمام ۱۲ پکیج بک‌اند ۱۰۰٪ سبز، بیلد فرانت‌اند تمیز و دیمون لایو روی پورت ۸۰۸۰. تمام فایل‌ها آزاد هستند.


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