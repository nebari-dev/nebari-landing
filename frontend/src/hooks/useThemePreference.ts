import { useCallback, useEffect, useState } from "react";

export type ThemeMode = "light" | "dark" | "system";

const THEME_MODE_STORAGE_KEY = "launchpad:themeMode";
// Legacy boolean key kept only for one-time migration of existing users.
const LEGACY_DARK_MODE_STORAGE_KEY = "launchpad:isDarkMode";

function prefersDark(): boolean {
  try {
    return window.matchMedia("(prefers-color-scheme: dark)").matches;
  } catch {
    return false;
  }
}

function readStoredMode(): ThemeMode {
  try {
    const stored = localStorage.getItem(THEME_MODE_STORAGE_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") {
      return stored;
    }

    // Migrate a previously persisted boolean preference, if present.
    const legacy = localStorage.getItem(LEGACY_DARK_MODE_STORAGE_KEY);
    if (legacy === "true") return "dark";
    if (legacy === "false") return "light";
  } catch {
    // Ignore storage access failures and fall back to system.
  }

  return "system";
}

export function useThemePreference() {
  const [themeMode, setThemeModeState] = useState<ThemeMode>(readStoredMode);
  const [systemPrefersDark, setSystemPrefersDark] = useState<boolean>(prefersDark);

  // Keep "system" mode in sync with the OS preference as it changes.
  useEffect(() => {
    let mediaQuery: MediaQueryList;
    try {
      mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    } catch {
      return;
    }

    const onChange = (event: MediaQueryListEvent) => setSystemPrefersDark(event.matches);
    mediaQuery.addEventListener("change", onChange);
    return () => mediaQuery.removeEventListener("change", onChange);
  }, []);

  const isDarkMode = themeMode === "system" ? systemPrefersDark : themeMode === "dark";

  useEffect(() => {
    try {
      localStorage.setItem(THEME_MODE_STORAGE_KEY, themeMode);
    } catch {
      console.error("Failed to persist theme preference");
    }
  }, [themeMode]);

  useEffect(() => {
    document.documentElement.classList.toggle("dark", isDarkMode);
  }, [isDarkMode]);

  const setThemeMode = useCallback((mode: ThemeMode) => {
    setThemeModeState(mode);
  }, []);

  return { themeMode, isDarkMode, setThemeMode };
}
