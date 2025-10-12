export const authFetch = async (url: string, options: RequestInit = {}): Promise<Response> => {
    const token = typeof window !== 'undefined' ? localStorage.getItem("token") : null;

    const headers = new Headers(options.headers || {});
    headers.set('Content-Type', 'application/json');

    if (token) {
        headers.set('Authorization', `Bearer ${token}`);
    }

    return fetch(url, {
        ...options,
        headers,
    });
};
