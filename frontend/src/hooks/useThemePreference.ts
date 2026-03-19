import { useCallback, useEffect, useState } from "react"

const DARK_MODE_STORAGE_KEY = "launchpad:isDarkMode"

export function useThemePreference() {
  const [isDarkMode, setIsDarkMode] = useState<boolean>(() => {
    try {
      const stored = localStorage.getItem(DARK_MODE_STORAGE_KEY)
      if (stored !== null) {
        return stored === "true"
      }

      return window.matchMedia("(prefers-color-scheme: dark)").matches
    } catch {
      return false
    }
  })

  useEffect(() => {
    try {
      localStorage.setItem(DARK_MODE_STORAGE_KEY, String(isDarkMode))
    } catch {
      console.error("Failed to persist dark mode preference")
    }
  }, [isDarkMode])

  useEffect(() => {
    document.documentElement.classList.toggle("dark", isDarkMode)
  }, [isDarkMode])

  const toggleTheme = useCallback(() => {
    setIsDarkMode((prev) => !prev)
  }, [])

  return { isDarkMode, toggleTheme }
}
