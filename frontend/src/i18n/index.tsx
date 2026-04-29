import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { defaultLocale, localeStorageKey, messages, type Locale } from './messages'

type TranslateValues = Record<string, string | number>

type I18nContextValue = {
  locale: Locale
  setLocale: (locale: Locale) => void
  toggleLocale: () => void
  t: (key: string, values?: TranslateValues) => string
}

const I18nContext = createContext<I18nContextValue | null>(null)
const warnedMissingKeys = new Set<string>()

function resolveInitialLocale(): Locale {
  if (typeof window === 'undefined') {
    return defaultLocale
  }

  const saved = window.localStorage.getItem(localeStorageKey)
  if (saved === 'zh-CN' || saved === 'en-US') {
    return saved
  }

  const language = window.navigator.language
  if (language.toLowerCase().startsWith('zh')) {
    return 'zh-CN'
  }

  return 'en-US'
}

function interpolate(template: string, values?: TranslateValues) {
  if (!values) {
    return template
  }

  return template.replace(/\{\{\s*(\w+)\s*\}\}/g, (_, key: string) => String(values[key] ?? ''))
}

function warnMissingTranslation(locale: Locale, key: string) {
  if (!import.meta.env.DEV || warnedMissingKeys.has(`${locale}:${key}`)) {
    return
  }

  warnedMissingKeys.add(`${locale}:${key}`)
  console.warn(`[i18n] Missing translation for key "${key}" in locale "${locale}"`)
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(resolveInitialLocale)

  useEffect(() => {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(localeStorageKey, locale)
      document.documentElement.lang = locale
    }
  }, [locale])

  const value = useMemo<I18nContextValue>(() => ({
    locale,
    setLocale: (nextLocale) => setLocaleState(nextLocale),
    toggleLocale: () => setLocaleState((prev) => (prev === 'zh-CN' ? 'en-US' : 'zh-CN')),
    t: (key, values) => {
      const dictionary = messages[locale]
      const fallback = messages[defaultLocale]
      const text = dictionary[key] ?? fallback[key]

      if (!text) {
        warnMissingTranslation(locale, key)
        return key
      }

      return interpolate(text, values)
    },
  }), [locale])

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const context = useContext(I18nContext)
  if (!context) {
    throw new Error('useI18n must be used within I18nProvider')
  }

  return context
}

export type { Locale }
