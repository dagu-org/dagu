import React, { createContext, useCallback, useContext, useState } from 'react';

export type DAGRunsViewMode = 'list' | 'grouped';

export type UserPreferences = {
  pageLimit: number;
  dagRunsViewMode: DAGRunsViewMode;
  logWrap: boolean;
  theme: 'light' | 'dark';
  safeMode: boolean;
};

const UserPreferencesContext = createContext<{
  preferences: UserPreferences;
  updatePreference: <K extends keyof UserPreferences>(
    key: K,
    value: UserPreferences[K]
  ) => void;
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
        dagRunsViewMode: 'list', // Default to list view
        logWrap: true, // Default to wrapped text
        theme: 'dark', // Default to dark theme
        safeMode: false, // Default to off (no approval dialogs)
      };
      return saved ? { ...defaultPrefs, ...JSON.parse(saved) } : defaultPrefs;
    } catch {
      // Fallback to defaults if parsing fails
      return {
        pageLimit: 50,
        dagRunsViewMode: 'list' as DAGRunsViewMode,
        logWrap: true,
        theme: 'dark',
        safeMode: false,
      };
    }
  });

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

  return (
    <UserPreferencesContext.Provider value={{ preferences, updatePreference }}>
      {children}
    </UserPreferencesContext.Provider>
  );
}

export function useUserPreferences() {
  return useContext(UserPreferencesContext);
}
