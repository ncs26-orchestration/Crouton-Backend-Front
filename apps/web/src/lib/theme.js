import { createContext, useContext, useEffect, useState } from "react";
const Ctx = createContext(null);
const STORAGE_KEY = "aios-theme";
export function ThemeProvider({ children }) {
    const [theme, setTheme] = useState(() => {
        if (typeof window === "undefined")
            return "dark";
        const saved = window.localStorage.getItem(STORAGE_KEY);
        if (saved === "light" || saved === "dark")
            return saved;
        return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    });
    useEffect(() => {
        const root = document.documentElement;
        if (theme === "dark")
            root.classList.add("dark");
        else
            root.classList.remove("dark");
        window.localStorage.setItem(STORAGE_KEY, theme);
    }, [theme]);
    return (<Ctx.Provider value={{
            theme,
            setTheme,
            toggle: () => setTheme((t) => (t === "light" ? "dark" : "light")),
        }}>
      {children}
    </Ctx.Provider>);
}
export function useTheme() {
    const ctx = useContext(Ctx);
    if (!ctx)
        throw new Error("useTheme must be used inside <ThemeProvider>");
    return ctx;
}
