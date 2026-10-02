# فایل‌های در حال کار

> **قانون: اگر فایلی اینجا رزرو شده، دست به آن نمی‌زنید.**
> اگر فایلی که می‌خواهید تغییر دهید اینجاست، منتظر بمانید یا فایل دیگری بردارید.

---

## وضعیت فعلی

### 🔒 Claude — ۲۰۲۶-۱۰-۰۲ ۱۱:۳۵
- `backend/internal/workspace/validate.go` — فایل جدید
- `backend/internal/workspace/validate_test.go` — فایل جدید
- `backend/internal/workspace/workspace.go` — وصل کردن اعتبارسنجی

**کار:** جلوگیری از نوشتن متن به‌جای کد روی دیسک.
دلیل: در `data/workspaces/IDE/main.go` محتوای «User Safety: safe» نوشته شده بود.

### 🔒 Antigravity — ۲۰۲۶-۱۰-۰۲ ۱۱:۱۵
- `backend/internal/llm/openai_compat.go` — پشتیبانی از پاسخ‌های SSE استریم پروکسی‌های هوش مصنوعی (مثل ناین‌روتر)
- `backend/internal/agent/coder.go` — رفع خطای کامپایل متغیر structure (تشخیص داده شده در تست)
- `backend/internal/agent/agent.go` — تصحیح provider resolution در Discuss fallback (نکته ارسالی کلود در LIVE_BOARD)
- `frontend/src/components/composer/ComposerPanel.tsx` — بازطراحی UI تفکر پله‌به‌پله، گفتگوی مدل‌ها و خروجی تجمیعی نهایی برای کاربر
- `frontend/src/App.tsx` — اتصال رویدادهای تفکر و پیام اجماع نهایی

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