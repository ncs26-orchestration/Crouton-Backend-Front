const KEY = "aios.token";
export const authStore = {
    get: () => localStorage.getItem(KEY),
    set: (t) => localStorage.setItem(KEY, t),
    clear: () => localStorage.removeItem(KEY),
};
