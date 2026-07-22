import { useEffect, useState } from "react";
import { getStoredValue, useLocalStorageState } from "./useLocalStorageState";

export const THEME_MODES = ["light", "dark", "system"] as const;
export type ThemeMode = (typeof THEME_MODES)[number];

export function isThemeMode(value: string): value is ThemeMode {
  return (THEME_MODES as readonly string[]).includes(value);
}

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

function readStoredMode(raw: string | null): ThemeMode {
  if (raw !== null && isThemeMode(raw)) {
    return raw;
  }

  // Migrate a previously persisted boolean preference, if present.
  const legacy = getStoredValue(LEGACY_DARK_MODE_STORAGE_KEY);
  if (legacy === "true") return "dark";
  if (legacy === "false") return "light";

  return "system";
}

export function useThemePreference() {
  const [themeMode, setThemeMode] = useLocalStorageState<ThemeMode>(
    THEME_MODE_STORAGE_KEY,
    readStoredMode,
  );
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
    document.documentElement.classList.toggle("dark", isDarkMode);
  }, [isDarkMode]);

  return { themeMode, isDarkMode, setThemeMode };
}
