import { useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react'

type FieldName = 'name' | 'company' | 'email' | 'phone'
type FormValues = Record<FieldName, string> & { demand: string }
type FormErrors = Partial<Record<keyof FormValues, string>>

type FieldMeta = {
  label: string; placeholder: string; type: 'text' | 'email' | 'tel'
  required?: boolean; inputMode?: 'numeric'
}

const EMPTY_VALUES: FormValues = { name: '', company: '', email: '', phone: '', demand: '' }

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/
const PHONE_RE = /^1[3-9]\d{9}$/

const FIELDS: Record<FieldName, FieldMeta> = {
  name: { label: '姓名', placeholder: '请输入姓名', type: 'text', required: true },
  company: { label: '公司名称', placeholder: '请输入公司名称', type: 'text' },
  email: { label: '邮箱', placeholder: 'name@company.com', type: 'email', required: true },
  phone: { label: '联系电话', placeholder: '11 位手机号', type: 'tel', inputMode: 'numeric' },
}

/** 表单按行分组：每行两个字段，窄屏下自动堆叠 */
const FIELD_ROWS: FieldName[][] = [
  ['name', 'company'],
  ['email', 'phone'],
]

const svgProps = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.7,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

const INFO_CARDS: { title: string; lines: string[]; icon: ReactNode }[] = [
  {
    title: '产品咨询',
    lines: ['sales@boticz.com'],
    icon: <svg viewBox="0 0 24 24" {...svgProps} aria-hidden="true"><rect x="3" y="5" width="18" height="14" rx="2" /><path d="m3.8 6.6 8.2 6 8.2-6" /></svg>,
  },
  {
    title: '技术支持',
    lines: ['support@boticz.com', '400-000-0000'],
    icon: <svg viewBox="0 0 24 24" {...svgProps} aria-hidden="true"><path d="M5 14v-2a7 7 0 0 1 14 0v2" /><rect x="3" y="13" width="4" height="6" rx="1.6" /><rect x="17" y="13" width="4" height="6" rx="1.6" /><path d="M19 19v.8a2.4 2.4 0 0 1-2.4 2.4H13" /></svg>,
  },
  {
    title: '服务时间',
    lines: ['周一至周五 09:00 - 18:00', '紧急故障 7×24 小时响应'],
    icon: <svg viewBox="0 0 24 24" {...svgProps} aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M12 7.4V12l3.2 2" /></svg>,
  },
]

/** 校验规则：姓名与邮箱必填，邮箱与手机号做格式校验 */
function validate(values: FormValues): FormErrors {
  const errors: FormErrors = {}
  const email = values.email.trim()
  const phone = values.phone.trim()

  if (!values.name.trim()) {
    errors.name = '请填写您的姓名'
  }

  if (!email) {
    errors.email = '请填写邮箱地址'
  } else if (!EMAIL_RE.test(email)) {
    errors.email = '邮箱格式不正确'
  }

  if (phone && !PHONE_RE.test(phone)) {
    errors.phone = '请填写 11 位手机号'
  }

  return errors
}

/** 联系我们：信息卡 + 预约演示表单（本地校验，仅展示成功态，不做真实发送） */
export function ContactSection() {
  const [values, setValues] = useState<FormValues>(EMPTY_VALUES)
  const [errors, setErrors] = useState<FormErrors>({})
  const [submitted, setSubmitted] = useState(false)

  const handleChange = (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = event.target
    setValues((prev) => ({ ...prev, [name]: value }))
    setErrors((prev) => (prev[name as keyof FormValues] ? { ...prev, [name]: undefined } : prev))
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()

    const nextErrors = validate(values)
    setErrors(nextErrors)

    if (Object.keys(nextErrors).length > 0) {
      return
    }

    // TODO: 接入飞书 WebHook —— 将 values 提交到后端接口（如 POST /web/website/demo-request），
    // 由后端转发至飞书群机器人（WebHook 地址存放于后端配置，避免在前端暴露）。
    setSubmitted(true)
  }

  const handleReset = () => {
    setValues(EMPTY_VALUES)
    setErrors({})
    setSubmitted(false)
  }

  const renderField = (name: FieldName) => {
    const meta = FIELDS[name]
    const fieldId = `demo-${name}`

    return (
      <div className="form-field" key={name}>
        <label className="form-label" htmlFor={fieldId}>
          {meta.label}
          {meta.required ? <span className="req">*</span> : null}
        </label>
        <input
          id={fieldId} name={name} type={meta.type} inputMode={meta.inputMode}
          className={`form-input${errors[name] ? ' has-error' : ''}`} placeholder={meta.placeholder}
          value={values[name]} onChange={handleChange} required={meta.required}
        />
        {errors[name] ? <div className="form-error">{errors[name]}</div> : null}
      </div>
    )
  }

  return (
    <section className="section section--alt" id="contact">
      <div className="container">
        <div className="section-head reveal">
          <span className="section-eyebrow">联系我们</span>
          <h2 className="section-title">预约一次平台演示</h2>
          <p className="section-subtitle">
            留下联系方式与现场监管需求，我们会结合您的区域划分与风险类型安排演示。
          </p>
        </div>

        <div className="contact-grid">
          <div className="contact-info reveal">
            {INFO_CARDS.map((card) => (
              <div className="info-card" key={card.title}>
                <span className="info-icon">{card.icon}</span>
                <div>
                  <div className="info-title">{card.title}</div>
                  <div className="info-text">
                    {card.lines.map((line) => (<div key={line}>{line}</div>))}
                  </div>
                </div>
              </div>
            ))}
          </div>

          <div className="form-card reveal">
            {submitted ? (
              <div className="form-success" role="status">
                <span className="form-success-icon">
                  <svg viewBox="0 0 24 24" {...svgProps} strokeWidth={2} aria-hidden="true"><path d="m5 12.6 4.6 4.6L19 7.8" /></svg>
                </span>
                <h3 className="form-success-title">预约申请已提交</h3>
                <p className="form-success-text">
                  感谢您的关注，我们会在 1 个工作日内与您联系确认演示安排。如需加急，可直接致电技术支持热线。
                </p>
                <button type="button" className="btn btn-ghost btn-sm" onClick={handleReset}>再次填写</button>
              </div>
            ) : (
              <form onSubmit={handleSubmit} noValidate>
                <h3 className="form-title">预约演示</h3>
                <p className="form-hint">带 * 为必填项，我们仅将信息用于本次演示沟通。</p>

                {FIELD_ROWS.map((row) => (
                  <div className="form-row" key={row.join('-')}>{row.map(renderField)}</div>
                ))}

                <div className="form-field">
                  <label className="form-label" htmlFor="demo-demand">需求描述</label>
                  <textarea
                    id="demo-demand" name="demand" className="form-textarea"
                    placeholder="例如：工地 20 路摄像头，重点关注安全帽佩戴与区域入侵"
                    value={values.demand} onChange={handleChange}
                  />
                </div>

                <button type="submit" className="btn btn-primary btn-block form-submit">提交预约</button>
                <p className="form-footnote">提交即表示同意我们通过上述方式与您联系。</p>
              </form>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}

export default ContactSection
