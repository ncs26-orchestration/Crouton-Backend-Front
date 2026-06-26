import { createContext, useContext, useState, useEffect } from "react";
import { authStore } from "../lib/auth";
const AuthContext = createContext(null);
function decodeJwtPayload(token) {
    try {
        const parts = token.split(".");
        if (parts.length < 2)
            return null;
        const segment = parts[1];
        if (!segment)
            return null;
        // Pad base64 string if needed
        const padded = segment + "=".repeat((4 - (segment.length % 4)) % 4);
        const decoded = atob(padded);
        return JSON.parse(decoded);
    }
    catch {
        return null;
    }
}
export function AuthProvider({ children }) {
    const [user, setUser] = useState(null);
    const [token, setToken] = useState(null);
    useEffect(() => {
        const stored = authStore.get();
        if (stored) {
            const payload = decodeJwtPayload(stored);
            if (payload && payload.uid != null && payload.email && payload.name) {
                setToken(stored);
                setUser({ id: payload.uid, email: payload.email, name: payload.name });
            }
            else {
                authStore.clear();
            }
        }
    }, []);
    const login = (newToken, newUser) => {
        authStore.set(newToken);
        setToken(newToken);
        setUser(newUser);
    };
    const logout = () => {
        authStore.clear();
        setToken(null);
        setUser(null);
    };
    return (<AuthContext.Provider value={{ user, token, login, logout }}>
      {children}
    </AuthContext.Provider>);
}
// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
    const ctx = useContext(AuthContext);
    if (!ctx)
        throw new Error("useAuth must be used within AuthProvider");
    return ctx;
}
