import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react';

export type Theme = 'dark' | 'light';
export type DAGRunsViewMode = 'list' | 'grouped';

export type UserPreferences = {
  pageLimit: number;
  theme: Theme;
  dagRunsViewMode: DAGRunsViewMode;
};

const UserPreferencesContext = createContext<{
  preferences: UserPreferences;
  updatePreference: <K extends keyof UserPreferences>(
    key: K,
    value: UserPreferences[K]
  ) => void;
  toggleTheme: () => void;
}>(null!);

export function UserPreferencesProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [preferences, setPreferences] = useState<UserPreferences>(() => {
    try {
      const saved = localStorage.getItem('user_preferences');
      const defaultPrefs: UserPreferences = {
        pageLimit: 50,
        theme: 'dark', // Default to dark theme
        dagRunsViewMode: 'list', // Default to list view
      };
      const prefs = saved
        ? { ...defaultPrefs, ...JSON.parse(saved) }
        : defaultPrefs;

      // Apply theme class immediately during initialization
      const root = document.documentElement;
      if (prefs.theme === 'dark') {
        root.classList.add('dark');
      } else {
        root.classList.remove('dark');
      }

      return prefs;
    } catch {
      // Fallback to defaults if parsing fails
      // Apply dark theme immediately
      document.documentElement.classList.add('dark');
      return {
        pageLimit: 50,
        theme: 'dark' as Theme,
        dagRunsViewMode: 'list' as DAGRunsViewMode,
      };
    }
  });

  // Apply theme class to document root
  useEffect(() => {
    const root = document.documentElement;
    if (preferences.theme === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }
  }, [preferences.theme]);

  const updatePreference = useCallback(
    <K extends keyof UserPreferences>(key: K, value: UserPreferences[K]) => {
      setPreferences((prev) => {
        const next = { ...prev, [key]: value };
        localStorage.setItem('user_preferences', JSON.stringify(next));
        return next;
      });
    },
    []
  );

  const toggleTheme = useCallback(() => {
    const newTheme = preferences.theme === 'dark' ? 'light' : 'dark';
    updatePreference('theme', newTheme);
  }, [preferences.theme, updatePreference]);

  return (
    <UserPreferencesContext.Provider
      value={{ preferences, updatePreference, toggleTheme }}
    >
      {children}
    </UserPreferencesContext.Provider>
  );
}

export function useUserPreferences() {
  return useContext(UserPreferencesContext);
}
