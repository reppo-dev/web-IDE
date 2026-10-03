# 📜 لاگ Buffy — ایجنت سوم

> ایجنت سوم تیم (Buffy / Codebuff). هر کاری که می‌کنم اینجا ثبت می‌شود.

---

## ۲۰۲۶-۱۰-۰۲ — تست زنده‌ی کامل + رفع باگ Guard (ساعت ۲۲:۳۵ تا ۲۲:۴۲)

### اجرای پروژه در دو سمت
- بک‌اند: `go vet` + `go test ./internal/... ./api/...` — ۱۲ پکیج ۱۰۰٪ پاس؛ `nexusd.exe` بیلد و روی ۸۰۸۰ اجرا (یک نمونه‌ی قدیمی روی پورت بود که kill کردم تا Guard فعال باشد).
- فرانت‌اند: `npm run build` (tsc strict + vite) تمیز؛ dev server روی ۳۰۰۰.
- تست زنده در مرورگر: UI رندر شد، WebSocket وصل، `/api/projects` → 200، کنسول تمیز.

### تست دستی از خود UI (طبق TESTING.md بخش ۵)
پیام «سلام» را از Composer فرستادم. اول لازم بود برای هر ۵ نقش مدل انتخاب شود (رفتار عمدی) — پریست NineRouter را زدم و پنج مدل `nemo_qwen / mimo / bunny / nemotron / flash_5` ست شدند.

### 🐛 باگی که همین تست زنده پیدا کرد
اجرای اول این‌طور شکست خورد:
```
role-based [phase 1]: architect failed: intent classification failed:
llm returned error: Upstream error from Nvidia: Service temporarily overloaded
```
خطای **گذرا** بود ولی Guard retry نکرد. علتش: NineRouter این خطا را با **HTTP 200 + بدنه‌ی error** برمی‌گرداند، نه ۵۰۳، و `IsRetryable` من فقط `api error (status ...)` و `api request failed:` را می‌شناخت.

**رفع:** طبقه‌بندی نشانه‌های گذرا در بدنه‌ی خطای provider (overload/temporarily/rate limit/timeout/try again/upstream error/busy/throttl)، با لیست permanent که اولویت دارد (invalid api key / unauthorized / model not found / context length) تا خطای دائمی اشتباه retry نشود.

**تست‌ها:** `TestGuardRetriesUpstreamOverloadWithHTTP200` (بازسازی همین رخداد واقعی) و `TestIsRetryableClassification` با ۱۴ حالت.

### نتیجه‌ی اجرای دوباره
```
22:39:46 WS RECEIVED start_swarm: prompt=سلام
22:40:26 SWARM FINISHED, err=<nil>
```
- گفتگوی ۵ مرحله‌ای کامل با پاسخ واقعی فارسی از هر ۵ مدل
- کارت اجماع نهایی: «سلام! خوشحالم 😊 ... چطور می‌تونم کمکتون کنم؟»
- ✅ **هیچ فایلی ساخته نشد** (workspace فقط README.md، بدون کامیت جدید) ⇒ تشخیص نیت درست کار می‌کند

### درس
تست واحد کافی نبود؛ فقط اجرای واقعی نشان داد providerها خطای گذرا را به شکل‌های غیرمنتظره برمی‌گردانند. این دقیقاً همان چیزی است که سند چشم‌انداز در بخش ۳۸ (ارزیابی روی کار واقعی) می‌گوید.

---

## ۲۰۲۶-۱۰-۰۲ — اولویت ۳ برد: LLM Guard (Retry + Concurrency + Budget)

### چرا این کار؟

کاربر سند `PROJECT_VISION.md` نسخه ۲ را داد و گفت به تیم بپیوندم. در برد،
**اولویت ۳** باز بود: «مکانیزم Retry و مدیریت همزمانی NineRouter» — مشکل
۵ ریکوئست حجیم هم‌زمان به ناین‌روتر که ۵۰۳ و timeout می‌گیرند. من آن را
بردم و چون بخش ۲۶ سند چشم‌انداز (Budget و Limit) الزامی است، یک لایه‌ی
بودجه هم اضافه کردم.

### چه ساختم؟

یک **Guard** در `backend/internal/llm/` که چون `Client` interface را
پیاده می‌کند، به‌صورت drop-in در `main.go` جلوی کلاینت می‌نشیند — یعنی
**هیچ کدام از ۸ نقطه‌ی فراخوانی ایجنت‌ها تغییر نکرد**.

| جزء | کار | پیش‌فرض |
| :--- | :--- | :--- |
| Concurrency Limiter | حداکثر N درخواست هم‌زمان به provider؛ بقیه در صف (با احترام به cancel) | ۲ (`NEXUS_LLM_MAX_CONCURRENT`) |
| Retry | خطاهای گذرا (شبکه، ۴۲۹، ۵۰۰/۵۰۲/۵۰۳/۵۰۴) با backoff نمایی + jitter کامل؛ ۴۰۰/۴۰۱/۴۰۳ و cancel هرگز retry نمی‌شوند | ۳ تلاش (`NEXUS_LLM_MAX_ATTEMPTS`) |
| Token Budget | سقف توکن سشن؛ هشدار یک‌باره در ۸۰٪، بلاک فراخوانی‌های بعدی با `ErrBudgetExhausted` در ۱۰۰٪ | بی‌نهایت (`NEXUS_MAX_TOTAL_TOKENS=0`) |

### تصمیم‌های طراحی

1. **Retry در سطح interface، نه HTTP.** `Client.Generate` فقط error برمی‌گرداند،
   پس `IsRetryable` روی رشته‌ی خطاهای این پکیج (`api error (status N)` و
   `api request failed:`) طبقه‌بندی می‌کند. این وابستگی به فرمت خطاهاست —
   اگر روزی `openai_compat.go` typed error بدهد، `IsRetryable` باید ساده‌تر شود.
2. **Budget قبل از فراخوانی چک و بعد از موفقیت ثبت می‌شود.** فراخوانی‌ای که
   سقف را رد کند کامل می‌شود (هزینه‌اش را نمی‌شود پس گرفت) ولی بعدی بلاک
   می‌شود — مطابق «۸۰٪ هشدار، ۱۰۰٪ توقف» بخش ۲۶.
3. **هیچ فایل پرخطری دست نخورد.** فقط `config.go` و `main.go` (کم‌خطر) +
   فایل‌های جدید llm.

### تست‌ها — ۱۴ عدد، همه سبز

- `TestGuardRetriesTransientErrors` — دو بار ۵۰۳، بار سوم موفق؛ دقیقاً ۳ فراخوانی
- `TestGuardDoesNotRetryPermanentErrors` — ۴۰۱ فقط ۱ فراخوانی
- `TestGuardGivesUpAfterMaxAttempts` — ۴۲۹ همیشگی = دقیقاً MaxAttempts فراخوانی
- `TestGuardLimitsConcurrentRequests` — ۶ درخواست موازی، اوج هم‌زمانی روی سرور = ۲
- `TestGuardQueueHonorsContext` — صف با expire شدن context ترک می‌شود، سریع
- `TestGuardDoesNotRetryCanceledContext` — cancel ⇒ بدون retry
- `TestGuardBudgetBlocksNextCallAfterExhaustion` — بعد ۱۰۰/۱۰۰، فراخوانی دوم بلاک و به provider نمی‌رسد
- `TestGuardRecordsBudgetAndWarns` — هشدار ۸۰٪ یک‌بار
- + ۶ تست واحد Budget (آستانه، اتمام، بی‌نهایت، nil-safe، هم‌زمانی، منفی‌ها)

### نتیجه‌ی کل مخزن

`go build ./cmd/... ./internal/... ./api/...` تمیز، `go vet` تمیز، و
`go test -count=1 ./internal/... ./api/...` — **هر ۱۲ پکیج ۱۰۰٪ پاس**.

### یادداشت‌ها برای تیم

- ⚠️ دیمون `nexusd.exe` باید rebuild/restart شود تا Guard فعال شود
  (اگر `NEXUS_MAX_TOTAL_TOKENS` ست نباشد، بودجه بی‌نهایت است — رفتار فعلی عوض نمی‌شود).
- 🐛 `backend/data/workspaces/my-nexus-project/main.go` (محصول اجرای دیمون)
  کد Go خرابی دارد و `go build ./...` کل مخزن را می‌شکند (تست ما روی
  `./internal/... ./api/...` بود و سبز). پیشنهاد: یا پاک شود یا `data/`
  در build نادیده گرفته شود.
- 🔜 پیشنهاد قدم بعدی از سوی من: وصل کردن `ErrBudgetExhausted` به
  `WAITING_FOR_HUMAN` در Orchestrator (بخش ۲۶) + ثبت snapshot بودجه در
  گزارش‌ها (بخش ۳۷).
