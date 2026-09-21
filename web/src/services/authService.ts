export const authService = {
  login: (username: string, password: string): boolean => {
    if (username === 'admin' && password === 'user') {
      localStorage.setItem('authToken', 'admin-token');
      return true;
    }
    return false;
  },
  logout: () => {
    localStorage.removeItem('authToken');
  },
  isAuthenticated: (): boolean => {
    return !!localStorage.getItem('authToken');
  },
};
