const DEMO_ACCOUNTS: Record<string, string> = {
  admin: 'user',
  agente: 'agente123',
  gerente: 'gerente123',
  viewer: 'viewer123',
};

export const authService = {
  login: (username: string, password: string): boolean => {
    if (DEMO_ACCOUNTS[username] === password) {
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
